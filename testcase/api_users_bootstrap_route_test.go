package testcase

import (
    "bytes"
    "net/http"
    "net/http/httptest"
    "testing"

    "kubeop/internal/api"
    "kubeop/internal/config"
)

// This test ensures the /v1/users/bootstrap route exists and is wired.
// We don't exercise the service implementation here.
func TestUsersBootstrapRoute_Exists(t *testing.T) {
    cfg := &config.Config{DisableAuth: true}
    r := api.NewRouter(cfg, nil)

    body := bytes.NewBufferString(`{"userId":"u","clusterId":"c"}`)
    rr := httptest.NewRecorder()
    req := httptest.NewRequest(http.MethodPost, "/v1/users/bootstrap", body)
    r.ServeHTTP(rr, req)
    if rr.Code != http.StatusInternalServerError { // service is nil; recoverer should return 500
        t.Fatalf("expected 500 for nil service, got %d", rr.Code)
    }
}

