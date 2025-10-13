package api

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"kubeop/internal/service"
)

type createProjectReq struct {
	UserID         string            `json:"userId,omitempty"`
	UserEmail      string            `json:"userEmail,omitempty"`
	UserName       string            `json:"userName,omitempty"`
	ClusterID      string            `json:"clusterId"`
	Name           string            `json:"name"`
	QuotaOverrides map[string]string `json:"quotaOverrides,omitempty"`
}

func (a *API) createProject(w http.ResponseWriter, r *http.Request) {
	svc, ok := a.serviceOrError(w, "createProject")
	if !ok {
		return
	}
	var req createProjectReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}
	out, err := svc.CreateProject(r.Context(), service.ProjectCreateInput{
		UserID:         req.UserID,
		UserEmail:      req.UserEmail,
		UserName:       req.UserName,
		ClusterID:      req.ClusterID,
		Name:           req.Name,
		QuotaOverrides: req.QuotaOverrides,
	})
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusCreated, out)
}

// listProjects returns all projects with optional pagination via query params: limit, offset.
func (a *API) listProjects(w http.ResponseWriter, r *http.Request) {
	svc, ok := a.serviceOrError(w, "listProjects")
	if !ok {
		return
	}
	// Parse pagination with simple defaults
	q := r.URL.Query()
	limit := 100
	offset := 0
	if v := q.Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			limit = n
		}
	}
	if v := q.Get("offset"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			offset = n
		}
	}
	ps, err := svc.ListProjects(r.Context(), limit, offset)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, ps)
}

// listUserProjects returns all projects for a given user id.
func (a *API) listUserProjects(w http.ResponseWriter, r *http.Request) {
	svc, ok := a.serviceOrError(w, "listUserProjects")
	if !ok {
		return
	}
	userID := chi.URLParam(r, "id")
	q := r.URL.Query()
	limit := 100
	offset := 0
	if v := q.Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			limit = n
		}
	}
	if v := q.Get("offset"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			offset = n
		}
	}
	ps, err := svc.ListUserProjects(r.Context(), userID, limit, offset)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, ps)
}

type quotaPatchReq struct {
	Overrides map[string]string `json:"overrides"`
}

func (a *API) patchProjectQuota(w http.ResponseWriter, r *http.Request) {
	svc, ok := a.serviceOrError(w, "patchProjectQuota")
	if !ok {
		return
	}
	id := chi.URLParam(r, "id")
	var req quotaPatchReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}
	if err := svc.UpdateProjectQuota(r.Context(), id, req.Overrides); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (a *API) suspendProject(w http.ResponseWriter, r *http.Request) {
	svc, ok := a.serviceOrError(w, "suspendProject")
	if !ok {
		return
	}
	id := chi.URLParam(r, "id")
	if err := svc.SetProjectSuspended(r.Context(), id, true); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "suspended"})
}

func (a *API) unsuspendProject(w http.ResponseWriter, r *http.Request) {
	svc, ok := a.serviceOrError(w, "unsuspendProject")
	if !ok {
		return
	}
	id := chi.URLParam(r, "id")
	if err := svc.SetProjectSuspended(r.Context(), id, false); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "unsuspended"})
}

func (a *API) getProject(w http.ResponseWriter, r *http.Request) {
	svc, ok := a.serviceOrError(w, "getProject")
	if !ok {
		return
	}
	id := chi.URLParam(r, "id")
	st, err := svc.GetProjectStatus(r.Context(), id)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, st)
}

func (a *API) deleteProject(w http.ResponseWriter, r *http.Request) {
	svc, ok := a.serviceOrError(w, "deleteProject")
	if !ok {
		return
	}
	id := chi.URLParam(r, "id")
	if err := svc.DeleteProject(r.Context(), id); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}
