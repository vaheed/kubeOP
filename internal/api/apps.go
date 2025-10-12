package api

import (
	"encoding/json"
	"io"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"kubeop/internal/service"
)

// -------- Templates --------

type createTemplateReq struct {
	Name string         `json:"name"`
	Kind string         `json:"kind"` // helm | manifests | blueprint
	Spec map[string]any `json:"spec"`
}

func (a *API) createTemplate(w http.ResponseWriter, r *http.Request) {
	var req createTemplateReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}
	out, err := a.svc.CreateTemplate(r.Context(), service.TemplateCreateInput{
		Name: req.Name,
		Kind: req.Kind,
		Spec: req.Spec,
	})
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusCreated, out)
}

// -------- Apps (Deploy) --------

type appPort struct {
	ContainerPort int32  `json:"containerPort"`
	ServicePort   int32  `json:"servicePort"`
	Protocol      string `json:"protocol,omitempty"`    // TCP/UDP
	ServiceType   string `json:"serviceType,omitempty"` // ClusterIP|LoadBalancer
}

type deployAppReq struct {
	Name      string            `json:"name"`
	Flavor    string            `json:"flavor,omitempty"`
	Resources map[string]string `json:"resources,omitempty"` // custom overrides
	Replicas  *int32            `json:"replicas,omitempty"`
	Env       map[string]string `json:"env,omitempty"`
	Secrets   []string          `json:"secrets,omitempty"` // envFrom secretRef
	Ports     []appPort         `json:"ports,omitempty"`
	Domain    string            `json:"domain,omitempty"` // optional explicit domain
	Repo      string            `json:"repo,omitempty"`   // optional repo to link for CI webhooks

	// one-of source
	Image         string         `json:"image,omitempty"`
	Helm          map[string]any `json:"helm,omitempty"`
	Manifests     []string       `json:"manifests,omitempty"` // raw YAML docs
	WebhookSecret string         `json:"webhookSecret,omitempty"`
}

func (a *API) deployApp(w http.ResponseWriter, r *http.Request) {
	projectID := chi.URLParam(r, "id")
	var req deployAppReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}
	var ports []service.AppPort
	for _, p := range req.Ports {
		ports = append(ports, service.AppPort{ContainerPort: p.ContainerPort, ServicePort: p.ServicePort, Protocol: p.Protocol, ServiceType: p.ServiceType})
	}
	out, err := a.svc.DeployApp(r.Context(), service.AppDeployInput{
		ProjectID:     projectID,
		Name:          req.Name,
		Flavor:        req.Flavor,
		Resources:     req.Resources,
		Replicas:      req.Replicas,
		Env:           req.Env,
		Secrets:       req.Secrets,
		Ports:         ports,
		Domain:        req.Domain,
		Repo:          req.Repo,
		WebhookSecret: req.WebhookSecret,
		Image:         req.Image,
		Helm:          req.Helm,
		Manifests:     req.Manifests,
	})
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusCreated, out)
}

// List apps for a project (with summary status)
func (a *API) listProjectApps(w http.ResponseWriter, r *http.Request) {
	projectID := chi.URLParam(r, "id")
	sts, err := a.svc.ListProjectAppsStatus(r.Context(), projectID)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, sts)
}

// Get a single app with detailed status
func (a *API) getProjectApp(w http.ResponseWriter, r *http.Request) {
	projectID := chi.URLParam(r, "id")
	appID := chi.URLParam(r, "appId")
	st, err := a.svc.GetAppStatus(r.Context(), projectID, appID)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, st)
}

// -------- Logs --------

func (a *API) appLogs(w http.ResponseWriter, r *http.Request) {
	projectID := chi.URLParam(r, "id")
	appID := chi.URLParam(r, "appId")
	container := r.URL.Query().Get("container")
	tailStr := r.URL.Query().Get("tailLines")
	followStr := r.URL.Query().Get("follow")
	var tail *int64
	if tailStr != "" {
		if v, err := strconv.ParseInt(tailStr, 10, 64); err == nil {
			tail = &v
		}
	}
	follow := true
	if followStr != "" {
		if v, err := strconv.ParseBool(followStr); err == nil {
			follow = v
		}
	}
	rc, closer, err := a.svc.StreamAppLogs(r.Context(), service.AppLogsInput{
		ProjectID: projectID,
		AppID:     appID,
		Container: container,
		TailLines: tail,
		Follow:    follow,
	})
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	defer closer()
	w.Header().Set("Content-Type", "text/plain")
	w.WriteHeader(http.StatusOK)
	// stream
	_, _ = io.Copy(w, rc)
}

// -------- Kubeconfig Renew --------

func (a *API) renewProjectKubeconfig(w http.ResponseWriter, r *http.Request) {
	projectID := chi.URLParam(r, "id")
	out, err := a.svc.RenewProjectKubeconfig(r.Context(), projectID)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, out)
}

// -------- App scale/update/rollout --------

type scaleReq struct {
	Replicas int32 `json:"replicas"`
}
type imageReq struct {
	Image string `json:"image"`
}
type attachConfigReq struct {
	Name   string   `json:"name"`
	Keys   []string `json:"keys,omitempty"`
	Prefix string   `json:"prefix,omitempty"`
}
type detachConfigReq struct {
	Name string `json:"name"`
}
type attachSecretReq struct {
	Name   string   `json:"name"`
	Keys   []string `json:"keys,omitempty"`
	Prefix string   `json:"prefix,omitempty"`
}
type detachSecretReq struct {
	Name string `json:"name"`
}

func (a *API) scaleApp(w http.ResponseWriter, r *http.Request) {
	projectID := chi.URLParam(r, "id")
	appID := chi.URLParam(r, "appId")
	var req scaleReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}
	if req.Replicas < 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "replicas must be >= 0"})
		return
	}
	if err := a.svc.ScaleApp(r.Context(), projectID, appID, req.Replicas); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "scaled"})
}

func (a *API) updateAppImage(w http.ResponseWriter, r *http.Request) {
	projectID := chi.URLParam(r, "id")
	appID := chi.URLParam(r, "appId")
	var req imageReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}
	if err := a.svc.UpdateAppImage(r.Context(), projectID, appID, req.Image); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "updated"})
}

func (a *API) rolloutRestartApp(w http.ResponseWriter, r *http.Request) {
	projectID := chi.URLParam(r, "id")
	appID := chi.URLParam(r, "appId")
	if err := a.svc.RolloutRestartApp(r.Context(), projectID, appID); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "restarted"})
}

func (a *API) attachConfigToApp(w http.ResponseWriter, r *http.Request) {
	projectID := chi.URLParam(r, "id")
	appID := chi.URLParam(r, "appId")
	var req attachConfigReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}
	if err := a.svc.AttachConfigMapToApp(r.Context(), projectID, appID, req.Name, req.Keys, req.Prefix); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "attached"})
}

func (a *API) detachConfigFromApp(w http.ResponseWriter, r *http.Request) {
	projectID := chi.URLParam(r, "id")
	appID := chi.URLParam(r, "appId")
	var req detachConfigReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}
	if err := a.svc.DetachConfigMapFromApp(r.Context(), projectID, appID, req.Name); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "detached"})
}

func (a *API) attachSecretToApp(w http.ResponseWriter, r *http.Request) {
	projectID := chi.URLParam(r, "id")
	appID := chi.URLParam(r, "appId")
	var req attachSecretReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}
	if err := a.svc.AttachSecretToApp(r.Context(), projectID, appID, req.Name, req.Keys, req.Prefix); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "attached"})
}

func (a *API) detachSecretFromApp(w http.ResponseWriter, r *http.Request) {
	projectID := chi.URLParam(r, "id")
	appID := chi.URLParam(r, "appId")
	var req detachSecretReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}
	if err := a.svc.DetachSecretFromApp(r.Context(), projectID, appID, req.Name); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "detached"})
}

// -------- Delete App --------

func (a *API) deleteApp(w http.ResponseWriter, r *http.Request) {
	projectID := chi.URLParam(r, "id")
	appID := chi.URLParam(r, "appId")
	if err := a.svc.DeleteApp(r.Context(), projectID, appID); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}
