package api

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"
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
		r.Group(func(r chi.Router) {
			r.Use(AdminAuthMiddleware(cfg))

			r.Post("/apps/validate", a.validateApp)

			r.Route("/clusters", func(r chi.Router) {
				r.Post("/", a.createCluster)
				r.Get("/", a.listClusters)
				r.Get("/{id}", a.getCluster)
				r.Patch("/{id}", a.updateCluster)
				r.Get("/health", a.clustersHealth)
				r.Get("/{id}/health", a.clusterHealth)
				r.Get("/{id}/status", a.clusterStatus)
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
				r.Get("/{id}/apps/{appId}/releases", a.listAppReleases)
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
				r.Post("/{id}/templates/{templateId}/deploy", a.deployTemplate)
			})

			r.Route("/kubeconfigs", func(r chi.Router) {
				r.Post("/", a.ensureKubeconfig)
				r.Post("/rotate", a.rotateKubeconfig)
				r.Delete("/{id}", a.deleteKubeconfig)
			})

			r.Route("/credentials", func(r chi.Router) {
				r.Route("/git", func(r chi.Router) {
					r.Post("/", a.createGitCredential)
					r.Get("/", a.listGitCredentials)
					r.Get("/{id}", a.getGitCredential)
					r.Delete("/{id}", a.deleteGitCredential)
				})
				r.Route("/registries", func(r chi.Router) {
					r.Post("/", a.createRegistryCredential)
					r.Get("/", a.listRegistryCredentials)
					r.Get("/{id}", a.getRegistryCredential)
					r.Delete("/{id}", a.deleteRegistryCredential)
				})
			})

			// templates
			r.Route("/templates", func(r chi.Router) {
				r.Post("/", a.createTemplate)
				r.Get("/", a.listTemplates)
				r.Get("/{id}", a.getTemplate)
				r.Post("/{id}/render", a.renderTemplate)
			})

			r.Route("/admin", func(r chi.Router) {
				r.Get("/maintenance", a.getMaintenance)
				r.Put("/maintenance", a.updateMaintenance)
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
	meta := version.Metadata()
	resp := map[string]any{
		"version": meta.Build.Version,
		"commit":  meta.Build.Commit,
		"date":    meta.Build.Date,
		"compatibility": map[string]string{
			"minClientVersion": meta.Compatibility.MinClientVersion,
			"minApiVersion":    meta.Compatibility.MinAPIVersion,
			"maxApiVersion":    meta.Compatibility.MaxAPIVersion,
		},
	}
	if dep := meta.Deprecation; dep != nil {
		depResp := map[string]string{}
		if dep.Deadline != "" {
			depResp["deadline"] = dep.Deadline
		}
		if dep.Note != "" {
			depResp["note"] = dep.Note
		}
		if len(depResp) > 0 {
			resp["deprecation"] = depResp
		}
	}
	if meta.Deprecated(time.Now().UTC()) {
		fields := []zap.Field{
			zap.String("version", meta.Build.Version),
		}
		if deadline, ok := meta.DeadlineTime(); ok {
			fields = append(fields, zap.Time("deprecation_deadline", deadline))
		}
		logging.L().Warn("deprecated kubeOP build queried via /v1/version", fields...)
	}
	writeJSON(w, http.StatusOK, resp)
}

type createClusterReq struct {
	Name       string `json:"name"`
	Kubeconfig string `json:"kubeconfig"`
	// Optional: base64-encoded kubeconfig. If provided, it takes precedence over Kubeconfig.
	KubeconfigB64 string   `json:"kubeconfig_b64"`
	Owner         string   `json:"owner,omitempty"`
	Contact       string   `json:"contact,omitempty"`
	Environment   string   `json:"environment,omitempty"`
	Region        string   `json:"region,omitempty"`
	APIServer     string   `json:"apiServer,omitempty"`
	Description   string   `json:"description,omitempty"`
	Tags          []string `json:"tags,omitempty"`
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
		if writeMaintenanceError(w, err) {
			return
		}
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	c, err := svc.RegisterCluster(r.Context(), service.ClusterRegisterInput{
		Name:       strings.TrimSpace(req.Name),
		Kubeconfig: kubeconfig,
		Metadata: service.ClusterMetadataInput{
			Owner:       req.Owner,
			Contact:     req.Contact,
			Environment: req.Environment,
			Region:      req.Region,
			APIServer:   req.APIServer,
			Description: req.Description,
			Tags:        req.Tags,
		},
	})
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

func (a *API) getCluster(w http.ResponseWriter, r *http.Request) {
	svc, ok := a.serviceOrError(w, "getCluster")
	if !ok {
		return
	}
	id := chi.URLParam(r, "id")
	c, err := svc.GetCluster(r.Context(), id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
			return
		}
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, c)
}

type updateClusterReq struct {
	Owner       string   `json:"owner,omitempty"`
	Contact     string   `json:"contact,omitempty"`
	Environment string   `json:"environment,omitempty"`
	Region      string   `json:"region,omitempty"`
	APIServer   string   `json:"apiServer,omitempty"`
	Description string   `json:"description,omitempty"`
	Tags        []string `json:"tags,omitempty"`
}

func (a *API) updateCluster(w http.ResponseWriter, r *http.Request) {
	svc, ok := a.serviceOrError(w, "updateCluster")
	if !ok {
		return
	}
	id := chi.URLParam(r, "id")
	var req updateClusterReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}
	c, err := svc.UpdateClusterMetadata(r.Context(), id, service.ClusterMetadataInput{
		Owner:       req.Owner,
		Contact:     req.Contact,
		Environment: req.Environment,
		Region:      req.Region,
		APIServer:   req.APIServer,
		Description: req.Description,
		Tags:        req.Tags,
	})
	if err != nil {
		if writeMaintenanceError(w, err) {
			return
		}
		status := http.StatusBadRequest
		if errors.Is(err, sql.ErrNoRows) {
			status = http.StatusNotFound
		}
		writeJSON(w, status, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, c)
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

func (a *API) clusterStatus(w http.ResponseWriter, r *http.Request) {
	svc, ok := a.serviceOrError(w, "clusterStatus")
	if !ok {
		return
	}
	id := chi.URLParam(r, "id")
	limit := 20
	if q := strings.TrimSpace(r.URL.Query().Get("limit")); q != "" {
		if v, err := strconv.Atoi(q); err == nil && v > 0 {
			if v > 100 {
				v = 100
			}
			limit = v
		}
	}
	status, err := svc.ListClusterStatus(r.Context(), id, limit)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, status)
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(true)
	_ = enc.Encode(v)
}
