package testcase

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"kubeop/internal/api"
	"kubeop/internal/config"
)

func TestAppsAndTemplatesRoutes_Exist(t *testing.T) {
	cfg := &config.Config{DisableAuth: true}
	r := api.NewRouter(cfg, nil)

	// POST /v1/templates
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/templates", bytes.NewBufferString(`{"name":"n","kind":"manifests","spec":{}}`))
	r.ServeHTTP(rr, req)
	if rr.Code != http.StatusInternalServerError { // service is nil
		t.Fatalf("/v1/templates expected 500, got %d", rr.Code)
	}

	// GET /v1/templates
	rr = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/v1/templates", nil)
	r.ServeHTTP(rr, req)
	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("/v1/templates GET expected 500, got %d", rr.Code)
	}

	// GET /v1/templates/{id}
	rr = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/v1/templates/t1", nil)
	r.ServeHTTP(rr, req)
	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("/v1/templates/{id} expected 500, got %d", rr.Code)
	}

	// POST /v1/templates/{id}/render
	rr = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/v1/templates/t1/render", bytes.NewBufferString(`{"values":{}}`))
	r.ServeHTTP(rr, req)
	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("/v1/templates/{id}/render expected 500, got %d", rr.Code)
	}

	// POST /v1/projects/{id}/apps
	rr = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/v1/projects/p1/apps", bytes.NewBufferString(`{"name":"app","image":"nginx"}`))
	r.ServeHTTP(rr, req)
	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("/v1/projects/{id}/apps expected 500, got %d", rr.Code)
	}

	// POST /v1/projects/{id}/templates/{templateId}/deploy
	rr = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/v1/projects/p1/templates/t1/deploy", bytes.NewBufferString(`{"values":{}}`))
	r.ServeHTTP(rr, req)
	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("/v1/projects/{id}/templates/{templateId}/deploy expected 500, got %d", rr.Code)
	}

	// POST /v1/apps/validate
	rr = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/v1/apps/validate", bytes.NewBufferString(`{"projectId":"p1","name":"app","image":"nginx"}`))
	r.ServeHTTP(rr, req)
	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("/v1/apps/validate expected 500, got %d", rr.Code)
	}

	// GET logs
	rr = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/v1/projects/p1/apps/a1/logs", nil)
	r.ServeHTTP(rr, req)
	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("/v1/projects/{id}/apps/{appId}/logs expected 500, got %d", rr.Code)
	}

	// GET releases
	rr = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/v1/projects/p1/apps/a1/releases", nil)
	r.ServeHTTP(rr, req)
	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("/v1/projects/{id}/apps/{appId}/releases expected 500, got %d", rr.Code)
	}

	// Attach config
	rr = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/v1/projects/p1/apps/a1/configs/attach", bytes.NewBufferString(`{"name":"cfg"}`))
	r.ServeHTTP(rr, req)
	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("/v1/projects/{id}/apps/{appId}/configs/attach expected 500, got %d", rr.Code)
	}

	// Detach config
	rr = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/v1/projects/p1/apps/a1/configs/detach", bytes.NewBufferString(`{"name":"cfg"}`))
	r.ServeHTTP(rr, req)
	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("/v1/projects/{id}/apps/{appId}/configs/detach expected 500, got %d", rr.Code)
	}

	// Attach secret
	rr = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/v1/projects/p1/apps/a1/secrets/attach", bytes.NewBufferString(`{"name":"cred"}`))
	r.ServeHTTP(rr, req)
	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("/v1/projects/{id}/apps/{appId}/secrets/attach expected 500, got %d", rr.Code)
	}

	// Detach secret
	rr = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/v1/projects/p1/apps/a1/secrets/detach", bytes.NewBufferString(`{"name":"cred"}`))
	r.ServeHTTP(rr, req)
	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("/v1/projects/{id}/apps/{appId}/secrets/detach expected 500, got %d", rr.Code)
	}
}

func TestClusterRoutesExist(t *testing.T) {
	cfg := &config.Config{DisableAuth: true}
	r := api.NewRouter(cfg, nil)

	// POST /v1/clusters
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/clusters", bytes.NewBufferString(`{"name":"c","kubeconfig":"cfg"}`))
	r.ServeHTTP(rr, req)
	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("/v1/clusters POST expected 500, got %d", rr.Code)
	}

	// GET /v1/clusters
	rr = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/v1/clusters", nil)
	r.ServeHTTP(rr, req)
	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("/v1/clusters GET expected 500, got %d", rr.Code)
	}

	// GET /v1/clusters/{id}
	rr = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/v1/clusters/cluster-1", nil)
	r.ServeHTTP(rr, req)
	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("/v1/clusters/{id} expected 500, got %d", rr.Code)
	}

	// PATCH /v1/clusters/{id}
	rr = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPatch, "/v1/clusters/cluster-1", bytes.NewBufferString(`{"environment":"prod"}`))
	r.ServeHTTP(rr, req)
	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("/v1/clusters/{id} PATCH expected 500, got %d", rr.Code)
	}

	// GET /v1/clusters/{id}/health
	rr = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/v1/clusters/cluster-1/health", nil)
	r.ServeHTTP(rr, req)
	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("/v1/clusters/{id}/health expected 500, got %d", rr.Code)
	}

	// GET /v1/clusters/health
	rr = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/v1/clusters/health", nil)
	r.ServeHTTP(rr, req)
	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("/v1/clusters/health expected 500, got %d", rr.Code)
	}

	// GET /v1/clusters/{id}/status
	rr = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/v1/clusters/cluster-1/status", nil)
	r.ServeHTTP(rr, req)
	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("/v1/clusters/{id}/status expected 500, got %d", rr.Code)
	}
}
