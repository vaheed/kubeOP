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

    // POST /v1/projects/{id}/apps
    rr = httptest.NewRecorder()
    req = httptest.NewRequest(http.MethodPost, "/v1/projects/p1/apps", bytes.NewBufferString(`{"name":"app","image":"nginx"}`))
    r.ServeHTTP(rr, req)
    if rr.Code != http.StatusInternalServerError {
        t.Fatalf("/v1/projects/{id}/apps expected 500, got %d", rr.Code)
    }

    // GET logs
    rr = httptest.NewRecorder()
    req = httptest.NewRequest(http.MethodGet, "/v1/projects/p1/apps/a1/logs", nil)
    r.ServeHTTP(rr, req)
    if rr.Code != http.StatusInternalServerError {
        t.Fatalf("/v1/projects/{id}/apps/{appId}/logs expected 500, got %d", rr.Code)
    }
}

