package api

import (
    "encoding/json"
    "net/http"

    "github.com/go-chi/chi/v5"
    "kubeop/internal/service"
)

type createProjectReq struct {
    UserID    string            `json:"userId,omitempty"`
    UserEmail string            `json:"userEmail,omitempty"`
    UserName  string            `json:"userName,omitempty"`
    ClusterID string            `json:"clusterId"`
    Name      string            `json:"name"`
    QuotaOverrides map[string]string `json:"quotaOverrides,omitempty"`
}

func (a *API) createProject(w http.ResponseWriter, r *http.Request) {
    var req createProjectReq
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
        return
    }
    out, err := a.svc.CreateProject(r.Context(), service.ProjectCreateInput{
        UserID: req.UserID,
        UserEmail: req.UserEmail,
        UserName: req.UserName,
        ClusterID: req.ClusterID,
        Name: req.Name,
        QuotaOverrides: req.QuotaOverrides,
    })
    if err != nil {
        writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
        return
    }
    writeJSON(w, http.StatusCreated, out)
}

type quotaPatchReq struct {
    Overrides map[string]string `json:"overrides"`
}

func (a *API) patchProjectQuota(w http.ResponseWriter, r *http.Request) {
    id := chi.URLParam(r, "id")
    var req quotaPatchReq
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
        return
    }
    if err := a.svc.UpdateProjectQuota(r.Context(), id, req.Overrides); err != nil {
        writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
        return
    }
    writeJSON(w, http.StatusOK, map[string]string{"status":"ok"})
}

func (a *API) suspendProject(w http.ResponseWriter, r *http.Request) {
    id := chi.URLParam(r, "id")
    if err := a.svc.SetProjectSuspended(r.Context(), id, true); err != nil {
        writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
        return
    }
    writeJSON(w, http.StatusOK, map[string]string{"status":"suspended"})
}

func (a *API) unsuspendProject(w http.ResponseWriter, r *http.Request) {
    id := chi.URLParam(r, "id")
    if err := a.svc.SetProjectSuspended(r.Context(), id, false); err != nil {
        writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
        return
    }
    writeJSON(w, http.StatusOK, map[string]string{"status":"unsuspended"})
}

func (a *API) getProject(w http.ResponseWriter, r *http.Request) {
    id := chi.URLParam(r, "id")
    st, err := a.svc.GetProjectStatus(r.Context(), id)
    if err != nil {
        writeJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
        return
    }
    writeJSON(w, http.StatusOK, st)
}

func (a *API) deleteProject(w http.ResponseWriter, r *http.Request) {
    id := chi.URLParam(r, "id")
    if err := a.svc.DeleteProject(r.Context(), id); err != nil {
        writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
        return
    }
    writeJSON(w, http.StatusOK, map[string]string{"status":"deleted"})
}
