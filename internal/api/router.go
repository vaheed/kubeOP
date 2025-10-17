package api

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"reflect"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"
	"github.com/golang-jwt/jwt/v5"
	"go.uber.org/zap"
	"kubeop/internal/config"
	httpmw "kubeop/internal/http/middleware"
	"kubeop/internal/logging"
	"kubeop/internal/metrics"
	"kubeop/internal/service"
	"kubeop/internal/util"
	"kubeop/internal/version"
)

type API struct {
	cfg *config.Config
	svc *service.Service
	hc  HealthChecker
}

type Option func(*API)

type HealthChecker interface {
	Health(context.Context) error
}

func WithHealthChecker(h HealthChecker) Option {
	return func(a *API) {
		a.hc = h
	}
}

func NewRouter(cfg *config.Config, svc *service.Service, opts ...Option) http.Handler {
	a := &API{cfg: cfg, svc: svc, hc: svc}
	for _, opt := range opts {
		if opt != nil {
			opt(a)
		}
	}
	r := chi.NewRouter()

	r.Use(chimw.RequestID)
	r.Use(chimw.RealIP)
	r.Use(chimw.Recoverer)
	r.Use(httpmw.AccessLog)
	r.Use(httpmw.AuditLog)

	r.Get("/healthz", a.healthz)
	r.Get("/readyz", a.readyz)
	r.Get("/v1/version", a.version)
	// metrics outside auth
	r.Get("/metrics", a.metrics)

	r.Route("/v1", func(r chi.Router) {
		r.Route("/watchers", func(r chi.Router) {
			r.Post("/register", a.registerWatcher)
			r.Post("/refresh", a.refreshWatcherTokens)
			r.Group(func(r chi.Router) {
				r.Use(WatcherAuthMiddleware(cfg, svc))
				r.Post("/handshake", a.watcherHandshake)
			})
		})

		r.Group(func(r chi.Router) {
			r.Use(WatcherAuthMiddleware(cfg, svc))
			r.Post("/events/ingest", a.ingestWatcherEvents)
		})

		r.Group(func(r chi.Router) {
			r.Use(AdminAuthMiddleware(cfg))

			r.Route("/clusters", func(r chi.Router) {
				r.Post("/", a.createCluster)
				r.Get("/", a.listClusters)
				r.Get("/health", a.clustersHealth)
				r.Get("/{id}/health", a.clusterHealth)
			})

			r.Route("/users", func(r chi.Router) {
				r.Post("/bootstrap", a.bootstrapUser)
				r.Get("/", a.listUsers)
				r.Get("/{id}", a.getUser)
				r.Delete("/{id}", a.deleteUser)
				r.Post("/{id}/kubeconfig/renew", a.renewUserKubeconfig)
				r.Get("/{id}/projects", a.listUserProjects)
			})

			r.Route("/projects", func(r chi.Router) {
				r.Get("/", a.listProjects)
				r.Post("/", a.createProject)
				r.Get("/{id}", a.getProject)
				r.Get("/{id}/quota", a.getProjectQuota)
				r.Patch("/{id}/quota", a.patchProjectQuota)
				r.Post("/{id}/suspend", a.suspendProject)
				r.Post("/{id}/unsuspend", a.unsuspendProject)
				r.Delete("/{id}", a.deleteProject)
				// apps
				r.Get("/{id}/apps", a.listProjectApps)
				r.Post("/{id}/apps", a.deployApp)
				r.Get("/{id}/logs", a.projectLogs)
				r.Get("/{id}/events", a.listProjectEvents)
				r.Post("/{id}/events", a.appendProjectEvent)
				r.Get("/{id}/apps/{appId}/logs", a.appLogs)
				r.Get("/{id}/apps/{appId}", a.getProjectApp)
				r.Delete("/{id}/apps/{appId}", a.deleteApp)
				r.Patch("/{id}/apps/{appId}/scale", a.scaleApp)
				r.Patch("/{id}/apps/{appId}/image", a.updateAppImage)
				r.Post("/{id}/apps/{appId}/rollout/restart", a.rolloutRestartApp)
				// kubeconfig lifecycle
				r.Post("/{id}/kubeconfig/renew", a.renewProjectKubeconfig)
				// configs and secrets
				r.Post("/{id}/configs", a.createConfig)
				r.Get("/{id}/configs", a.listConfigs)
				r.Delete("/{id}/configs/{name}", a.deleteConfig)
				r.Post("/{id}/secrets", a.createSecret)
				r.Get("/{id}/secrets", a.listSecrets)
				r.Delete("/{id}/secrets/{name}", a.deleteSecret)
				r.Post("/{id}/apps/{appId}/configs/attach", a.attachConfigToApp)
				r.Post("/{id}/apps/{appId}/configs/detach", a.detachConfigFromApp)
				r.Post("/{id}/apps/{appId}/secrets/attach", a.attachSecretToApp)
				r.Post("/{id}/apps/{appId}/secrets/detach", a.detachSecretFromApp)
			})

			r.Route("/kubeconfigs", func(r chi.Router) {
				r.Post("/", a.ensureKubeconfig)
				r.Post("/rotate", a.rotateKubeconfig)
				r.Delete("/{id}", a.deleteKubeconfig)
			})

			// templates
			r.Route("/templates", func(r chi.Router) {
				r.Post("/", a.createTemplate)
			})

			// webhooks
			r.Post("/webhooks/git", a.gitWebhook)
		})
	})

	return r
}

func (a *API) healthz(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"status": "ok"})
}

func (a *API) readyz(w http.ResponseWriter, r *http.Request) {
	logger := logging.L()
	checker := a.hc
	if isNilHealthChecker(checker) {
		checker = a.svc
	}
	if isNilHealthChecker(checker) {
		metrics.ObserveReadyzFailure("service_missing")
		logger.Warn("readyz", zap.String("event", "readyz_failure"), zap.String("status", "service_missing"))
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"status": "not_ready", "error": "service unavailable"})
		return
	}

	ctx := r.Context()
	if err := checker.Health(ctx); err != nil {
		metrics.ObserveReadyzFailure("health_check_failed")
		logger.Warn(
			"readyz",
			zap.String("event", "readyz_failure"),
			zap.String("status", "health_check_failed"),
			zap.String("error", err.Error()),
		)
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"status": "not_ready", "error": err.Error()})
		return
	}
	logger.Info("readyz", zap.String("event", "readyz_ok"), zap.String("status", "ready"))
	writeJSON(w, http.StatusOK, map[string]any{"status": "ready"})
}

func isNilHealthChecker(h HealthChecker) bool {
	if h == nil {
		return true
	}
	v := reflect.ValueOf(h)
	switch v.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return v.IsNil()
	default:
		return false
	}
}

func (a *API) version(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{
		"version": version.Version,
		"commit":  version.Commit,
		"date":    version.Date,
	})
}

func (a *API) watcherHandshake(w http.ResponseWriter, r *http.Request) {
	svc, ok := a.serviceOrError(w, "watcherHandshake")
	if !ok {
		return
	}
	claims := claimsFromContext(r.Context())
	bodyClusterID := ""
	if r.Body != nil {
		defer r.Body.Close()
		limited := io.LimitReader(r.Body, 1<<20)
		var payload struct {
			ClusterID string `json:"cluster_id"`
		}
		dec := json.NewDecoder(limited)
		if err := dec.Decode(&payload); err != nil && err != io.EOF {
			writeJSON(w, http.StatusBadRequest, map[string]any{"status": "error", "error": "invalid payload"})
			return
		}
		bodyClusterID = strings.TrimSpace(payload.ClusterID)
	}

	claimClusterID := strings.TrimSpace(clusterIDFromClaims(claims))
	if claimClusterID == "" {
		claimClusterID = bodyClusterID
	}
	if claimClusterID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"status": "error", "error": "cluster_id missing"})
		return
	}
	if bodyClusterID != "" && bodyClusterID != claimClusterID {
		writeJSON(w, http.StatusBadRequest, map[string]any{"status": "error", "error": "cluster_id mismatch"})
		return
	}
	watcherID := watcherIDFromClaims(claims)
	if watcherID != "" {
		if err := svc.RecordWatcherHandshake(r.Context(), watcherID); err != nil {
			logging.L().Warn("watcher_handshake_record_failed", zap.String("watcher_id", watcherID), zap.Error(err))
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"status": "ok", "cluster_id": claimClusterID})
}

func (a *API) registerWatcher(w http.ResponseWriter, r *http.Request) {
	svc, ok := a.serviceOrError(w, "registerWatcher")
	if !ok {
		return
	}
	if a.cfg != nil && !a.cfg.K8SEventsBridge {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "k8s ingest disabled"})
		return
	}
	var payload struct {
		ClusterID string `json:"cluster_id"`
	}
	if r.Body != nil {
		defer r.Body.Close()
		dec := json.NewDecoder(io.LimitReader(r.Body, 1<<20))
		if err := dec.Decode(&payload); err != nil && !errors.Is(err, io.EOF) {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid payload"})
			return
		}
	}
	payload.ClusterID = strings.TrimSpace(payload.ClusterID)
	if payload.ClusterID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "cluster_id required"})
		return
	}
	if err := a.authorizeWatcherBootstrap(r, payload.ClusterID); err != nil {
		status := http.StatusUnauthorized
		if errors.Is(err, errBootstrapForbidden) {
			status = http.StatusForbidden
		}
		writeJSON(w, status, map[string]string{"error": err.Error()})
		return
	}
	creds, err := svc.RegisterWatcher(r.Context(), payload.ClusterID)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	writeWatcherCredentials(w, http.StatusCreated, creds)
}

func (a *API) refreshWatcherTokens(w http.ResponseWriter, r *http.Request) {
	svc, ok := a.serviceOrError(w, "refreshWatcherTokens")
	if !ok {
		return
	}
	var payload struct {
		WatcherID    string `json:"watcher_id"`
		RefreshToken string `json:"refresh_token"`
		ClusterID    string `json:"cluster_id"`
	}
	if r.Body != nil {
		defer r.Body.Close()
		dec := json.NewDecoder(io.LimitReader(r.Body, 1<<20))
		if err := dec.Decode(&payload); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid payload"})
			return
		}
	}
	payload.WatcherID = strings.TrimSpace(payload.WatcherID)
	payload.RefreshToken = strings.TrimSpace(payload.RefreshToken)
	payload.ClusterID = strings.TrimSpace(payload.ClusterID)
	if payload.WatcherID == "" || payload.RefreshToken == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "watcher_id and refresh_token required"})
		return
	}
	creds, err := svc.RefreshWatcherTokens(r.Context(), payload.WatcherID, payload.ClusterID, payload.RefreshToken)
	if err != nil {
		status := http.StatusBadRequest
		if strings.Contains(err.Error(), "invalid refresh token") || strings.Contains(err.Error(), "expired") {
			status = http.StatusUnauthorized
		}
		writeJSON(w, status, map[string]string{"error": err.Error()})
		return
	}
	writeWatcherCredentials(w, http.StatusOK, creds)
}

var errBootstrapForbidden = errors.New("bootstrap token forbidden")

func (a *API) authorizeWatcherBootstrap(r *http.Request, clusterID string) error {
	if a == nil || a.cfg == nil {
		return errors.New("configuration missing")
	}
	authz := strings.TrimSpace(r.Header.Get("Authorization"))
	if authz == "" || !strings.HasPrefix(authz, "Bearer ") {
		return errors.New("bootstrap token required")
	}
	token := strings.TrimSpace(strings.TrimPrefix(authz, "Bearer "))
	if token == "" {
		return errors.New("bootstrap token required")
	}
	if secret := strings.TrimSpace(a.cfg.WatcherToken); secret != "" {
		if subtle.ConstantTimeCompare([]byte(token), []byte(secret)) == 1 {
			return nil
		}
	}
	adminSecret := strings.TrimSpace(a.cfg.AdminJWTSecret)
	if adminSecret == "" {
		return errors.New("bootstrap token invalid")
	}
	parsed, err := jwt.Parse(token, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, jwt.ErrSignatureInvalid
		}
		return []byte(adminSecret), nil
	})
	if err != nil || !parsed.Valid {
		return errors.New("bootstrap token invalid")
	}
	claims, _ := parsed.Claims.(jwt.MapClaims)
	if claims == nil {
		return errors.New("bootstrap token invalid")
	}
	if role, ok := claims["role"].(string); ok && strings.TrimSpace(role) != "admin" {
		return errBootstrapForbidden
	}
	claimCluster := clusterIDFromClaims(claims)
	if clusterID != "" && claimCluster != "" && clusterID != claimCluster {
		return errors.New("cluster mismatch")
	}
	return nil
}

func writeWatcherCredentials(w http.ResponseWriter, status int, creds service.WatcherCredentials) {
	resp := map[string]any{
		"watcherId":        creds.WatcherID,
		"clusterId":        creds.ClusterID,
		"accessToken":      creds.AccessToken,
		"accessExpiresAt":  creds.AccessExpiresAt.UTC().Format(time.RFC3339),
		"refreshToken":     creds.RefreshToken,
		"refreshExpiresAt": creds.RefreshExpiresAt.UTC().Format(time.RFC3339),
	}
	writeJSON(w, status, resp)
}

type createClusterReq struct {
	Name       string `json:"name"`
	Kubeconfig string `json:"kubeconfig"`
	// Optional: base64-encoded kubeconfig. If provided, it takes precedence over Kubeconfig.
	KubeconfigB64 string `json:"kubeconfig_b64"`
}

func (a *API) createCluster(w http.ResponseWriter, r *http.Request) {
	svc, ok := a.serviceOrError(w, "createCluster")
	if !ok {
		return
	}
	var req createClusterReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}
	kubeconfig, err := util.DecodeKubeconfig(req.Kubeconfig, req.KubeconfigB64)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	c, err := svc.RegisterCluster(r.Context(), strings.TrimSpace(req.Name), kubeconfig)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusCreated, c)
}

func (a *API) listClusters(w http.ResponseWriter, r *http.Request) {
	svc, ok := a.serviceOrError(w, "listClusters")
	if !ok {
		return
	}
	cs, err := svc.ListClusters(r.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, cs)
}

func (a *API) clustersHealth(w http.ResponseWriter, r *http.Request) {
	svc, ok := a.serviceOrError(w, "clustersHealth")
	if !ok {
		return
	}
	hs, err := svc.CheckAllClusters(r.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, hs)
}

// createUser removed in v0.1.1

func (a *API) listUsers(w http.ResponseWriter, r *http.Request) {
	svc, ok := a.serviceOrError(w, "listUsers")
	if !ok {
		return
	}
	// Accept optional pagination via query params
	users, err := svc.ListUsers(r.Context(), 100, 0)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, users)
}

func (a *API) getUser(w http.ResponseWriter, r *http.Request) {
	svc, ok := a.serviceOrError(w, "getUser")
	if !ok {
		return
	}
	id := chi.URLParam(r, "id")
	u, err := svc.GetUser(r.Context(), id)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
		return
	}
	writeJSON(w, http.StatusOK, u)
}

func (a *API) clusterHealth(w http.ResponseWriter, r *http.Request) {
	svc, ok := a.serviceOrError(w, "clusterHealth")
	if !ok {
		return
	}
	id := chi.URLParam(r, "id")
	h, err := svc.CheckCluster(r.Context(), id)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, h)
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(true)
	_ = enc.Encode(v)
}
