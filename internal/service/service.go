package service

import (
	"context"
	"database/sql"
	"encoding/base64"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	jwt "github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"kubeop/internal/config"
	"kubeop/internal/crypto"
	"kubeop/internal/dns"
	"kubeop/internal/kube"
	"kubeop/internal/logging"
	"kubeop/internal/store"
	"kubeop/internal/util"
	"kubeop/internal/watcherdeploy"
	crclient "sigs.k8s.io/controller-runtime/pkg/client"
	apiutil "sigs.k8s.io/controller-runtime/pkg/client/apiutil"
)

type KubeManager interface {
	GetOrCreate(ctx context.Context, id string, loader func(context.Context) ([]byte, error)) (crclient.Client, error)
	GetClientset(ctx context.Context, id string, loader func(context.Context) ([]byte, error)) (kubernetes.Interface, error)
}

var _ KubeManager = (*kube.Manager)(nil)

type Service struct {
	cfg                *config.Config
	st                 *store.Store
	km                 KubeManager
	encKey             []byte
	logger             *zap.Logger
	dnsProviderFactory func(*config.Config) dns.Provider
	watchProvisioner   watcherdeploy.Provisioner
}

const (
	watcherTokenTTL               = 365 * 24 * time.Hour
	managedByAnnotationKey        = "managed-by"
	managedByAnnotationValue      = "kubeop-operator"
	namespaceQuotaObjectName      = "tenant-quota"
	namespaceLimitRangeName       = "tenant-limits"
	watcherStatusPending          = "Pending"
	watcherStatusDeploying        = "Deploying"
	watcherStatusReady            = "Ready"
	watcherStatusFailed           = "Failed"
	watcherStatusUpdateTimeout    = 10 * time.Second
	watcherDeploymentInitialDelay = 500 * time.Millisecond
	watcherDeploymentMaxDelay     = 5 * time.Minute
)

func New(cfg *config.Config, st *store.Store, km *kube.Manager) (*Service, error) {
	if cfg == nil || st == nil {
		return nil, errors.New("missing dependencies")
	}
	key := crypto.DeriveKey(cfg.KcfgEncryptionKey)
	s := &Service{cfg: cfg, st: st, km: km, encKey: key, logger: logging.L().Named("service"), dnsProviderFactory: dns.NewProvider}
	if cfg.WatcherAutoDeploy {
		if km == nil {
			return nil, errors.New("watcher auto deploy requires kube manager")
		}
		wdCfg := watcherdeploy.Config{
			Namespace:          cfg.WatcherNamespace,
			CreateNamespace:    cfg.WatcherNamespaceCreate,
			DeploymentName:     cfg.WatcherDeploymentName,
			ServiceAccountName: cfg.WatcherServiceAccount,
			SecretName:         cfg.WatcherSecretName,
			PVCName:            cfg.WatcherPVCName,
			PVCStorageClass:    cfg.WatcherPVCStorageClass,
			PVCSize:            cfg.WatcherPVCSize,
			Image:              cfg.WatcherImage,
			BaseURL:            cfg.WatcherURL,
			LogLevel:           cfg.LogLevel,
			BatchMax:           cfg.WatcherBatchMax,
			BatchWindowMillis:  cfg.WatcherBatchWindowMillis,
			StorePath:          cfg.WatcherStorePath,
			HeartbeatMinutes:   cfg.WatcherHeartbeatMinutes,
			WaitForReady:       cfg.WatcherWaitForReady,
			ReadyTimeout:       time.Duration(cfg.WatcherReadyTimeoutSeconds) * time.Second,
		}
		opts := []watcherdeploy.Option{}
		if strings.TrimSpace(cfg.WatcherToken) == "" {
			opts = append(opts, watcherdeploy.WithTokenProvider(func(ctx context.Context, clusterID string) (string, error) {
				return GenerateWatcherToken(cfg.AdminJWTSecret, clusterID, watcherTokenTTL)
			}))
		} else {
			wdCfg.Token = cfg.WatcherToken
		}
		provisioner, err := watcherdeploy.New(wdCfg, func(ctx context.Context, clusterID string, loader watcherdeploy.Loader) (kubernetes.Interface, error) {
			return km.GetClientset(ctx, clusterID, func(inner context.Context) ([]byte, error) {
				return loader(inner)
			})
		}, opts...)
		if err != nil {
			return nil, fmt.Errorf("watcher deployer: %w", err)
		}
		s.watchProvisioner = provisioner
	}
	return s, nil
}

// SetLogger replaces the service logger. Primarily used for tests.
func (s *Service) SetLogger(logger *zap.Logger) {
	if logger == nil {
		return
	}
	s.logger = logger
}

// SetDNSProviderFactory overrides the DNS provider factory. Primarily used for tests.
func (s *Service) SetDNSProviderFactory(factory func(*config.Config) dns.Provider) {
	if factory == nil {
		return
	}
	s.dnsProviderFactory = factory
}

// SetKubeManager swaps the kube manager dependency. Primarily used for tests.
func (s *Service) SetKubeManager(km KubeManager) {
	s.km = km
}

// SetWatcherProvisioner overrides the watcher deployer. Primarily used for tests.
func (s *Service) SetWatcherProvisioner(p watcherdeploy.Provisioner) {
	s.watchProvisioner = p
}

// Health checks DB connectivity.
func (s *Service) Health(ctx context.Context) error {
	return s.st.DB().PingContext(ctx)
}

// RegisterCluster stores a new cluster with encrypted kubeconfig.
func (s *Service) RegisterCluster(ctx context.Context, name, kubeconfig string) (store.Cluster, error) {
	name = strings.TrimSpace(name)
	if name == "" || strings.TrimSpace(kubeconfig) == "" {
		return store.Cluster{}, errors.New("name and kubeconfig required")
	}
	enc, err := crypto.EncryptAESGCM([]byte(kubeconfig), s.encKey)
	if err != nil {
		return store.Cluster{}, err
	}
	id := uuid.New().String()
	c := store.Cluster{ID: id, Name: name, CreatedAt: time.Now().UTC()}
	created, err := s.st.CreateCluster(ctx, c, enc)
	if err != nil {
		return store.Cluster{}, err
	}
	if s.watchProvisioner != nil {
		pendingMsg := "watcher deployment queued"
		created.WatcherStatus = watcherStatusPending
		created.WatcherStatusMessage = &pendingMsg
		if err := s.updateWatcherStatus(created.ID, watcherStatusPending, &pendingMsg, nil); err != nil {
			s.logger.Warn("failed to mark watcher pending", zap.String("cluster_id", created.ID), zap.Error(err))
		}
		s.logger.Info("scheduled watcher deployment", zap.String("cluster_id", created.ID), zap.String("cluster_name", created.Name))
		go s.runWatcherDeployment(created.ID, created.Name)
	} else {
		s.logger.Info("watcher auto deploy skipped", zap.String("cluster_id", created.ID), zap.String("reason", s.cfg.WatcherAutoDeployExplanation()))
	}
	return created, nil
}

func (s *Service) updateWatcherStatus(clusterID, status string, message *string, readyAt *time.Time) error {
	if s == nil || s.st == nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), watcherStatusUpdateTimeout)
	defer cancel()
	if err := s.st.UpdateClusterWatcherStatus(ctx, clusterID, status, message, readyAt); err != nil {
		return err
	}
	return nil
}

func (s *Service) runWatcherDeployment(clusterID, clusterName string) {
	if s == nil || s.watchProvisioner == nil {
		return
	}
	logger := s.logger.With(zap.String("cluster_id", clusterID))
	if clusterName != "" {
		logger = logger.With(zap.String("cluster_name", clusterName))
	}
	delay := watcherDeploymentInitialDelay
	if delay <= 0 {
		delay = 5 * time.Second
	}
	attempt := 0
	for {
		attempt++
		inProgress := fmt.Sprintf("attempt %d in progress", attempt)
		if err := s.updateWatcherStatus(clusterID, watcherStatusDeploying, &inProgress, nil); err != nil {
			logger.Warn("failed to update watcher status", zap.Error(err))
		}
		loader := func(ctx context.Context) ([]byte, error) {
			return s.DecryptClusterKubeconfig(ctx, clusterID)
		}
		logger.Info("applying watcher deployment", zap.Int("attempt", attempt))
		if err := s.watchProvisioner.Ensure(context.Background(), clusterID, clusterName, loader); err == nil {
			now := time.Now().UTC()
			readyMsg := fmt.Sprintf("watcher ready after %d attempt(s)", attempt)
			if err := s.updateWatcherStatus(clusterID, watcherStatusReady, &readyMsg, &now); err != nil {
				logger.Warn("failed to persist watcher ready status", zap.Error(err))
			}
			logger.Info("watcher deployment ready", zap.Int("attempt", attempt))
			return
		} else {
			failMsg := fmt.Sprintf("attempt %d failed: %v", attempt, err)
			if updErr := s.updateWatcherStatus(clusterID, watcherStatusFailed, &failMsg, nil); updErr != nil {
				logger.Warn("failed to record watcher failure", zap.Error(updErr))
			}
			logger.Error("watcher deployment not ready", zap.Int("attempt", attempt), zap.Error(err))
		}
		if delay < watcherDeploymentMaxDelay {
			next := delay + delay/2
			if next > watcherDeploymentMaxDelay {
				next = watcherDeploymentMaxDelay
			}
			delay = next
		}
		time.Sleep(delay)
	}
}

func (s *Service) ListClusters(ctx context.Context) ([]store.Cluster, error) {
	return s.st.ListClusters(ctx)
}

func (s *Service) GetUser(ctx context.Context, id string) (store.User, error) {
	return s.st.GetUser(ctx, id)
}

func (s *Service) ListUsers(ctx context.Context, limit, offset int) ([]store.User, error) {
	return s.st.ListUsers(ctx, limit, offset)
}

func (s *Service) ListProjects(ctx context.Context, limit, offset int) ([]store.Project, error) {
	return s.st.ListProjects(ctx, limit, offset)
}

func (s *Service) ListUserProjects(ctx context.Context, userID string, limit, offset int) ([]store.Project, error) {
	return s.st.ListProjectsByUser(ctx, userID, limit, offset)
}

// EnsureProjectLogs prepares per-project and per-app log directories on startup.
func (s *Service) EnsureProjectLogs(ctx context.Context) error {
	fm := logging.Files()
	if fm == nil {
		return nil
	}
	if err := fm.EnsureBase(); err != nil {
		return err
	}
	const pageSize = 100
	offset := 0
	for {
		projects, err := s.st.ListProjects(ctx, pageSize, offset)
		if err != nil {
			return err
		}
		if len(projects) == 0 {
			break
		}
		for _, p := range projects {
			apps, err := s.st.ListAppsByProject(ctx, p.ID)
			if err != nil {
				return fmt.Errorf("list apps for project %s: %w", p.ID, err)
			}
			appIDs := make([]string, 0, len(apps))
			for _, a := range apps {
				appIDs = append(appIDs, a.ID)
			}
			if err := fm.EnsureProject(p.ID, appIDs); err != nil {
				return fmt.Errorf("ensure project logs for %s: %w", p.ID, err)
			}
		}
		if len(projects) < pageSize {
			break
		}
		offset += len(projects)
	}
	return nil
}

// DecryptClusterKubeconfig returns the kubeconfig for a given cluster ID.
func (s *Service) DecryptClusterKubeconfig(ctx context.Context, id string) ([]byte, error) {
	b, err := s.st.GetClusterKubeconfigEnc(ctx, id)
	if err != nil {
		return nil, err
	}
	return crypto.DecryptAESGCM(b, s.encKey)
}

// GenerateWatcherToken signs a JWT for the watcher deployment to authenticate against kubeOP.
func GenerateWatcherToken(secret, clusterID string, ttl time.Duration) (string, error) {
	if strings.TrimSpace(secret) == "" {
		return "", errors.New("admin jwt secret required for watcher token")
	}
	now := time.Now().UTC()
	claims := jwt.MapClaims{
		"role":       "admin",
		"sub":        fmt.Sprintf("watcher:%s", clusterID),
		"cluster_id": clusterID,
		"iat":        now.Unix(),
	}
	if ttl > 0 {
		claims["exp"] = now.Add(ttl).Unix()
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(secret))
}

// ClusterHealth summarizes connectivity to a cluster.
type ClusterHealth struct {
	ID      string    `json:"id"`
	Name    string    `json:"name"`
	Healthy bool      `json:"healthy"`
	Error   string    `json:"error,omitempty"`
	Checked time.Time `json:"checked_at"`
}

// CheckCluster attempts a lightweight API call (list namespaces, limit 1).
func (s *Service) CheckCluster(ctx context.Context, id string) (ClusterHealth, error) {
	// Lookup name for response
	var (
		name                  string
		watcherStatus         string
		watcherStatusMessage  string
		watcherHealthDeadline time.Time
	)
	{
		cs, err := s.st.ListClusters(ctx)
		if err == nil {
			for _, c := range cs {
				if c.ID == id {
					name = c.Name
					watcherStatus = c.WatcherStatus
					if c.WatcherStatusMessage != nil {
						watcherStatusMessage = *c.WatcherStatusMessage
					}
					watcherHealthDeadline = c.WatcherHealthDeadline
					break
				}
			}
		}
	}
	loader := func(ctx context.Context) ([]byte, error) {
		return s.DecryptClusterKubeconfig(ctx, id)
	}
	c, err := s.km.GetOrCreate(ctx, id, loader)
	if err != nil {
		return ClusterHealth{ID: id, Name: name, Healthy: false, Error: err.Error(), Checked: time.Now().UTC()}, nil
	}
	// simple list with limit 1
	var nl corev1.NamespaceList
	if err := c.List(ctx, &nl, crclient.Limit(1)); err != nil {
		return ClusterHealth{ID: id, Name: name, Healthy: false, Error: err.Error(), Checked: time.Now().UTC()}, nil
	}
	now := time.Now().UTC()
	if watcherStatus != "" && !watcherHealthDeadline.IsZero() && now.After(watcherHealthDeadline) && watcherStatus != watcherStatusReady {
		msg := watcherStatusMessage
		if strings.TrimSpace(msg) == "" {
			msg = fmt.Sprintf("watcher status %s", strings.ToLower(watcherStatus))
		}
		return ClusterHealth{ID: id, Name: name, Healthy: false, Error: msg, Checked: now}, nil
	}
	return ClusterHealth{ID: id, Name: name, Healthy: true, Checked: now}, nil
}

// CheckAllClusters returns health for all clusters.
func (s *Service) CheckAllClusters(ctx context.Context) ([]ClusterHealth, error) {
	cs, err := s.st.ListClusters(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]ClusterHealth, 0, len(cs))
	for _, c := range cs {
		h, _ := s.CheckCluster(ctx, c.ID) // collect error in status
		if h.Name == "" {
			h.Name = c.Name
		}
		out = append(out, h)
	}
	return out, nil
}

// ---------------- Projects / Tenancy ----------------

type ProjectCreateInput struct {
	UserID         string
	UserEmail      string // optional: if UserID empty, create/reuse by email
	UserName       string // optional display name when creating by email
	ClusterID      string
	Name           string
	QuotaOverrides map[string]string // optional resource names -> quantities
}

type ProjectCreateOutput struct {
	Project       store.Project `json:"project"`
	KubeconfigB64 string        `json:"kubeconfig_b64"`
}

func (s *Service) CreateProject(ctx context.Context, in ProjectCreateInput) (ProjectCreateOutput, error) {
	// Resolve/ensure user
	if strings.TrimSpace(in.UserID) == "" {
		email := strings.TrimSpace(strings.ToLower(in.UserEmail))
		if email == "" {
			return ProjectCreateOutput{}, errors.New("either userId or userEmail is required")
		}
		// Try find by email; if missing, create a new user
		u, err := s.st.GetUserByEmail(ctx, email)
		if err != nil {
			name := strings.TrimSpace(in.UserName)
			if name == "" {
				// derive name from email local-part
				if i := strings.Index(email, "@"); i > 0 {
					name = email[:i]
				} else {
					name = email
				}
			}
			nu := store.User{ID: uuid.New().String(), Name: name, Email: email}
			if nu, err = s.st.CreateUser(ctx, nu); err != nil {
				return ProjectCreateOutput{}, err
			}
			in.UserID = nu.ID
		} else {
			in.UserID = u.ID
		}
	}
	if strings.TrimSpace(in.ClusterID) == "" || strings.TrimSpace(in.Name) == "" {
		return ProjectCreateOutput{}, errors.New("clusterId and name are required")
	}
	projectID := uuid.New().String()
	if fm := logging.Files(); fm != nil {
		if err := fm.EnsureProject(projectID, nil); err != nil {
			return ProjectCreateOutput{}, fmt.Errorf("prepare project logs: %w", err)
		}
	}
	// Determine namespace: user's namespace or project-specific
	var nsSlug string
	if s.cfg.ProjectsInUserNamespace {
		us, _, err := s.st.GetUserSpace(ctx, in.UserID, in.ClusterID)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				ns, err2 := s.provisionUserSpace(ctx, in.UserID, in.ClusterID)
				if err2 != nil {
					return ProjectCreateOutput{}, fmt.Errorf("failed to provision user space: %w", err2)
				}
				nsSlug = ns
			} else {
				s.logger.Error("lookup user space failed", zap.String("user_id", in.UserID), zap.String("cluster_id", in.ClusterID), zap.String("error", err.Error()))
				return ProjectCreateOutput{}, fmt.Errorf("lookup user space: %w", err)
			}
		} else {
			nsSlug = us.Namespace
		}
	} else {
		nsSlug = util.Slugify(fmt.Sprintf("tenant-%s-%s", in.UserID, in.Name))
		if len(nsSlug) > 63 {
			nsSlug = nsSlug[:63]
		}
	}

	// Build clients
	loader := func(ctx context.Context) ([]byte, error) { return s.DecryptClusterKubeconfig(ctx, in.ClusterID) }
	c, err := s.km.GetOrCreate(ctx, in.ClusterID, loader)
	if err != nil {
		return ProjectCreateOutput{}, err
	}
	cs, err := s.km.GetClientset(ctx, in.ClusterID, loader)
	if err != nil {
		return ProjectCreateOutput{}, err
	}

	// 1) Namespace with PSA labels (only when creating per-project namespace)
	if !s.cfg.ProjectsInUserNamespace {
		ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: nsSlug, Labels: map[string]string{}}}
		if s.cfg.PodSecurityLevel != "" {
			if ns.Labels == nil {
				ns.Labels = map[string]string{}
			}
			ns.Labels["pod-security.kubernetes.io/enforce"] = s.cfg.PodSecurityLevel
		}
		if err := apply(ctx, c, ns); err != nil {
			return ProjectCreateOutput{}, err
		}
	}

	// 2) Namespace limit policy
	if s.cfg.ProjectsInUserNamespace {
		if err := s.ensureNamespaceLimitPolicy(ctx, c, nsSlug, nil); err != nil {
			return ProjectCreateOutput{}, err
		}
	} else {
		if err := s.ensureNamespaceLimitPolicy(ctx, c, nsSlug, in.QuotaOverrides); err != nil {
			return ProjectCreateOutput{}, err
		}
	}

	// 3) Project limit range (user-namespace mode only)
	if s.cfg.ProjectsInUserNamespace {
		lr := &corev1.LimitRange{ObjectMeta: metav1.ObjectMeta{
			Name:        "proj-" + util.Slugify(in.Name) + "-limits",
			Namespace:   nsSlug,
			Annotations: map[string]string{managedByAnnotationKey: managedByAnnotationValue},
		}}
		lr.Spec.Limits = projectLimitRange(s.cfg)
		if err := apply(ctx, c, lr); err != nil {
			return ProjectCreateOutput{}, err
		}
	}

	// 4) NetworkPolicies (only in per-project namespace mode)
	if !s.cfg.ProjectsInUserNamespace {
		for _, np := range BuildTenantNetworkPolicies(s.cfg, nsSlug) {
			if err := apply(ctx, c, np); err != nil {
				return ProjectCreateOutput{}, err
			}
		}
	}

	// 5) ServiceAccount + Role + RoleBinding (only per-project namespace mode)
	var kcStr string
	var enc []byte
	if !s.cfg.ProjectsInUserNamespace {
		sa, role, rb := BuildNamespaceRBAC(nsSlug, "tenant-sa", "tenant-role", "tenant-rb", defaultRoleRules())
		for _, obj := range []crclient.Object{sa, role, rb} {
			if err := apply(ctx, c, obj); err != nil {
				return ProjectCreateOutput{}, err
			}
		}

		// 6) Secret-backed ServiceAccount token
		secret, err := s.mintServiceAccountSecret(ctx, cs, nsSlug, sa.Name)
		if err != nil {
			return ProjectCreateOutput{}, err
		}

		// 7) Build kubeconfig (namespace-scoped) using cluster from cluster kubeconfig
		kubeconfigBytes, err := s.DecryptClusterKubeconfig(ctx, in.ClusterID)
		if err != nil {
			return ProjectCreateOutput{}, err
		}
		server := extractServer(kubeconfigBytes)
		// label cluster name for kubeconfig
		var clusterName string
		if cls, err2 := s.st.ListClusters(ctx); err2 == nil {
			for _, ci := range cls {
				if ci.ID == in.ClusterID {
					clusterName = ci.Name
					break
				}
			}
		}
		if clusterName == "" {
			clusterName = "kubeop-target"
		}
		caB64 := base64.StdEncoding.EncodeToString(secret.Data["ca.crt"])
		token := string(secret.Data[corev1.ServiceAccountTokenKey])
		kc, err := buildNamespaceScopedKubeconfig(server, caB64, nsSlug, sa.Name, clusterName, token)
		if err != nil {
			return ProjectCreateOutput{}, err
		}
		kcStr = kc

		// Store in DB (encrypted)
		e, err := crypto.EncryptAESGCM([]byte(kcStr), s.encKey)
		if err != nil {
			return ProjectCreateOutput{}, err
		}
		enc = e

		if existing, _, err := s.st.GetKubeconfigByProject(ctx, projectID); err == nil {
			if err := s.st.UpdateKubeconfigRecord(ctx, existing.ID, secret.Name, sa.Name, e); err != nil {
				return ProjectCreateOutput{}, err
			}
		} else if errors.Is(err, sql.ErrNoRows) {
			pid := projectID
			rec := store.KubeconfigRecord{
				ID:             uuid.New().String(),
				ClusterID:      in.ClusterID,
				Namespace:      nsSlug,
				UserID:         in.UserID,
				ProjectID:      &pid,
				ServiceAccount: sa.Name,
				SecretName:     secret.Name,
			}
			if _, err := s.st.CreateKubeconfigRecord(ctx, rec, e); err != nil {
				return ProjectCreateOutput{}, err
			}
		} else {
			return ProjectCreateOutput{}, err
		}
	}
	p := store.Project{ID: projectID, UserID: in.UserID, ClusterID: in.ClusterID, Name: in.Name, Namespace: nsSlug}
	qoJSON, err := EncodeQuotaOverrides(in.QuotaOverrides)
	if err != nil {
		return ProjectCreateOutput{}, err
	}
	p, err = s.st.CreateProject(ctx, p, qoJSON, enc)
	if err != nil {
		return ProjectCreateOutput{}, err
	}
	fields := []zap.Field{
		zap.String("project_name", p.Name),
		zap.String("cluster_id", p.ClusterID),
		zap.String("namespace", p.Namespace),
		zap.String("user_id", p.UserID),
	}
	logging.ProjectLogger(p.ID).Info("project_created", fields...)
	if _, err := s.AppendProjectEvent(ctx, EventInput{
		ProjectID: p.ID,
		Kind:      "project_created",
		Severity:  SeverityInfo,
		Message:   fmt.Sprintf("project %s created", p.Name),
		Meta: map[string]any{
			"project_name": p.Name,
			"cluster_id":   p.ClusterID,
			"namespace":    p.Namespace,
			"user_id":      p.UserID,
		},
	}); err != nil {
		return ProjectCreateOutput{}, err
	}
	if s.cfg.ProjectsInUserNamespace {
		return ProjectCreateOutput{Project: p, KubeconfigB64: ""}, nil
	}
	return ProjectCreateOutput{Project: p, KubeconfigB64: toB64([]byte(kcStr))}, nil
}

// provisionUserSpace ensures a per-user namespace exists on the target cluster and stores
// an encrypted kubeconfig for that namespace in the database.
func (s *Service) provisionUserSpace(ctx context.Context, userID, clusterID string) (string, error) {
	// Build clients
	loader := func(ctx context.Context) ([]byte, error) { return s.DecryptClusterKubeconfig(ctx, clusterID) }
	c, err := s.km.GetOrCreate(ctx, clusterID, loader)
	if err != nil {
		return "", err
	}
	cs, err := s.km.GetClientset(ctx, clusterID, loader)
	if err != nil {
		return "", err
	}

	nsName := "user-" + strings.TrimSpace(userID)
	if nsName == "user-" {
		return "", errors.New("invalid userID")
	}
	ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: nsName, Labels: map[string]string{}}}
	if s.cfg.PodSecurityLevel != "" {
		ns.Labels["pod-security.kubernetes.io/enforce"] = s.cfg.PodSecurityLevel
	}
	if err := apply(ctx, c, ns); err != nil {
		return "", err
	}

	// Defaults: ResourceQuota and LimitRange
	if err := s.ensureNamespaceLimitPolicy(ctx, c, nsName, nil); err != nil {
		return "", err
	}

	// NetworkPolicies: default-deny + allow DNS + allow from ingress namespace
	for _, np := range BuildTenantNetworkPolicies(s.cfg, nsName) {
		if err := apply(ctx, c, np); err != nil {
			return "", err
		}
	}

	// ServiceAccount + Role/Binding for the user
	sa, role, rb := BuildNamespaceRBAC(nsName, "user-sa", "user-role", "user-rb", defaultRoleRules())
	for _, obj := range []crclient.Object{sa, role, rb} {
		if err := apply(ctx, c, obj); err != nil {
			return "", err
		}
	}

	// Secret-backed token and kubeconfig
	secret, err := s.mintServiceAccountSecret(ctx, cs, nsName, sa.Name)
	if err != nil {
		return "", err
	}
	// Resolve cluster name for kubeconfig labels
	var clusterName string
	if cl, err := s.st.ListClusters(ctx); err == nil {
		for _, cinfo := range cl {
			if cinfo.ID == clusterID {
				clusterName = cinfo.Name
				break
			}
		}
	}
	if clusterName == "" {
		clusterName = "kubeop-target"
	}
	kubeconfigBytes, err := s.DecryptClusterKubeconfig(ctx, clusterID)
	if err != nil {
		return "", err
	}
	userLabel := s.kubeconfigUserLabel(ctx, userID)
	server := extractServer(kubeconfigBytes)
	caB64 := base64.StdEncoding.EncodeToString(secret.Data["ca.crt"])
	token := string(secret.Data[corev1.ServiceAccountTokenKey])
	kcStr, err := buildNamespaceScopedKubeconfig(server, caB64, nsName, userLabel, clusterName, token)
	if err != nil {
		return "", err
	}
	enc, err := crypto.EncryptAESGCM([]byte(kcStr), s.encKey)
	if err != nil {
		return "", err
	}

	// Store userspace
	if _, err = s.st.CreateUserSpace(ctx, store.UserSpace{ID: uuid.New().String(), UserID: userID, ClusterID: clusterID, Namespace: nsName}, enc); err != nil {
		if existing, _, err2 := s.st.GetUserSpace(ctx, userID, clusterID); err2 == nil {
			if err := s.st.UpdateUserSpaceKubeconfig(ctx, existing.ID, enc); err != nil {
				return "", err
			}
		} else {
			return "", err
		}
	}
	// Persist kubeconfig mapping for user scope (idempotent on namespace)
	if existing, _, err := s.st.GetKubeconfigByUserScope(ctx, clusterID, nsName, userID); err == nil {
		if err := s.st.UpdateKubeconfigRecord(ctx, existing.ID, secret.Name, sa.Name, enc); err != nil {
			return "", err
		}
	} else if errors.Is(err, sql.ErrNoRows) {
		rec := store.KubeconfigRecord{
			ID:             uuid.New().String(),
			ClusterID:      clusterID,
			Namespace:      nsName,
			UserID:         userID,
			ServiceAccount: sa.Name,
			SecretName:     secret.Name,
		}
		if _, err := s.st.CreateKubeconfigRecord(ctx, rec, enc); err != nil {
			return "", err
		}
	} else {
		return "", err
	}
	return nsName, nil
}

type ProjectStatus struct {
	Project store.Project   `json:"project"`
	Exists  bool            `json:"exists"`
	Details map[string]bool `json:"details"`
}

type ResourceQuotaSnapshot struct {
	Name string            `json:"name"`
	Hard map[string]string `json:"hard"`
	Used map[string]string `json:"used"`
}

type LoadBalancerQuota struct {
	Default   int  `json:"default"`
	Override  *int `json:"override,omitempty"`
	Effective int  `json:"effective"`
	Used      int  `json:"used"`
}

type ProjectQuotaSnapshot struct {
	Project       store.Project         `json:"project"`
	Defaults      map[string]string     `json:"defaults"`
	Overrides     map[string]string     `json:"overrides"`
	Effective     map[string]string     `json:"effective"`
	ResourceQuota ResourceQuotaSnapshot `json:"resourceQuota"`
	LoadBalancers LoadBalancerQuota     `json:"loadBalancers"`
}

func (s *Service) GetProjectStatus(ctx context.Context, id string) (ProjectStatus, error) {
	p, _, _, err := s.st.GetProject(ctx, id)
	if err != nil {
		return ProjectStatus{}, err
	}
	loader := func(ctx context.Context) ([]byte, error) { return s.DecryptClusterKubeconfig(ctx, p.ClusterID) }
	c, err := s.km.GetOrCreate(ctx, p.ClusterID, loader)
	if err != nil {
		return ProjectStatus{}, err
	}
	ns := &corev1.Namespace{}
	err = c.Get(ctx, crclient.ObjectKey{Name: p.Namespace}, ns)
	details := map[string]bool{}
	exists := err == nil
	if exists {
		// check key resources
		rq := &corev1.ResourceQuota{}
		details["resourcequota"] = c.Get(ctx, crclient.ObjectKey{Namespace: p.Namespace, Name: namespaceQuotaObjectName}, rq) == nil
		lr := &corev1.LimitRange{}
		details["limitrange"] = c.Get(ctx, crclient.ObjectKey{Namespace: p.Namespace, Name: namespaceLimitRangeName}, lr) == nil
		sa := &corev1.ServiceAccount{}
		details["serviceaccount"] = c.Get(ctx, crclient.ObjectKey{Namespace: p.Namespace, Name: "tenant-sa"}, sa) == nil
	}
	return ProjectStatus{Project: p, Exists: exists, Details: details}, nil
}

func (s *Service) GetProjectQuota(ctx context.Context, id string) (ProjectQuotaSnapshot, error) {
	if s.cfg.ProjectsInUserNamespace {
		return ProjectQuotaSnapshot{}, errors.New("per-project quotas not supported when projects share user namespace; adjust namespace ResourceQuota")
	}
	p, qo, _, err := s.st.GetProject(ctx, id)
	if err != nil {
		return ProjectQuotaSnapshot{}, err
	}
	overrides, err := DecodeQuotaOverrides(qo)
	if err != nil {
		return ProjectQuotaSnapshot{}, err
	}
	loader := func(ctx context.Context) ([]byte, error) { return s.DecryptClusterKubeconfig(ctx, p.ClusterID) }
	c, err := s.km.GetOrCreate(ctx, p.ClusterID, loader)
	if err != nil {
		return ProjectQuotaSnapshot{}, err
	}
	rq := &corev1.ResourceQuota{}
	rqSnapshot := ResourceQuotaSnapshot{
		Name: namespaceQuotaObjectName,
		Hard: map[string]string{},
		Used: map[string]string{},
	}
	if err := c.Get(ctx, crclient.ObjectKey{Namespace: p.Namespace, Name: namespaceQuotaObjectName}, rq); err != nil {
		if !apierrors.IsNotFound(err) {
			return ProjectQuotaSnapshot{}, err
		}
	} else {
		rqSnapshot.Name = rq.Name
		hard := rq.Status.Hard
		if len(hard) == 0 {
			hard = rq.Spec.Hard
		}
		rqSnapshot.Hard = resourceListToStringMap(hard)
		rqSnapshot.Used = resourceListToStringMap(rq.Status.Used)
	}

	defaults := resourceListToStringMap(defaultQuota(s.cfg, nil))
	effective := resourceListToStringMap(defaultQuota(s.cfg, overrides))
	overridesCopy := copyStringMap(overrides)

	var overridePtr *int
	effectiveLB := s.cfg.MaxLoadBalancersPerProject
	if raw, ok := overrides["services.loadbalancers"]; ok {
		if n, err := strconv.Atoi(strings.TrimSpace(raw)); err == nil {
			overridePtr = new(int)
			*overridePtr = n
			effectiveLB = n
		}
	}
	existingLB := 0
	var svcs corev1.ServiceList
	if err := c.List(ctx, &svcs, crclient.InNamespace(p.Namespace)); err != nil {
		return ProjectQuotaSnapshot{}, err
	}
	for _, svc := range svcs.Items {
		if svc.Spec.Type == corev1.ServiceTypeLoadBalancer {
			existingLB++
		}
	}

	snapshot := ProjectQuotaSnapshot{
		Project:   p,
		Defaults:  defaults,
		Overrides: overridesCopy,
		Effective: effective,
		ResourceQuota: ResourceQuotaSnapshot{
			Name: rqSnapshot.Name,
			Hard: rqSnapshot.Hard,
			Used: rqSnapshot.Used,
		},
		LoadBalancers: LoadBalancerQuota{
			Default:   s.cfg.MaxLoadBalancersPerProject,
			Override:  overridePtr,
			Effective: effectiveLB,
			Used:      existingLB,
		},
	}
	return snapshot, nil
}

func (s *Service) SetProjectSuspended(ctx context.Context, id string, suspended bool) error {
	if s.cfg.ProjectsInUserNamespace {
		return errors.New("project suspend/unsuspend not supported when projects share user namespace; use user-level quotas")
	}
	p, qo, _, err := s.st.GetProject(ctx, id)
	if err != nil {
		return err
	}
	loader := func(ctx context.Context) ([]byte, error) { return s.DecryptClusterKubeconfig(ctx, p.ClusterID) }
	c, err := s.km.GetOrCreate(ctx, p.ClusterID, loader)
	if err != nil {
		return err
	}
	var (
		rq *corev1.ResourceQuota
		lr *corev1.LimitRange
	)
	if suspended {
		var err error
		rq, lr, err = buildNamespaceLimitPolicyObjects(s.cfg, p.Namespace, nil)
		if err != nil {
			return err
		}
		rq.Spec.Hard = corev1.ResourceList{corev1.ResourcePods: resourceMustParse("0")}
	} else {
		overrides, err := DecodeQuotaOverrides(qo)
		if err != nil {
			return err
		}
		var buildErr error
		rq, lr, buildErr = buildNamespaceLimitPolicyObjects(s.cfg, p.Namespace, overrides)
		if buildErr != nil {
			return buildErr
		}
	}
	if err := s.applyNamespaceLimitObjects(ctx, c, p.Namespace, rq, lr); err != nil {
		return err
	}
	if err := s.st.UpdateProjectSuspended(ctx, id, suspended); err != nil {
		return err
	}
	msg := "project_unsuspended"
	if suspended {
		msg = "project_suspended"
	}
	fields := []zap.Field{
		zap.Bool("suspended", suspended),
		zap.String("cluster_id", p.ClusterID),
		zap.String("namespace", p.Namespace),
	}
	logging.ProjectLogger(id).Info(msg, fields...)
	statusMsg := "project unsuspended"
	if suspended {
		statusMsg = "project suspended"
	}
	if _, err := s.AppendProjectEvent(ctx, EventInput{
		ProjectID: id,
		Kind:      msg,
		Severity:  SeverityInfo,
		Message:   statusMsg,
		Meta: map[string]any{
			"suspended":  suspended,
			"cluster_id": p.ClusterID,
			"namespace":  p.Namespace,
		},
	}); err != nil {
		return err
	}
	return nil
}

func (s *Service) UpdateProjectQuota(ctx context.Context, id string, overrides map[string]string) error {
	if s.cfg.ProjectsInUserNamespace {
		return errors.New("per-project quotas not supported when projects share user namespace; adjust namespace ResourceQuota")
	}
	p, _, _, err := s.st.GetProject(ctx, id)
	if err != nil {
		return err
	}
	loader := func(ctx context.Context) ([]byte, error) { return s.DecryptClusterKubeconfig(ctx, p.ClusterID) }
	c, err := s.km.GetOrCreate(ctx, p.ClusterID, loader)
	if err != nil {
		return err
	}
	rq, lr, err := buildNamespaceLimitPolicyObjects(s.cfg, p.Namespace, overrides)
	if err != nil {
		return err
	}
	if err := s.applyNamespaceLimitObjects(ctx, c, p.Namespace, rq, lr); err != nil {
		return err
	}
	qoJSON, err := EncodeQuotaOverrides(overrides)
	if err != nil {
		return err
	}
	if err := s.st.UpdateProjectQuotaOverrides(ctx, id, qoJSON); err != nil {
		return err
	}
	fields := []zap.Field{
		zap.Any("overrides", overrides),
		zap.String("cluster_id", p.ClusterID),
		zap.String("namespace", p.Namespace),
	}
	logging.ProjectLogger(id).Info("project_quota_updated", fields...)
	if _, err := s.AppendProjectEvent(ctx, EventInput{
		ProjectID: id,
		Kind:      "project_quota_updated",
		Severity:  SeverityInfo,
		Message:   "project quotas updated",
		Meta: map[string]any{
			"overrides":  overrides,
			"cluster_id": p.ClusterID,
			"namespace":  p.Namespace,
		},
	}); err != nil {
		return err
	}
	return nil
}

func (s *Service) DeleteProject(ctx context.Context, id string) error {
	p, _, _, err := s.st.GetProject(ctx, id)
	if err != nil {
		return err
	}
	loader := func(ctx context.Context) ([]byte, error) { return s.DecryptClusterKubeconfig(ctx, p.ClusterID) }
	c, err := s.km.GetOrCreate(ctx, p.ClusterID, loader)
	if err != nil {
		return err
	}
	if s.cfg.ProjectsInUserNamespace {
		// delete project-specific LimitRange if present
		lr := &corev1.LimitRange{ObjectMeta: metav1.ObjectMeta{Name: "proj-" + util.Slugify(p.Name) + "-limits", Namespace: p.Namespace}}
		_ = c.Delete(ctx, lr) // ignore not found
	} else {
		// delete namespace (cascades)
		ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: p.Namespace}}
		if err := c.Delete(ctx, ns); err != nil && !apierrors.IsNotFound(err) {
			return err
		}
	}
	// Soft-delete apps under this project and the project row
	_ = s.st.SoftDeleteAppsByProject(ctx, id)
	if err := s.st.SoftDeleteProject(ctx, id); err != nil {
		return err
	}
	fields := []zap.Field{
		zap.String("project_name", p.Name),
		zap.String("cluster_id", p.ClusterID),
		zap.String("namespace", p.Namespace),
	}
	logging.ProjectLogger(id).Info("project_deleted", fields...)
	if _, err := s.AppendProjectEvent(ctx, EventInput{
		ProjectID: id,
		Kind:      "project_deleted",
		Severity:  SeverityWarn,
		Message:   fmt.Sprintf("project %s deleted", p.Name),
		Meta: map[string]any{
			"project_name": p.Name,
			"cluster_id":   p.ClusterID,
			"namespace":    p.Namespace,
		},
	}); err != nil {
		return err
	}
	return nil
}

// Helpers
func apply(ctx context.Context, c crclient.Client, obj crclient.Object) error {
	obj.SetManagedFields(nil)
	// Ensure GVK is set for server-side apply
	if obj.GetObjectKind().GroupVersionKind().Empty() {
		if gvk, err := apiutil.GVKForObject(obj, c.Scheme()); err == nil {
			obj.GetObjectKind().SetGroupVersionKind(gvk)
		} else {
			return err
		}
	}
	// Use server-side apply
	return c.Patch(ctx, obj, crclient.Apply, crclient.ForceOwnership, crclient.FieldOwner("kubeop"))
}

func defaultRoleRules() []rbacv1.PolicyRule {
	return []rbacv1.PolicyRule{
		{
			APIGroups: []string{""},
			Resources: []string{"pods", "services", "configmaps", "secrets", "persistentvolumeclaims", "events"},
			Verbs:     []string{"get", "list", "watch", "create", "update", "delete"},
		},
		{
			APIGroups: []string{"apps"},
			Resources: []string{"deployments", "replicasets", "statefulsets", "daemonsets"},
			Verbs:     []string{"get", "list", "watch", "create", "update", "delete"},
		},
		{
			APIGroups: []string{"apps"},
			Resources: []string{"deployments/scale", "statefulsets/scale"},
			Verbs:     []string{"get", "patch", "update"},
		},
		{
			APIGroups: []string{"networking.k8s.io"},
			Resources: []string{"ingresses"},
			Verbs:     []string{"get", "list", "watch"},
		},
		{
			APIGroups: []string{"batch"},
			Resources: []string{"jobs", "cronjobs"},
			Verbs:     []string{"get", "list", "watch", "create", "update", "delete"},
		},
	}
}

// DefaultUserNamespaceRoleRules exposes the default rules for testing and documentation.
func DefaultUserNamespaceRoleRules() []rbacv1.PolicyRule { return defaultRoleRules() }

func (s *Service) ensureNamespaceLimitPolicy(ctx context.Context, c crclient.Client, namespace string, overrides map[string]string) error {
	rq, lr, err := buildNamespaceLimitPolicyObjects(s.cfg, namespace, overrides)
	if err != nil {
		return err
	}
	return s.applyNamespaceLimitObjects(ctx, c, namespace, rq, lr)
}

func (s *Service) applyNamespaceLimitObjects(ctx context.Context, c crclient.Client, namespace string, rq *corev1.ResourceQuota, lr *corev1.LimitRange) error {
	if rq == nil || lr == nil {
		return errors.New("resource quota and limit range are required")
	}
	if err := apply(ctx, c, rq); err != nil {
		return fmt.Errorf("apply %s resource quota: %w", namespaceQuotaObjectName, err)
	}
	if err := apply(ctx, c, lr); err != nil {
		return fmt.Errorf("apply %s limit range: %w", namespaceLimitRangeName, err)
	}
	if s.logger != nil {
		s.logger.Debug("namespace_limit_policy_applied", zap.String("namespace", namespace))
	}
	return nil
}

func buildNamespaceLimitPolicyObjects(cfg *config.Config, namespace string, overrides map[string]string) (*corev1.ResourceQuota, *corev1.LimitRange, error) {
	if cfg == nil {
		return nil, nil, errors.New("config is required")
	}
	ns := strings.TrimSpace(namespace)
	if ns == "" {
		return nil, nil, errors.New("namespace is required")
	}
	rq := &corev1.ResourceQuota{ObjectMeta: metav1.ObjectMeta{
		Name:        namespaceQuotaObjectName,
		Namespace:   ns,
		Annotations: map[string]string{managedByAnnotationKey: managedByAnnotationValue},
	}}
	configureNamespaceResourceQuota(cfg, rq, overrides)
	lr := &corev1.LimitRange{ObjectMeta: metav1.ObjectMeta{
		Name:        namespaceLimitRangeName,
		Namespace:   ns,
		Annotations: map[string]string{managedByAnnotationKey: managedByAnnotationValue},
	}}
	lr.Spec.Limits = defaultLimitRange(cfg)
	return rq, lr, nil
}

func configureNamespaceResourceQuota(cfg *config.Config, rq *corev1.ResourceQuota, overrides map[string]string) {
	rq.Spec.Hard = defaultQuota(cfg, overrides)
	rq.Spec.Scopes = buildQuotaScopes(cfg.NamespaceQuotaScopes)
	rq.Spec.ScopeSelector = buildPriorityClassSelector(cfg.NamespaceQuotaPriorityClasses)
}

func defaultQuota(cfg *config.Config, overrides map[string]string) corev1.ResourceList {
	rl := corev1.ResourceList{}
	setResourceQuantity(rl, corev1.ResourceRequestsCPU, cfg.NamespaceQuotaRequestsCPU)
	setResourceQuantity(rl, corev1.ResourceLimitsCPU, cfg.NamespaceQuotaLimitsCPU)
	setResourceQuantity(rl, corev1.ResourceRequestsMemory, cfg.NamespaceQuotaRequestsMemory)
	setResourceQuantity(rl, corev1.ResourceLimitsMemory, cfg.NamespaceQuotaLimitsMemory)
	setResourceQuantity(rl, corev1.ResourceRequestsEphemeralStorage, cfg.NamespaceQuotaRequestsEphemeral)
	setResourceQuantity(rl, corev1.ResourceLimitsEphemeralStorage, cfg.NamespaceQuotaLimitsEphemeral)
	setResourceQuantity(rl, corev1.ResourcePods, cfg.NamespaceQuotaPods)
	setResourceQuantity(rl, corev1.ResourceServices, cfg.NamespaceQuotaServices)
	setResourceQuantity(rl, corev1.ResourceName("services.loadbalancers"), cfg.NamespaceQuotaServicesLoadBalancers)
	setResourceQuantity(rl, corev1.ResourceConfigMaps, cfg.NamespaceQuotaConfigMaps)
	setResourceQuantity(rl, corev1.ResourceSecrets, cfg.NamespaceQuotaSecrets)
	setResourceQuantity(rl, corev1.ResourcePersistentVolumeClaims, cfg.NamespaceQuotaPVCs)
	setResourceQuantity(rl, corev1.ResourceRequestsStorage, cfg.NamespaceQuotaRequestsStorage)
	setResourceQuantity(rl, corev1.ResourceName("deployments.apps"), cfg.NamespaceQuotaDeployments)
	setResourceQuantity(rl, corev1.ResourceName("replicasets.apps"), cfg.NamespaceQuotaReplicaSets)
	setResourceQuantity(rl, corev1.ResourceName("statefulsets.apps"), cfg.NamespaceQuotaStatefulSets)
	setResourceQuantity(rl, corev1.ResourceName("jobs.batch"), cfg.NamespaceQuotaJobs)
	setResourceQuantity(rl, corev1.ResourceName("cronjobs.batch"), cfg.NamespaceQuotaCronJobs)
	setResourceQuantity(rl, corev1.ResourceName("ingresses.networking.k8s.io"), cfg.NamespaceQuotaIngresses)
	for k, v := range overrides {
		setResourceQuantity(rl, corev1.ResourceName(k), v)
	}
	return rl
}

func defaultLimitRange(cfg *config.Config) []corev1.LimitRangeItem {
	container := corev1.LimitRangeItem{
		Type:           corev1.LimitTypeContainer,
		Max:            corev1.ResourceList{},
		Min:            corev1.ResourceList{},
		Default:        corev1.ResourceList{},
		DefaultRequest: corev1.ResourceList{},
	}
	setResourceQuantity(container.Max, corev1.ResourceCPU, cfg.NamespaceLRContainerMaxCPU)
	setResourceQuantity(container.Max, corev1.ResourceMemory, cfg.NamespaceLRContainerMaxMemory)
	setResourceQuantity(container.Max, corev1.ResourceEphemeralStorage, cfg.NamespaceLRContainerMaxEphemeral)
	setResourceQuantity(container.Min, corev1.ResourceCPU, cfg.NamespaceLRContainerMinCPU)
	setResourceQuantity(container.Min, corev1.ResourceMemory, cfg.NamespaceLRContainerMinMemory)
	setResourceQuantity(container.Min, corev1.ResourceEphemeralStorage, cfg.NamespaceLRContainerMinEphemeral)
	setResourceQuantity(container.Default, corev1.ResourceCPU, cfg.NamespaceLRContainerDefaultCPU)
	setResourceQuantity(container.Default, corev1.ResourceMemory, cfg.NamespaceLRContainerDefaultMemory)
	setResourceQuantity(container.Default, corev1.ResourceEphemeralStorage, cfg.NamespaceLRContainerDefaultEphemeral)
	setResourceQuantity(container.DefaultRequest, corev1.ResourceCPU, cfg.NamespaceLRContainerDefaultRequestCPU)
	setResourceQuantity(container.DefaultRequest, corev1.ResourceMemory, cfg.NamespaceLRContainerDefaultRequestMemory)
	setResourceQuantity(container.DefaultRequest, corev1.ResourceEphemeralStorage, cfg.NamespaceLRContainerDefaultRequestEphemeral)
	applyExtendedResources(container.Max, cfg.NamespaceLRExtMax)
	applyExtendedResources(container.Min, cfg.NamespaceLRExtMin)
	applyExtendedResources(container.Default, cfg.NamespaceLRExtDefault)
	applyExtendedResources(container.DefaultRequest, cfg.NamespaceLRExtDefaultRequest)

	pod := corev1.LimitRangeItem{
		Type: corev1.LimitTypePod,
		Max:  corev1.ResourceList{},
		Min:  corev1.ResourceList{},
	}
	setResourceQuantity(pod.Max, corev1.ResourceCPU, cfg.NamespaceQuotaLimitsCPU)
	setResourceQuantity(pod.Max, corev1.ResourceMemory, cfg.NamespaceQuotaLimitsMemory)
	setResourceQuantity(pod.Max, corev1.ResourceEphemeralStorage, cfg.NamespaceQuotaLimitsEphemeral)
	setResourceQuantity(pod.Min, corev1.ResourceCPU, cfg.NamespaceLRContainerMinCPU)
	setResourceQuantity(pod.Min, corev1.ResourceMemory, cfg.NamespaceLRContainerMinMemory)
	setResourceQuantity(pod.Min, corev1.ResourceEphemeralStorage, cfg.NamespaceLRContainerMinEphemeral)
	applyExtendedResources(pod.Max, cfg.NamespaceLRExtMax)
	applyExtendedResources(pod.Min, cfg.NamespaceLRExtMin)

	return []corev1.LimitRangeItem{container, pod}
}
func projectLimitRange(cfg *config.Config) []corev1.LimitRangeItem {
	return []corev1.LimitRangeItem{{
		Type: corev1.LimitTypeContainer,
		DefaultRequest: corev1.ResourceList{
			corev1.ResourceCPU:    resourceMustParse(cfg.ProjectLRRequestCPU),
			corev1.ResourceMemory: resourceMustParse(cfg.ProjectLRRequestMemory),
		},
		Default: corev1.ResourceList{
			corev1.ResourceCPU:    resourceMustParse(cfg.ProjectLRLimitCPU),
			corev1.ResourceMemory: resourceMustParse(cfg.ProjectLRLimitMemory),
		},
	}}
}

func buildNamespaceScopedKubeconfig(server, caBase64, namespace, userLabel, clusterLabel, token string) (string, error) {
	if server == "" {
		return "", errors.New("cluster server is required")
	}
	if caBase64 == "" {
		return "", errors.New("cluster CA is required")
	}
	var out strings.Builder
	out.WriteString("apiVersion: v1\nkind: Config\n")
	out.WriteString("clusters:\n")
	out.WriteString("- cluster:\n")
	out.WriteString("    certificate-authority-data: ")
	out.WriteString(caBase64)
	out.WriteString("\n    server: ")
	out.WriteString(server)
	out.WriteString("\n  name: ")
	out.WriteString(clusterLabel)
	out.WriteString("\n")
	out.WriteString("contexts:\n- context:\n    cluster: ")
	out.WriteString(clusterLabel)
	out.WriteString("\n    namespace: ")
	out.WriteString(namespace)
	out.WriteString("\n    user: ")
	out.WriteString(userLabel)
	out.WriteString("\n  name: ")
	out.WriteString(clusterLabel)
	out.WriteString("\n")
	out.WriteString("current-context: ")
	out.WriteString(clusterLabel)
	out.WriteString("\nusers:\n- name: ")
	out.WriteString(userLabel)
	out.WriteString("\n  user:\n    token: ")
	out.WriteString(token)
	out.WriteString("\n")
	return out.String(), nil
}

// extractServer and extractCABase64 are simple YAML scrapers to keep dependencies light
func extractServer(kc []byte) string { return extractYAMLScalar(kc, "server:") }
func extractCABase64(kc []byte) string {
	return extractYAMLScalar(kc, "certificate-authority-data:")
}

func (s *Service) mintServiceAccountSecret(ctx context.Context, cs kubernetes.Interface, namespace, saName string) (*corev1.Secret, error) {
	if namespace == "" || saName == "" {
		return nil, errors.New("namespace and serviceaccount required")
	}
	secretName := fmt.Sprintf("%s-token-%s", saName, strings.ReplaceAll(uuid.New().String(), "-", ""))
	sec := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{
		Name:        secretName,
		Namespace:   namespace,
		Annotations: map[string]string{corev1.ServiceAccountNameKey: saName},
	}, Type: corev1.SecretTypeServiceAccountToken}
	created, err := cs.CoreV1().Secrets(namespace).Create(ctx, sec, metav1.CreateOptions{})
	if apierrors.IsAlreadyExists(err) {
		created, err = cs.CoreV1().Secrets(namespace).Get(ctx, secretName, metav1.GetOptions{})
	}
	if err != nil {
		return nil, err
	}
	backoff := wait.Backoff{Steps: 8, Duration: 200 * time.Millisecond, Factor: 1.5}
	if err := wait.ExponentialBackoffWithContext(ctx, backoff, func(ctx context.Context) (bool, error) {
		fresh, err := cs.CoreV1().Secrets(namespace).Get(ctx, created.Name, metav1.GetOptions{})
		if err != nil {
			if apierrors.IsNotFound(err) {
				return false, nil
			}
			return false, err
		}
		token := fresh.Data[corev1.ServiceAccountTokenKey]
		ca := fresh.Data["ca.crt"]
		if len(token) == 0 || len(ca) == 0 {
			return false, nil
		}
		created = fresh
		return true, nil
	}); err != nil {
		return nil, err
	}
	return created, nil
}

func extractYAMLScalar(kc []byte, key string) string {
	s := string(kc)
	idx := strings.Index(s, key)
	if idx == -1 {
		return ""
	}
	rest := s[idx+len(key):]
	rest = strings.TrimSpace(rest)
	if i := strings.Index(rest, "\n"); i >= 0 {
		rest = rest[:i]
	}
	return rest
}

// TestExtractYAMLScalar exposes extractYAMLScalar for white-box tests in testcase/.
var TestExtractYAMLScalar = extractYAMLScalar
var TestBuildNamespaceScopedKubeconfig = buildNamespaceScopedKubeconfig
var TestMintServiceAccountSecret = (*Service).mintServiceAccountSecret
var TestConfigureNamespaceResourceQuota = configureNamespaceResourceQuota
var TestDefaultQuota = defaultQuota
var TestDefaultLimitRange = defaultLimitRange
var TestBuildNamespaceLimitPolicyObjects = buildNamespaceLimitPolicyObjects

func buildQuotaScopes(raw string) []corev1.ResourceQuotaScope {
	values := parseCSV(raw)
	if len(values) == 0 {
		return nil
	}
	sort.Strings(values)
	scopes := make([]corev1.ResourceQuotaScope, 0, len(values))
	for _, v := range values {
		scopes = append(scopes, corev1.ResourceQuotaScope(v))
	}
	return scopes
}

func buildPriorityClassSelector(raw string) *corev1.ScopeSelector {
	values := parseCSV(raw)
	if len(values) == 0 {
		return nil
	}
	sort.Strings(values)
	return &corev1.ScopeSelector{MatchExpressions: []corev1.ScopedResourceSelectorRequirement{{
		ScopeName: corev1.ResourceQuotaScopePriorityClass,
		Operator:  corev1.ScopeSelectorOpIn,
		Values:    values,
	}}}
}

func parseCSV(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}

func parseResourcePairs(raw string) map[corev1.ResourceName]string {
	out := map[corev1.ResourceName]string{}
	for _, segment := range parseCSV(raw) {
		kv := strings.SplitN(segment, "=", 2)
		if len(kv) != 2 {
			continue
		}
		key := strings.TrimSpace(kv[0])
		val := strings.TrimSpace(kv[1])
		if key == "" || val == "" {
			continue
		}
		out[corev1.ResourceName(key)] = val
	}
	return out
}

func setResourceQuantity(list corev1.ResourceList, name corev1.ResourceName, raw string) {
	if list == nil {
		return
	}
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return
	}
	list[name] = resourceMustParse(raw)
}

func applyExtendedResources(list corev1.ResourceList, raw string) {
	if list == nil {
		return
	}
	for name, val := range parseResourcePairs(raw) {
		setResourceQuantity(list, name, val)
	}
}

func protoPtr(p corev1.Protocol) *corev1.Protocol  { return &p }
func intstrPtr(p int32) *intstr.IntOrString        { v := intstr.FromInt(int(p)); return &v }
func resourceMustParse(s string) resource.Quantity { q := resource.MustParse(s); return q }

func resourceListToStringMap(list corev1.ResourceList) map[string]string {
	if len(list) == 0 {
		return map[string]string{}
	}
	out := make(map[string]string, len(list))
	for name, qty := range list {
		out[string(name)] = qty.String()
	}
	return out
}

func copyStringMap(in map[string]string) map[string]string {
	if len(in) == 0 {
		return map[string]string{}
	}
	out := make(map[string]string, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}
func toB64(b []byte) string { return base64.StdEncoding.EncodeToString(b) }

func (s *Service) kubeconfigUserLabel(ctx context.Context, userID string) string {
	if u, err := s.st.GetUser(ctx, userID); err == nil {
		return ResolveUserLabel(u.Name, u.Email, userID)
	}
	return ResolveUserLabel("", "", userID)
}
