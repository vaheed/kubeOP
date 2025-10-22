package testcase

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"kubeop/internal/api"
	"kubeop/internal/config"
	"kubeop/internal/service"
	"kubeop/internal/store"
)

func newRouterWithService(t *testing.T) http.Handler {
	t.Helper()
	db, _, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	cfg := &config.Config{DisableAuth: true, KcfgEncryptionKey: "test"}
	svc, err := service.New(cfg, store.NewWithDB(db), nil)
	if err != nil {
		t.Fatalf("service.New: %v", err)
	}
	return api.NewRouter(cfg, svc)
}

func TestScaleAppRequiresIfMatch(t *testing.T) {
	router := newRouterWithService(t)
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPatch, "/v1/projects/p1/apps/a1/scale", bytes.NewBufferString(`{"replicas":1}`))
	router.ServeHTTP(rr, req)
	if rr.Code != http.StatusPreconditionRequired {
		t.Fatalf("expected 428, got %d", rr.Code)
	}
}

func TestUpdateAppImageRequiresIfMatch(t *testing.T) {
	router := newRouterWithService(t)
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPatch, "/v1/projects/p1/apps/a1/image", bytes.NewBufferString(`{"image":"nginx"}`))
	router.ServeHTTP(rr, req)
	if rr.Code != http.StatusPreconditionRequired {
		t.Fatalf("expected 428, got %d", rr.Code)
	}
}
