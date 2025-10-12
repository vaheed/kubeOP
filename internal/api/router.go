package api

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"kubeop/internal/config"
	"kubeop/internal/service"
	"kubeop/internal/util"
	"kubeop/internal/version"
)

type API struct {
	cfg *config.Config
	svc *service.Service
}

func NewRouter(cfg *config.Config, svc *service.Service) http.Handler {
	a := &API{cfg: cfg, svc: svc}
	r := chi.NewRouter()

	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Recoverer)
	r.Use(LoggingMiddleware)

	r.Get("/healthz", a.healthz)
	r.Get("/readyz", a.readyz)
	r.Get("/v1/version", a.version)
	// metrics outside auth
	r.Get("/metrics", a.metrics)

	r.Route("/v1", func(r chi.Router) {
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
			r.Patch("/{id}/quota", a.patchProjectQuota)
			r.Post("/{id}/suspend", a.suspendProject)
			r.Post("/{id}/unsuspend", a.unsuspendProject)
			r.Delete("/{id}", a.deleteProject)
			// apps
			r.Get("/{id}/apps", a.listProjectApps)
			r.Post("/{id}/apps", a.deployApp)
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

		// templates
		r.Route("/templates", func(r chi.Router) {
			r.Post("/", a.createTemplate)
		})

		// webhooks
		r.Post("/webhooks/git", a.gitWebhook)
	})

	return r
}

func (a *API) healthz(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"status": "ok"})
}

func (a *API) readyz(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if err := a.svc.Health(ctx); err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"status": "not_ready", "error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"status": "ready"})
}

func (a *API) version(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{
		"version": version.Version,
		"commit":  version.Commit,
		"date":    version.Date,
	})
}

type createClusterReq struct {
	Name       string `json:"name"`
	Kubeconfig string `json:"kubeconfig"`
	// Optional: base64-encoded kubeconfig. If provided, it takes precedence over Kubeconfig.
	KubeconfigB64 string `json:"kubeconfig_b64"`
}

func (a *API) createCluster(w http.ResponseWriter, r *http.Request) {
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
	c, err := a.svc.RegisterCluster(r.Context(), strings.TrimSpace(req.Name), kubeconfig)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusCreated, c)
}

func (a *API) listClusters(w http.ResponseWriter, r *http.Request) {
	cs, err := a.svc.ListClusters(r.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, cs)
}

func (a *API) clustersHealth(w http.ResponseWriter, r *http.Request) {
	hs, err := a.svc.CheckAllClusters(r.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, hs)
}

// createUser removed in v0.1.1

func (a *API) listUsers(w http.ResponseWriter, r *http.Request) {
	// Accept optional pagination via query params
	users, err := a.svc.ListUsers(r.Context(), 100, 0)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, users)
}

func (a *API) getUser(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	u, err := a.svc.GetUser(r.Context(), id)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
		return
	}
	writeJSON(w, http.StatusOK, u)
}

func (a *API) clusterHealth(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	h, err := a.svc.CheckCluster(r.Context(), id)
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

func LoggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t0 := time.Now()
		next.ServeHTTP(w, r)
		slog.Info("request", slog.String("method", r.Method), slog.String("path", r.URL.Path), slog.Duration("dur", time.Since(t0)))
	})
}
