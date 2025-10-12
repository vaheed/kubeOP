package testcase

import (
	"net/http"
	"net/http/httptest"
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
