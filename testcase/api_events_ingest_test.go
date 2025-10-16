package testcase

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/golang-jwt/jwt/v5"
	"go.uber.org/zap"
	"kubeop/internal/api"
	"kubeop/internal/config"
	"kubeop/internal/service"
	"kubeop/internal/sink"
	"kubeop/internal/store"
)

func newIngestRouter(t *testing.T, bridgeEnabled bool) (*config.Config, *service.Service, sqlmock.Sqlmock, http.Handler, func()) {
	t.Helper()
	cfg := &config.Config{
		AdminJWTSecret:    "secret",
		KcfgEncryptionKey: strings.Repeat("b", 32),
		EventsDBEnabled:   true,
		K8SEventsBridge:   bridgeEnabled,
	}
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	cleanup := func() { db.Close() }
	st := store.NewWithDB(db)
	var svc *service.Service
	if bridgeEnabled {
		svc, err = service.New(cfg, st, nil)
		if err != nil {
			t.Fatalf("service.New: %v", err)
		}
		svc.SetLogger(zap.NewNop())
	} else {
		svc = nil
	}
	router := api.NewRouter(cfg, svc)
	return cfg, svc, mock, router, cleanup
}

func signWatcherToken(t *testing.T, cfg *config.Config, clusterID string) string {
	t.Helper()
	claims := jwt.MapClaims{"role": "admin"}
	if clusterID != "" {
		claims["cluster_id"] = clusterID
		claims["sub"] = "watcher:" + clusterID
	}
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	str, err := tok.SignedString([]byte(cfg.AdminJWTSecret))
	if err != nil {
		t.Fatalf("sign token: %v", err)
	}
	return str
}

func TestWatcherEventsIngestAcceptsGzipBatch(t *testing.T) {
	cfg, svc, mock, router, cleanup := newIngestRouter(t, true)
	defer cleanup()
	if svc == nil {
		t.Fatalf("expected service to be initialised")
	}
	now := time.Now()
	mock.ExpectQuery(`INSERT INTO project_events`).
		WithArgs(sqlmock.AnyArg(), "proj-77", sqlmock.AnyArg(), sqlmock.AnyArg(), "K8S_DEPLOYMENT_ADDED", "INFO", "Deployment rollout", sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows([]string{"at"}).AddRow(now))

	events := []sink.Event{{
		ClusterID: "cluster-1",
		EventType: "Added",
		Kind:      "Deployment",
		Namespace: "ns-1",
		Name:      "api",
		Labels: map[string]string{
			"kubeop.project-id": "proj-77",
		},
		Summary:  "Deployment rollout",
		DedupKey: "uid#1",
	}}
	raw, err := json.Marshal(events)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var gzBuf bytes.Buffer
	zw := gzip.NewWriter(&gzBuf)
	if _, err := zw.Write(raw); err != nil {
		t.Fatalf("gzip write: %v", err)
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("gzip close: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/v1/events/ingest", bytes.NewReader(gzBuf.Bytes()))
	req.Header.Set("Authorization", "Bearer "+signWatcherToken(t, cfg, "cluster-1"))
	req.Header.Set("Content-Encoding", "gzip")

	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)
	if rr.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d", rr.Code)
	}
	if !strings.Contains(rr.Body.String(), "\"accepted\":1") {
		t.Fatalf("expected accepted=1 in response, got %s", rr.Body.String())
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("expectations: %v", err)
	}
}

func TestWatcherEventsIngestRequiresClusterID(t *testing.T) {
	cfg, _, mock, router, cleanup := newIngestRouter(t, true)
	defer cleanup()
	req := httptest.NewRequest(http.MethodPost, "/v1/events/ingest", bytes.NewReader([]byte("[]")))
	req.Header.Set("Authorization", "Bearer "+signWatcherToken(t, cfg, ""))

	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rr.Code)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("expectations: %v", err)
	}
}

func TestWatcherEventsIngestDisabledBridge(t *testing.T) {
	cfg, _, mock, router, cleanup := newIngestRouter(t, false)
	defer cleanup()
	req := httptest.NewRequest(http.MethodPost, "/v1/events/ingest", bytes.NewReader([]byte("[]")))
	req.Header.Set("Authorization", "Bearer "+signWatcherToken(t, cfg, "cluster-1"))

	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)
	if rr.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d", rr.Code)
	}
	if !strings.Contains(rr.Body.String(), "\"status\":\"ignored\"") {
		t.Fatalf("expected ignored status, got %s", rr.Body.String())
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("expectations: %v", err)
	}
}
