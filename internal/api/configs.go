package api

import (
	"encoding/json"
	"github.com/go-chi/chi/v5"
	"net/http"
)

// ConfigMaps
type configCreateReq struct {
	Name string            `json:"name"`
	Data map[string]string `json:"data"`
}

func (a *API) createConfig(w http.ResponseWriter, r *http.Request) {
	svc, ok := a.serviceOrError(w, "createConfig")
	if !ok {
		return
	}
	projectID := chi.URLParam(r, "id")
	var req configCreateReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}
	if req.Name == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "name is required"})
		return
	}
	if err := svc.CreateConfigMap(r.Context(), projectID, req.Name, req.Data); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusCreated, map[string]string{"name": req.Name})
}

func (a *API) listConfigs(w http.ResponseWriter, r *http.Request) {
	svc, ok := a.serviceOrError(w, "listConfigs")
	if !ok {
		return
	}
	projectID := chi.URLParam(r, "id")
	out, err := svc.ListConfigMaps(r.Context(), projectID)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (a *API) deleteConfig(w http.ResponseWriter, r *http.Request) {
	svc, ok := a.serviceOrError(w, "deleteConfig")
	if !ok {
		return
	}
	projectID := chi.URLParam(r, "id")
	name := chi.URLParam(r, "name")
	if err := svc.DeleteConfigMap(r.Context(), projectID, name); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

// Secrets
type secretCreateReq struct {
	Name       string            `json:"name"`
	Type       string            `json:"type,omitempty"`
	StringData map[string]string `json:"stringData"`
}

func (a *API) createSecret(w http.ResponseWriter, r *http.Request) {
	svc, ok := a.serviceOrError(w, "createSecret")
	if !ok {
		return
	}
	projectID := chi.URLParam(r, "id")
	var req secretCreateReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}
	if req.Name == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "name is required"})
		return
	}
	if err := svc.CreateSecret(r.Context(), projectID, req.Name, req.Type, req.StringData); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusCreated, map[string]string{"name": req.Name})
}

func (a *API) listSecrets(w http.ResponseWriter, r *http.Request) {
	svc, ok := a.serviceOrError(w, "listSecrets")
	if !ok {
		return
	}
	projectID := chi.URLParam(r, "id")
	out, err := svc.ListSecrets(r.Context(), projectID)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (a *API) deleteSecret(w http.ResponseWriter, r *http.Request) {
	svc, ok := a.serviceOrError(w, "deleteSecret")
	if !ok {
		return
	}
	projectID := chi.URLParam(r, "id")
	name := chi.URLParam(r, "name")
	if err := svc.DeleteSecret(r.Context(), projectID, name); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}
