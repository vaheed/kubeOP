package api

import (
	"bufio"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"kubeop/internal/logging"
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
	ctx := contextWithActor(r)
	out, err := svc.CreateProject(ctx, service.ProjectCreateInput{
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

func (a *API) projectLogs(w http.ResponseWriter, r *http.Request) {
	if _, ok := a.serviceOrError(w, "projectLogs"); !ok {
		return
	}
	projectID := chi.URLParam(r, "id")
	path, err := logging.ProjectLogPath(projectID)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	tailParam := strings.TrimSpace(r.URL.Query().Get("tail"))
	if tailParam != "" {
		tailLines, err := strconv.Atoi(tailParam)
		if err != nil || tailLines < 0 {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid tail value"})
			return
		}
		data, err := tailProjectLog(path, tailLines)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				writeJSON(w, http.StatusNotFound, map[string]string{"error": "project log not found"})
				return
			}
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		if len(data) > 0 {
			_, _ = w.Write(data)
		}
		return
	}
	file, err := os.Open(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "project log not found"})
			return
		}
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	http.ServeContent(w, r, filepath.Base(path), info.ModTime(), file)
}

func tailProjectLog(path string, lines int) ([]byte, error) {
	if lines <= 0 {
		return []byte{}, nil
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	reader := bufio.NewReader(f)
	ring := make([][]byte, lines)
	count := 0
	index := 0
	for {
		chunk, err := reader.ReadBytes('\n')
		if len(chunk) > 0 {
			copyChunk := append([]byte(nil), chunk...)
			ring[index] = copyChunk
			index = (index + 1) % lines
			if count < lines {
				count++
			}
		}
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, err
		}
	}
	if count == 0 {
		return []byte{}, nil
	}
	start := (index - count + lines) % lines
	buf := make([]byte, 0)
	for i := 0; i < count; i++ {
		pos := (start + i) % lines
		buf = append(buf, ring[pos]...)
	}
	return buf, nil
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
	ctx := contextWithActor(r)
	if err := svc.UpdateProjectQuota(ctx, id, req.Overrides); err != nil {
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
	ctx := contextWithActor(r)
	if err := svc.SetProjectSuspended(ctx, id, true); err != nil {
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
	ctx := contextWithActor(r)
	if err := svc.SetProjectSuspended(ctx, id, false); err != nil {
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
	ctx := contextWithActor(r)
	if err := svc.DeleteProject(ctx, id); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}
