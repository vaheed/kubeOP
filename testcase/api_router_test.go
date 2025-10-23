package testcase

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/prometheus/client_golang/prometheus/testutil"
	"kubeop/internal/api"
	"kubeop/internal/config"
	"kubeop/internal/metrics"
	"kubeop/internal/service"
	"kubeop/internal/version"
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
	meta := version.Metadata()
	// keys exist and are strings
	for _, k := range []string{"version", "commit", "date"} {
		v, ok := ver[k].(string)
		if !ok {
			t.Fatalf("/v1/version: expected key %q to be string; got %T", k, ver[k])
		}
		if k == "version" && v != meta.Build.Version {
			t.Fatalf("/v1/version: version mismatch: got %q want %q", v, meta.Build.Version)
		}
	}
	if _, ok := ver["compatibility"]; ok {
		t.Fatalf("/v1/version: did not expect compatibility metadata in response")
	}
}

func TestRouter_ReadyzWithoutService(t *testing.T) {
	cfg := &config.Config{DisableAuth: true}
	r := api.NewRouter(cfg, nil)

	metrics.ResetReadyzFailures()
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusServiceUnavailable {
		t.Fatalf("/readyz: expected 503, got %d", rr.Code)
	}
	var body map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatalf("/readyz: invalid json: %v", err)
	}
	if body["error"] != "service unavailable" {
		t.Fatalf("/readyz: expected service unavailable error, got %v", body["error"])
	}

	if got := testutil.ToFloat64(metrics.ReadyzFailures.WithLabelValues("service_missing")); got != 1 {
		t.Fatalf("/readyz: expected service_missing metric to be 1, got %v", got)
	}
}

type failingHealth struct{}

func (f failingHealth) Health(context.Context) error { return errors.New("db offline") }

func TestRouter_ReadyzHealthFailure(t *testing.T) {
	metrics.ResetReadyzFailures()
	cfg := &config.Config{DisableAuth: true}
	svc := &service.Service{}
	r := api.NewRouter(cfg, svc, api.WithHealthChecker(failingHealth{}))

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusServiceUnavailable {
		t.Fatalf("/readyz: expected 503, got %d", rr.Code)
	}
	var body map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatalf("/readyz: invalid json: %v", err)
	}
	if body["error"] != "db offline" {
		t.Fatalf("/readyz: expected db offline error, got %v", body["error"])
	}
	if got := testutil.ToFloat64(metrics.ReadyzFailures.WithLabelValues("health_check_failed")); got != 1 {
		t.Fatalf("/readyz: expected health_check_failed metric to be 1, got %v", got)
	}
}
