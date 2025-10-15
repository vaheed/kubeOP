package testcase

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"kubeop/internal/api"
	"kubeop/internal/config"
)

// Ensure GET /v1/projects route exists and is wired.
func TestProjectsListRoute_Exists(t *testing.T) {
	cfg := &config.Config{DisableAuth: true}
	r := api.NewRouter(cfg, nil)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/projects", nil)
	r.ServeHTTP(rr, req)
	if rr.Code != http.StatusInternalServerError { // nil service panics, recoverer returns 500
		t.Fatalf("/v1/projects expected 500, got %d", rr.Code)
	}
}

// Ensure GET /v1/users/{id}/projects route exists and is wired.
func TestUserProjectsListRoute_Exists(t *testing.T) {
	cfg := &config.Config{DisableAuth: true}
	r := api.NewRouter(cfg, nil)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/users/abc/projects", nil)
	r.ServeHTTP(rr, req)
	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("/v1/users/{id}/projects expected 500, got %d", rr.Code)
	}
}

func TestProjectLogsRoute_Exists(t *testing.T) {
	cfg := &config.Config{DisableAuth: true}
	r := api.NewRouter(cfg, nil)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/projects/proj-1/logs", nil)
	r.ServeHTTP(rr, req)
	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("/v1/projects/{id}/logs expected 500, got %d", rr.Code)
	}
}

func TestProjectQuotaRoute_Exists(t *testing.T) {
	cfg := &config.Config{DisableAuth: true}
	r := api.NewRouter(cfg, nil)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/projects/proj-1/quota", nil)
	r.ServeHTTP(rr, req)
	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("GET /v1/projects/{id}/quota expected 500, got %d", rr.Code)
	}
}

func TestProjectEventsRoutes_Exist(t *testing.T) {
	cfg := &config.Config{DisableAuth: true}
	r := api.NewRouter(cfg, nil)

	getRecorder := httptest.NewRecorder()
	getReq := httptest.NewRequest(http.MethodGet, "/v1/projects/proj-1/events", nil)
	r.ServeHTTP(getRecorder, getReq)
	if getRecorder.Code != http.StatusInternalServerError {
		t.Fatalf("GET /v1/projects/{id}/events expected 500, got %d", getRecorder.Code)
	}

	postRecorder := httptest.NewRecorder()
	postReq := httptest.NewRequest(http.MethodPost, "/v1/projects/proj-1/events", strings.NewReader(`{}`))
	r.ServeHTTP(postRecorder, postReq)
	if postRecorder.Code != http.StatusInternalServerError {
		t.Fatalf("POST /v1/projects/{id}/events expected 500, got %d", postRecorder.Code)
	}
}
