package testcase

import (
    "encoding/json"
    "net/http"
    "net/http/httptest"
    "testing"

    "kubeop/internal/api"
    "kubeop/internal/config"
)

func TestRouter_HealthAndVersion(t *testing.T) {
    cfg := &config.Config{DisableAuth: true}
    r := api.NewRouter(cfg, nil)

    // /healthz
    rr := httptest.NewRecorder()
    req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
    r.ServeHTTP(rr, req)
    if rr.Code != http.StatusOK {
        t.Fatalf("/healthz: expected 200, got %d", rr.Code)
    }
    var hv map[string]any
    if err := json.Unmarshal(rr.Body.Bytes(), &hv); err != nil {
        t.Fatalf("/healthz: invalid json: %v", err)
    }
    if hv["status"] != "ok" {
        t.Fatalf("/healthz: expected status=ok, got %v", hv["status"])
    }

    // /v1/version
    rr = httptest.NewRecorder()
    req = httptest.NewRequest(http.MethodGet, "/v1/version", nil)
    r.ServeHTTP(rr, req)
    if rr.Code != http.StatusOK {
        t.Fatalf("/v1/version: expected 200, got %d", rr.Code)
    }
    var ver map[string]any
    if err := json.Unmarshal(rr.Body.Bytes(), &ver); err != nil {
        t.Fatalf("/v1/version: invalid json: %v", err)
    }
    // keys exist and are strings
    for _, k := range []string{"version", "commit", "date"} {
        if _, ok := ver[k].(string); !ok {
            t.Fatalf("/v1/version: expected key %q to be string; got %T", k, ver[k])
        }
    }
}

