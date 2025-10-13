package api

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"kubeop/internal/service"
)

type kubeconfigEnsureRequest struct {
	UserID    string `json:"userId"`
	ProjectID string `json:"projectId,omitempty"`
	ClusterID string `json:"clusterId,omitempty"`
}

type kubeconfigRotateRequest struct {
	ID string `json:"id"`
}

func (a *API) ensureKubeconfig(w http.ResponseWriter, r *http.Request) {
	if a.svc == nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "service unavailable"})
		return
	}
	var req kubeconfigEnsureRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}
	out, err := a.svc.EnsureKubeconfigBinding(r.Context(), service.KubeconfigEnsureInput{
		UserID:    strings.TrimSpace(req.UserID),
		ProjectID: strings.TrimSpace(req.ProjectID),
		ClusterID: strings.TrimSpace(req.ClusterID),
	})
	if err != nil {
		status := http.StatusBadRequest
		if errors.Is(err, sql.ErrNoRows) {
			status = http.StatusNotFound
		}
		writeJSON(w, status, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (a *API) rotateKubeconfig(w http.ResponseWriter, r *http.Request) {
	if a.svc == nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "service unavailable"})
		return
	}
	var req kubeconfigRotateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}
	req.ID = strings.TrimSpace(req.ID)
	if req.ID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "id is required"})
		return
	}
	out, err := a.svc.RotateKubeconfigByID(r.Context(), req.ID)
	if err != nil {
		status := http.StatusBadRequest
		if errors.Is(err, sql.ErrNoRows) {
			status = http.StatusNotFound
		}
		writeJSON(w, status, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (a *API) deleteKubeconfig(w http.ResponseWriter, r *http.Request) {
	if a.svc == nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "service unavailable"})
		return
	}
	id := strings.TrimSpace(chi.URLParam(r, "id"))
	if id == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "id is required"})
		return
	}
	if err := a.svc.DeleteKubeconfigBinding(r.Context(), id); err != nil {
		status := http.StatusBadRequest
		if errors.Is(err, sql.ErrNoRows) {
			status = http.StatusNotFound
		}
		writeJSON(w, status, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}
