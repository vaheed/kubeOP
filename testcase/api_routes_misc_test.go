package testcase

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"kubeop/internal/api"
	"kubeop/internal/config"
)

func TestMetricsAndWebhookAndRenewRoutes_Exist(t *testing.T) {
	cfg := &config.Config{DisableAuth: true}
	r := api.NewRouter(cfg, nil)

	// /metrics should be 200 even with nil service
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	r.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("/metrics expected 200, got %d", rr.Code)
	}

	// webhooks
	rr = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/v1/webhooks/git", nil)
	r.ServeHTTP(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("/v1/webhooks/git expected 400 for invalid json, got %d", rr.Code)
	}

	rr = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/v1/events/ingest", strings.NewReader("[]"))
	r.ServeHTTP(rr, req)
	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("/v1/events/ingest expected 500, got %d", rr.Code)
	}

	// kubeconfig renew
	rr = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/v1/projects/prj/kubeconfig/renew", nil)
	r.ServeHTTP(rr, req)
	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("/v1/projects/{id}/kubeconfig/renew expected 500, got %d", rr.Code)
	}
}
