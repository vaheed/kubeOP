package api

import (
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"kubeop/internal/service"
)

// -------- Apps (Deploy) --------

type appPort struct {
	ContainerPort int32  `json:"containerPort"`
	ServicePort   int32  `json:"servicePort"`
	Protocol      string `json:"protocol,omitempty"`    // TCP/UDP
	ServiceType   string `json:"serviceType,omitempty"` // ClusterIP|LoadBalancer
}

type deployAppReq struct {
	ProjectID string            `json:"projectId"`
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
	Git           *appGitSpec    `json:"git,omitempty"`
}

type appGitSpec struct {
	URL             string `json:"url"`
	Ref             string `json:"ref,omitempty"`
	Path            string `json:"path,omitempty"`
	Mode            string `json:"mode,omitempty"`
	CredentialID    string `json:"credentialId,omitempty"`
	InsecureSkipTLS bool   `json:"insecureSkipTLS,omitempty"`
}

func toServiceGit(spec *appGitSpec) *service.AppGitSpec {
	if spec == nil {
		return nil
	}
	return &service.AppGitSpec{
		URL:             strings.TrimSpace(spec.URL),
		Ref:             strings.TrimSpace(spec.Ref),
		Path:            strings.TrimSpace(spec.Path),
		Mode:            strings.TrimSpace(spec.Mode),
		CredentialID:    strings.TrimSpace(spec.CredentialID),
		InsecureSkipTLS: spec.InsecureSkipTLS,
	}
}

func (a *API) deployApp(w http.ResponseWriter, r *http.Request) {
	svc, ok := a.serviceOrError(w, "deployApp")
	if !ok {
		return
	}
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
	ctx := contextWithActor(r)
	out, err := svc.DeployApp(ctx, service.AppDeployInput{
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
		Git:           toServiceGit(req.Git),
	})
	if err != nil {
		if writeMaintenanceError(w, err) {
			return
		}
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusCreated, out)
}

func (a *API) validateApp(w http.ResponseWriter, r *http.Request) {
	svc, ok := a.serviceOrError(w, "validateApp")
	if !ok {
		return
	}
	var req deployAppReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}
	if strings.TrimSpace(req.ProjectID) == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "projectId is required"})
		return
	}
	var ports []service.AppPort
	for _, p := range req.Ports {
		ports = append(ports, service.AppPort{ContainerPort: p.ContainerPort, ServicePort: p.ServicePort, Protocol: p.Protocol, ServiceType: p.ServiceType})
	}
	ctx := contextWithActor(r)
	out, err := svc.ValidateApp(ctx, service.AppDeployInput{
		ProjectID:     req.ProjectID,
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
		Git:           toServiceGit(req.Git),
	})
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, out)
}

// List apps for a project (with summary status)
func (a *API) listProjectApps(w http.ResponseWriter, r *http.Request) {
	svc, ok := a.serviceOrError(w, "listProjectApps")
	if !ok {
		return
	}
	projectID := chi.URLParam(r, "id")
	sts, err := svc.ListProjectAppsStatus(r.Context(), projectID)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, sts)
}

// Get a single app with detailed status
func (a *API) getProjectApp(w http.ResponseWriter, r *http.Request) {
	svc, ok := a.serviceOrError(w, "getProjectApp")
	if !ok {
		return
	}
	projectID := chi.URLParam(r, "id")
	appID := chi.URLParam(r, "appId")
	st, err := svc.GetAppStatus(r.Context(), projectID, appID)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, st)
}

// -------- Logs --------

func (a *API) appLogs(w http.ResponseWriter, r *http.Request) {
	svc, ok := a.serviceOrError(w, "appLogs")
	if !ok {
		return
	}
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
	rc, closer, err := svc.StreamAppLogs(r.Context(), service.AppLogsInput{
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
	svc, ok := a.serviceOrError(w, "renewProjectKubeconfig")
	if !ok {
		return
	}
	projectID := chi.URLParam(r, "id")
	ctx := contextWithActor(r)
	out, err := svc.RenewProjectKubeconfig(ctx, projectID)
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
	svc, ok := a.serviceOrError(w, "scaleApp")
	if !ok {
		return
	}
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
	ctx := contextWithActor(r)
	if err := svc.ScaleApp(ctx, projectID, appID, req.Replicas); err != nil {
		if writeMaintenanceError(w, err) {
			return
		}
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "scaled"})
}

func (a *API) updateAppImage(w http.ResponseWriter, r *http.Request) {
	svc, ok := a.serviceOrError(w, "updateAppImage")
	if !ok {
		return
	}
	projectID := chi.URLParam(r, "id")
	appID := chi.URLParam(r, "appId")
	var req imageReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}
	ctx := contextWithActor(r)
	if err := svc.UpdateAppImage(ctx, projectID, appID, req.Image); err != nil {
		if writeMaintenanceError(w, err) {
			return
		}
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "updated"})
}

func (a *API) rolloutRestartApp(w http.ResponseWriter, r *http.Request) {
	svc, ok := a.serviceOrError(w, "rolloutRestartApp")
	if !ok {
		return
	}
	projectID := chi.URLParam(r, "id")
	appID := chi.URLParam(r, "appId")
	ctx := contextWithActor(r)
	if err := svc.RolloutRestartApp(ctx, projectID, appID); err != nil {
		if writeMaintenanceError(w, err) {
			return
		}
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "restarted"})
}

func (a *API) attachConfigToApp(w http.ResponseWriter, r *http.Request) {
	svc, ok := a.serviceOrError(w, "attachConfigToApp")
	if !ok {
		return
	}
	projectID := chi.URLParam(r, "id")
	appID := chi.URLParam(r, "appId")
	var req attachConfigReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}
	ctx := contextWithActor(r)
	if err := svc.AttachConfigMapToApp(ctx, projectID, appID, req.Name, req.Keys, req.Prefix); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "attached"})
}

func (a *API) detachConfigFromApp(w http.ResponseWriter, r *http.Request) {
	svc, ok := a.serviceOrError(w, "detachConfigFromApp")
	if !ok {
		return
	}
	projectID := chi.URLParam(r, "id")
	appID := chi.URLParam(r, "appId")
	var req detachConfigReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}
	ctx := contextWithActor(r)
	if err := svc.DetachConfigMapFromApp(ctx, projectID, appID, req.Name); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "detached"})
}

func (a *API) attachSecretToApp(w http.ResponseWriter, r *http.Request) {
	svc, ok := a.serviceOrError(w, "attachSecretToApp")
	if !ok {
		return
	}
	projectID := chi.URLParam(r, "id")
	appID := chi.URLParam(r, "appId")
	var req attachSecretReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}
	ctx := contextWithActor(r)
	if err := svc.AttachSecretToApp(ctx, projectID, appID, req.Name, req.Keys, req.Prefix); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "attached"})
}

func (a *API) detachSecretFromApp(w http.ResponseWriter, r *http.Request) {
	svc, ok := a.serviceOrError(w, "detachSecretFromApp")
	if !ok {
		return
	}
	projectID := chi.URLParam(r, "id")
	appID := chi.URLParam(r, "appId")
	var req detachSecretReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}
	ctx := contextWithActor(r)
	if err := svc.DetachSecretFromApp(ctx, projectID, appID, req.Name); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "detached"})
}

// -------- Delete App --------

func (a *API) deleteApp(w http.ResponseWriter, r *http.Request) {
	svc, ok := a.serviceOrError(w, "deleteApp")
	if !ok {
		return
	}
	projectID := chi.URLParam(r, "id")
	appID := chi.URLParam(r, "appId")
	ctx := contextWithActor(r)
	if err := svc.DeleteApp(ctx, projectID, appID); err != nil {
		if writeMaintenanceError(w, err) {
			return
		}
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}
