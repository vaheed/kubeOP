package testcase

import (
    "net/http"
    "net/http/httptest"
    "testing"

    "kubeop/internal/api"
    "kubeop/internal/config"
)

func TestDeleteUserRoute_Exists(t *testing.T) {
    cfg := &config.Config{DisableAuth: true}
    r := api.NewRouter(cfg, nil)
    rr := httptest.NewRecorder()
    req := httptest.NewRequest(http.MethodDelete, "/v1/users/u-123", nil)
    r.ServeHTTP(rr, req)
    if rr.Code != http.StatusInternalServerError {
        t.Fatalf("/v1/users/{id} DELETE expected 500, got %d", rr.Code)
    }
}

func TestDeleteAppRoute_Exists(t *testing.T) {
    cfg := &config.Config{DisableAuth: true}
    r := api.NewRouter(cfg, nil)
    rr := httptest.NewRecorder()
    req := httptest.NewRequest(http.MethodDelete, "/v1/projects/p-123/apps/a-123", nil)
    r.ServeHTTP(rr, req)
    if rr.Code != http.StatusInternalServerError {
        t.Fatalf("/v1/projects/{id}/apps/{appId} DELETE expected 500, got %d", rr.Code)
    }
}

