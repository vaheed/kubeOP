package watcherdeploy

import (
    "context"
    "crypto/sha256"
    "errors"
    "fmt"
    "net/url"
    "strings"
    "time"

	"go.uber.org/zap"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/utils/pointer"
	"kubeop/internal/logging"
)

// Loader returns kubeconfig bytes for the requested cluster. The service layer wires this to
// decrypt the stored kubeconfig before client creation.
type Loader func(context.Context) ([]byte, error)

// ClientFactory resolves a Kubernetes clientset for the provided cluster ID.
type ClientFactory func(ctx context.Context, clusterID string, loader Loader) (kubernetes.Interface, error)

// Config describes how the watcher deployment should be orchestrated inside managed clusters.
type Config struct {
	Namespace          string
	CreateNamespace    bool
	DeploymentName     string
	ServiceAccountName string
	SecretName         string
	PVCName            string
	PVCStorageClass    string
	PVCSize            string
	Image              string
	EventsURL          string
	Token              string
	LogLevel           string
	BatchMax           int
	BatchWindowMillis  int
	StorePath          string
	HeartbeatMinutes   int
	WaitForReady       bool
	ReadyTimeout       time.Duration
}

// TokenProvider resolves the API bearer token for a given cluster.
type TokenProvider func(ctx context.Context, clusterID string) (string, error)

// Option mutates a Deployer instance during construction.
type Option func(*Deployer)

// WithTokenProvider configures a dynamic token provider. When set, Config.Token is
// ignored and tokens are resolved per-cluster via the provider.
func WithTokenProvider(provider TokenProvider) Option {
	return func(d *Deployer) {
		d.tokenProvider = provider
	}
}

// Provisioner ensures the watcher deployment and RBAC are in place for a cluster.
type Provisioner interface {
	Ensure(ctx context.Context, clusterID, clusterName string, loader Loader) error
}

// Deployer applies the watcher manifests to a managed cluster.
type Deployer struct {
	cfg           Config
	factory       ClientFactory
	logger        *zap.Logger
	tokenProvider TokenProvider
}

// New constructs a Deployer, validating configuration and wiring the client factory.
func New(cfg Config, factory ClientFactory, opts ...Option) (*Deployer, error) {
	if factory == nil {
		return nil, errors.New("client factory required")
	}
	if strings.TrimSpace(cfg.Namespace) == "" {
		return nil, errors.New("namespace required")
	}
	if strings.TrimSpace(cfg.DeploymentName) == "" {
		return nil, errors.New("deployment name required")
	}
	if strings.TrimSpace(cfg.ServiceAccountName) == "" {
		return nil, errors.New("service account name required")
	}
	if strings.TrimSpace(cfg.SecretName) == "" {
		return nil, errors.New("secret name required")
	}
	if strings.TrimSpace(cfg.Image) == "" {
		return nil, errors.New("watcher image required")
	}
	if strings.TrimSpace(cfg.EventsURL) == "" {
		return nil, errors.New("kubeOP events URL required")
	}
	if cfg.WaitForReady && cfg.ReadyTimeout <= 0 {
		cfg.ReadyTimeout = 2 * time.Minute
	}
	if cfg.PVCName == "" {
		cfg.PVCName = cfg.DeploymentName + "-state"
	}
	if cfg.StorePath == "" {
		cfg.StorePath = "/var/lib/kubeop-watcher/state.db"
	}
	d := &Deployer{
		cfg:     cfg,
		factory: factory,
		logger:  logging.L().Named("watcher_deployer"),
	}
	for _, opt := range opts {
		opt(d)
	}
	if strings.TrimSpace(d.cfg.Token) == "" && d.tokenProvider == nil {
		return nil, errors.New("kubeOP token required")
	}
	return d, nil
}

// Ensure wires RBAC, configuration secrets, persistent storage, and the watcher Deployment.
func (d *Deployer) Ensure(ctx context.Context, clusterID, clusterName string, loader Loader) error {
	if loader == nil {
		return errors.New("kubeconfig loader required")
	}
	logger := d.logger.With(zap.String("cluster_id", clusterID))
	if clusterName != "" {
		logger = logger.With(zap.String("cluster_name", clusterName))
	}
	token, err := d.resolveToken(ctx, clusterID)
	if err != nil {
		logger.Error("failed to resolve watcher token", zap.String("error", err.Error()))
		return err
	}
	clientset, err := d.factory(ctx, clusterID, loader)
	if err != nil {
		logger.Error("failed to build clientset", zap.String("error", err.Error()))
		return fmt.Errorf("clientset: %w", err)
	}

	if err := d.ensureNamespace(ctx, clientset, logger); err != nil {
		return err
	}
	if err := d.ensureServiceAccount(ctx, clientset, logger); err != nil {
		return err
	}
	if err := d.ensureRBAC(ctx, clientset, logger); err != nil {
		return err
	}
	if err := d.ensureSecret(ctx, clientset, token, logger); err != nil {
		return err
	}
	if err := d.ensurePVC(ctx, clientset, logger); err != nil {
		return err
	}
	if err := d.ensureDeployment(ctx, clientset, clusterID, logger); err != nil {
		return err
	}

	if d.cfg.WaitForReady {
		logger.Info("waiting for watcher deployment availability", zap.Duration("timeout", d.cfg.ReadyTimeout))
		if err := d.waitForReady(ctx, clientset); err != nil {
			logger.Error("watcher deployment not ready", zap.String("error", err.Error()))
			return fmt.Errorf("watcher deployment readiness: %w", err)
		}
		logger.Info("watcher deployment ready")
	}

	return nil
}

func (d *Deployer) ensureNamespace(ctx context.Context, clientset kubernetes.Interface, logger *zap.Logger) error {
	_, err := clientset.CoreV1().Namespaces().Get(ctx, d.cfg.Namespace, metav1.GetOptions{})
	if err == nil {
		return nil
	}
	if !apierrors.IsNotFound(err) {
		logger.Error("get namespace failed", zap.String("error", err.Error()))
		return fmt.Errorf("get namespace: %w", err)
	}
	if !d.cfg.CreateNamespace {
		return fmt.Errorf("namespace %s does not exist and automatic creation disabled", d.cfg.Namespace)
	}
	ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: d.cfg.Namespace}}
	if _, err := clientset.CoreV1().Namespaces().Create(ctx, ns, metav1.CreateOptions{}); err != nil && !apierrors.IsAlreadyExists(err) {
		logger.Error("create namespace failed", zap.String("error", err.Error()))
		return fmt.Errorf("create namespace: %w", err)
	}
	logger.Info("namespace ensured", zap.String("namespace", d.cfg.Namespace))
	return nil
}

func (d *Deployer) ensureServiceAccount(ctx context.Context, clientset kubernetes.Interface, logger *zap.Logger) error {
	sa := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      d.cfg.ServiceAccountName,
			Namespace: d.cfg.Namespace,
			Labels: map[string]string{
				"app.kubernetes.io/name":       "kubeop-watcher",
				"app.kubernetes.io/component":  "bridge",
				"app.kubernetes.io/managed-by": "kubeop",
			},
		},
	}
	if _, err := clientset.CoreV1().ServiceAccounts(d.cfg.Namespace).Create(ctx, sa, metav1.CreateOptions{}); err != nil {
		if !apierrors.IsAlreadyExists(err) {
			logger.Error("create serviceaccount failed", zap.String("error", err.Error()))
			return fmt.Errorf("create serviceaccount: %w", err)
		}
		if _, err := clientset.CoreV1().ServiceAccounts(d.cfg.Namespace).Update(ctx, sa, metav1.UpdateOptions{}); err != nil {
			logger.Error("update serviceaccount failed", zap.String("error", err.Error()))
			return fmt.Errorf("update serviceaccount: %w", err)
		}
	}
	logger.Info("service account ensured", zap.String("serviceaccount", d.cfg.ServiceAccountName))
	return nil
}

func (d *Deployer) ensureRBAC(ctx context.Context, clientset kubernetes.Interface, logger *zap.Logger) error {
	cr := &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name: d.cfg.ServiceAccountName,
			Labels: map[string]string{
				"app.kubernetes.io/name":       "kubeop-watcher",
				"app.kubernetes.io/component":  "bridge",
				"app.kubernetes.io/managed-by": "kubeop",
			},
		},
		Rules: []rbacv1.PolicyRule{
			{APIGroups: []string{""}, Resources: []string{"pods", "services", "configmaps", "secrets", "events", "persistentvolumeclaims"}, Verbs: []string{"get", "list", "watch"}},
			{APIGroups: []string{"apps"}, Resources: []string{"deployments"}, Verbs: []string{"get", "list", "watch"}},
			{APIGroups: []string{"batch"}, Resources: []string{"jobs", "cronjobs"}, Verbs: []string{"get", "list", "watch"}},
			{APIGroups: []string{"autoscaling"}, Resources: []string{"horizontalpodautoscalers"}, Verbs: []string{"get", "list", "watch"}},
			{APIGroups: []string{"networking.k8s.io"}, Resources: []string{"ingresses"}, Verbs: []string{"get", "list", "watch"}},
			{APIGroups: []string{"cert-manager.io"}, Resources: []string{"certificates"}, Verbs: []string{"get", "list", "watch"}},
		},
	}
	if _, err := clientset.RbacV1().ClusterRoles().Create(ctx, cr, metav1.CreateOptions{}); err != nil {
		if !apierrors.IsAlreadyExists(err) {
			logger.Error("create clusterrole failed", zap.String("error", err.Error()))
			return fmt.Errorf("create clusterrole: %w", err)
		}
		if _, err := clientset.RbacV1().ClusterRoles().Update(ctx, cr, metav1.UpdateOptions{}); err != nil {
			logger.Error("update clusterrole failed", zap.String("error", err.Error()))
			return fmt.Errorf("update clusterrole: %w", err)
		}
	}

	crb := &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: d.cfg.ServiceAccountName,
			Labels: map[string]string{
				"app.kubernetes.io/name":       "kubeop-watcher",
				"app.kubernetes.io/component":  "bridge",
				"app.kubernetes.io/managed-by": "kubeop",
			},
		},
		Subjects: []rbacv1.Subject{{
			Kind:      "ServiceAccount",
			Name:      d.cfg.ServiceAccountName,
			Namespace: d.cfg.Namespace,
		}},
		RoleRef: rbacv1.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "ClusterRole",
			Name:     d.cfg.ServiceAccountName,
		},
	}
	if _, err := clientset.RbacV1().ClusterRoleBindings().Create(ctx, crb, metav1.CreateOptions{}); err != nil {
		if !apierrors.IsAlreadyExists(err) {
			logger.Error("create clusterrolebinding failed", zap.String("error", err.Error()))
			return fmt.Errorf("create clusterrolebinding: %w", err)
		}
		if _, err := clientset.RbacV1().ClusterRoleBindings().Update(ctx, crb, metav1.UpdateOptions{}); err != nil {
			logger.Error("update clusterrolebinding failed", zap.String("error", err.Error()))
			return fmt.Errorf("update clusterrolebinding: %w", err)
		}
	}
	logger.Info("rbac ensured")
	return nil
}

func (d *Deployer) ensureSecret(ctx context.Context, clientset kubernetes.Interface, token string, logger *zap.Logger) error {
	digest := sha256.Sum256([]byte(token))
	hash := fmt.Sprintf("%x", digest[:])
	sec := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      d.cfg.SecretName,
			Namespace: d.cfg.Namespace,
			Labels: map[string]string{
				"app.kubernetes.io/name":       "kubeop-watcher",
				"app.kubernetes.io/component":  "bridge",
				"app.kubernetes.io/managed-by": "kubeop",
			},
			Annotations: map[string]string{
				"kubeop.io/token-sha256": hash,
			},
		},
		Data: map[string][]byte{
			"token": []byte(token),
		},
		Type: corev1.SecretTypeOpaque,
	}
	if _, err := clientset.CoreV1().Secrets(d.cfg.Namespace).Create(ctx, sec, metav1.CreateOptions{}); err != nil {
		if !apierrors.IsAlreadyExists(err) {
			logger.Error("create secret failed", zap.String("error", err.Error()))
			return fmt.Errorf("create secret: %w", err)
		}
		existing, err := clientset.CoreV1().Secrets(d.cfg.Namespace).Get(ctx, d.cfg.SecretName, metav1.GetOptions{})
		if err != nil {
			logger.Error("get secret failed", zap.String("error", err.Error()))
			return fmt.Errorf("get secret: %w", err)
		}
		if existing.Data == nil {
			existing.Data = map[string][]byte{}
		}
		existing.Data["token"] = []byte(token)
		existing.Labels = sec.Labels
		if existing.Annotations == nil {
			existing.Annotations = map[string]string{}
		}
		existing.Annotations["kubeop.io/token-sha256"] = hash
		if _, err := clientset.CoreV1().Secrets(d.cfg.Namespace).Update(ctx, existing, metav1.UpdateOptions{}); err != nil {
			logger.Error("update secret failed", zap.String("error", err.Error()))
			return fmt.Errorf("update secret: %w", err)
		}
	}
	logger.Info("api token secret ensured", zap.String("secret", d.cfg.SecretName), zap.String("sha256", hash))
	return nil
}

func (d *Deployer) ensurePVC(ctx context.Context, clientset kubernetes.Interface, logger *zap.Logger) error {
	if strings.TrimSpace(d.cfg.PVCSize) == "" {
		return nil
	}
	quantity, err := resource.ParseQuantity(d.cfg.PVCSize)
	if err != nil {
		return fmt.Errorf("parse pvc size: %w", err)
	}
	pvc := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      d.cfg.PVCName,
			Namespace: d.cfg.Namespace,
			Labels: map[string]string{
				"app.kubernetes.io/name":       "kubeop-watcher",
				"app.kubernetes.io/component":  "bridge",
				"app.kubernetes.io/managed-by": "kubeop",
			},
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
			Resources: corev1.VolumeResourceRequirements{
				Requests: corev1.ResourceList{corev1.ResourceStorage: quantity},
			},
		},
	}
	if d.cfg.PVCStorageClass != "" {
		pvc.Spec.StorageClassName = pointer.String(d.cfg.PVCStorageClass)
	}
	if _, err := clientset.CoreV1().PersistentVolumeClaims(d.cfg.Namespace).Create(ctx, pvc, metav1.CreateOptions{}); err != nil {
		if !apierrors.IsAlreadyExists(err) {
			logger.Error("create pvc failed", zap.String("error", err.Error()))
			return fmt.Errorf("create pvc: %w", err)
		}
		if _, err := clientset.CoreV1().PersistentVolumeClaims(d.cfg.Namespace).Update(ctx, pvc, metav1.UpdateOptions{}); err != nil {
			logger.Error("update pvc failed", zap.String("error", err.Error()))
			return fmt.Errorf("update pvc: %w", err)
		}
	}
	logger.Info("state pvc ensured", zap.String("pvc", d.cfg.PVCName))
	return nil
}

func (d *Deployer) ensureDeployment(ctx context.Context, clientset kubernetes.Interface, clusterID string, logger *zap.Logger) error {
	labels := map[string]string{
		"app":                          "kubeop-watcher",
		"app.kubernetes.io/name":       "kubeop-watcher",
		"app.kubernetes.io/instance":   d.cfg.DeploymentName,
		"app.kubernetes.io/component":  "bridge",
		"app.kubernetes.io/managed-by": "kubeop",
	}

    // Derive base URL from the configured events URL for newer watcher images.
    baseURL := ""
    allowInsecure := "false"
    if u, err := url.Parse(strings.TrimSpace(d.cfg.EventsURL)); err == nil {
        if u.Scheme != "" && u.Host != "" {
            baseURL = (&url.URL{Scheme: u.Scheme, Host: u.Host}).String()
            if strings.ToLower(u.Scheme) == "http" {
                allowInsecure = "true"
            }
        }
    }

    env := []corev1.EnvVar{
        {Name: "CLUSTER_ID", Value: clusterID},
        // Backward compatibility
        {Name: "KUBEOP_EVENTS_URL", Value: d.cfg.EventsURL},
        // Preferred by newer watcher images
        {Name: "KUBEOP_BASE_URL", Value: strings.TrimSuffix(baseURL, "/")},
        {Name: "ALLOW_INSECURE_HTTP", Value: allowInsecure},
        {
            Name: "KUBEOP_TOKEN",
            ValueFrom: &corev1.EnvVarSource{SecretKeyRef: &corev1.SecretKeySelector{
                LocalObjectReference: corev1.LocalObjectReference{Name: d.cfg.SecretName},
                Key:                  "token",
            }},
        },
        {Name: "STORE_PATH", Value: d.cfg.StorePath},
    }
	if d.cfg.LogLevel != "" {
		env = append(env, corev1.EnvVar{Name: "LOG_LEVEL", Value: d.cfg.LogLevel})
	}
	if d.cfg.BatchMax > 0 {
		env = append(env, corev1.EnvVar{Name: "BATCH_MAX", Value: fmt.Sprintf("%d", d.cfg.BatchMax)})
	}
	if d.cfg.BatchWindowMillis > 0 {
		env = append(env, corev1.EnvVar{Name: "BATCH_WINDOW_MS", Value: fmt.Sprintf("%d", d.cfg.BatchWindowMillis)})
	}
	if d.cfg.HeartbeatMinutes > 0 {
		env = append(env, corev1.EnvVar{Name: "HEARTBEAT_MINUTES", Value: fmt.Sprintf("%d", d.cfg.HeartbeatMinutes)})
	}

	volumes := []corev1.Volume{}
	volumeMounts := []corev1.VolumeMount{}
	if strings.TrimSpace(d.cfg.PVCSize) != "" {
		volumes = append(volumes, corev1.Volume{
			Name:         "state",
			VolumeSource: corev1.VolumeSource{PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{ClaimName: d.cfg.PVCName}},
		})
	} else {
		volumes = append(volumes, corev1.Volume{
			Name:         "state",
			VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{}},
		})
	}
	volumeMounts = append(volumeMounts, corev1.VolumeMount{Name: "state", MountPath: "/var/lib/kubeop-watcher"})

	dep := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      d.cfg.DeploymentName,
			Namespace: d.cfg.Namespace,
			Labels:    labels,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: pointer.Int32(1),
			Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": "kubeop-watcher", "app.kubernetes.io/instance": d.cfg.DeploymentName}},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: labels},
				Spec: corev1.PodSpec{
					SecurityContext: &corev1.PodSecurityContext{
						RunAsNonRoot:   pointer.Bool(true),
						SeccompProfile: &corev1.SeccompProfile{Type: corev1.SeccompProfileTypeRuntimeDefault},
					},
					ServiceAccountName: d.cfg.ServiceAccountName,
					Containers: []corev1.Container{{
						Name:            "watcher",
                    Image:           d.cfg.Image,
                    ImagePullPolicy: corev1.PullAlways,
						Env:             env,
						Ports: []corev1.ContainerPort{{
							Name:          "http",
							ContainerPort: 8081,
						}},
						SecurityContext: &corev1.SecurityContext{
							AllowPrivilegeEscalation: pointer.Bool(false),
							Capabilities:             &corev1.Capabilities{Drop: []corev1.Capability{"ALL"}},
							RunAsNonRoot:             pointer.Bool(true),
						},
						LivenessProbe: &corev1.Probe{
							ProbeHandler:        corev1.ProbeHandler{HTTPGet: &corev1.HTTPGetAction{Path: "/healthz", Port: intstr.FromString("http")}},
							InitialDelaySeconds: 10,
							PeriodSeconds:       15,
						},
						ReadinessProbe: &corev1.Probe{
							ProbeHandler:        corev1.ProbeHandler{HTTPGet: &corev1.HTTPGetAction{Path: "/readyz", Port: intstr.FromString("http")}},
							InitialDelaySeconds: 5,
							PeriodSeconds:       10,
						},
						VolumeMounts: volumeMounts,
					}},
					Volumes: volumes,
				},
			},
		},
	}
	if _, err := clientset.AppsV1().Deployments(d.cfg.Namespace).Create(ctx, dep, metav1.CreateOptions{}); err != nil {
		if !apierrors.IsAlreadyExists(err) {
			logger.Error("create deployment failed", zap.String("error", err.Error()))
			return fmt.Errorf("create deployment: %w", err)
		}
		existing, err := clientset.AppsV1().Deployments(d.cfg.Namespace).Get(ctx, d.cfg.DeploymentName, metav1.GetOptions{})
		if err != nil {
			logger.Error("get deployment failed", zap.String("error", err.Error()))
			return fmt.Errorf("get deployment: %w", err)
		}
		existing.Spec = dep.Spec
		existing.Labels = dep.Labels
		existing.Spec.Template.Labels = dep.Spec.Template.Labels
		if _, err := clientset.AppsV1().Deployments(d.cfg.Namespace).Update(ctx, existing, metav1.UpdateOptions{}); err != nil {
			logger.Error("update deployment failed", zap.String("error", err.Error()))
			return fmt.Errorf("update deployment: %w", err)
		}
	}
	logger.Info("watcher deployment ensured", zap.String("deployment", d.cfg.DeploymentName))
	return nil
}

func (d *Deployer) resolveToken(ctx context.Context, clusterID string) (string, error) {
	if token := strings.TrimSpace(d.cfg.Token); token != "" {
		return token, nil
	}
	if d.tokenProvider == nil {
		return "", errors.New("kubeOP token required")
	}
	token, err := d.tokenProvider(ctx, clusterID)
	if err != nil {
		return "", fmt.Errorf("resolve token: %w", err)
	}
	if strings.TrimSpace(token) == "" {
		return "", errors.New("token provider returned empty token")
	}
	return token, nil
}

func (d *Deployer) waitForReady(ctx context.Context, clientset kubernetes.Interface) error {
	return wait.PollUntilContextTimeout(ctx, 5*time.Second, d.cfg.ReadyTimeout, true, func(ctx context.Context) (bool, error) {
		dep, err := clientset.AppsV1().Deployments(d.cfg.Namespace).Get(ctx, d.cfg.DeploymentName, metav1.GetOptions{})
		if err != nil {
			if apierrors.IsNotFound(err) {
				return false, nil
			}
			return false, err
		}
		if dep.Status.UpdatedReplicas >= 1 && dep.Status.AvailableReplicas >= 1 {
			return true, nil
		}
		return false, nil
	})
}
