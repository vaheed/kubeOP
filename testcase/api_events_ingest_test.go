package testcase

import (
	"bytes"
	"compress/gzip"
	"context"
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
	svc, err := service.New(cfg, st, nil)
	if err != nil {
		t.Fatalf("service.New: %v", err)
	}
	svc.SetLogger(zap.NewNop())
	router := api.NewRouter(cfg, svc)
	return cfg, svc, mock, router, cleanup
}

func mintWatcherToken(t *testing.T, svc *service.Service, mock sqlmock.Sqlmock, clusterID string) service.WatcherCredentials {
	t.Helper()
	if svc == nil {
		t.Fatalf("service not initialised")
	}
	now := time.Now().UTC()
	mock.ExpectQuery("SELECT id, name, created_at FROM clusters").
		WithArgs(clusterID).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name", "created_at"}).AddRow(clusterID, clusterID, now))
	mock.ExpectQuery("INSERT INTO watchers").
		WithArgs(clusterID, sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows([]string{
			"id",
			"cluster_id",
			"refresh_token_hash",
			"refresh_token_expires_at",
			"access_token_expires_at",
			"last_seen_at",
			"last_refresh_at",
			"created_at",
			"updated_at",
			"disabled",
		}).AddRow(
			"watcher-"+clusterID,
			clusterID,
			"hash",
			now.Add(24*time.Hour),
			now.Add(time.Hour),
			nil,
			now,
			now,
			now,
			false,
		))
	creds, err := svc.RegisterWatcher(context.Background(), clusterID)
	if err != nil {
		t.Fatalf("RegisterWatcher: %v", err)
	}
	return creds
}

func expectWatcherLookup(t *testing.T, mock sqlmock.Sqlmock, watcherID, clusterID string) {
	t.Helper()
	now := time.Now().UTC()
	const watcherSelect = "SELECT id, cluster_id, refresh_token_hash, refresh_token_expires_at, access_token_expires_at, last_seen_at, last_refresh_at, created_at, updated_at, disabled FROM watchers WHERE id = \\$1"
	mock.ExpectQuery(watcherSelect).
		WithArgs(watcherID).
		WillReturnRows(sqlmock.NewRows([]string{
			"id",
			"cluster_id",
			"refresh_token_hash",
			"refresh_token_expires_at",
			"access_token_expires_at",
			"last_seen_at",
			"last_refresh_at",
			"created_at",
			"updated_at",
			"disabled",
		}).AddRow(
			watcherID,
			clusterID,
			"hash",
			now.Add(24*time.Hour),
			now.Add(time.Hour),
			now,
			now,
			now.Add(-time.Minute),
			now.Add(-time.Minute),
			false,
		))
}

func expectWatcherLookupByCluster(t *testing.T, mock sqlmock.Sqlmock, clusterID, watcherID string) {
	t.Helper()
	now := time.Now().UTC()
	const watcherByCluster = "SELECT id, cluster_id, refresh_token_hash, refresh_token_expires_at, access_token_expires_at, last_seen_at, last_refresh_at, created_at, updated_at, disabled\\s+FROM watchers\\s+WHERE cluster_id = \\$1\\s+ORDER BY updated_at DESC\\s+LIMIT 1"
	mock.ExpectQuery(watcherByCluster).
		WithArgs(clusterID).
		WillReturnRows(sqlmock.NewRows([]string{
			"id",
			"cluster_id",
			"refresh_token_hash",
			"refresh_token_expires_at",
			"access_token_expires_at",
			"last_seen_at",
			"last_refresh_at",
			"created_at",
			"updated_at",
			"disabled",
		}).AddRow(
			watcherID,
			clusterID,
			"hash",
			now.Add(24*time.Hour),
			now.Add(time.Hour),
			now,
			now,
			now.Add(-time.Minute),
			now.Add(-time.Minute),
			false,
		))
}

func TestWatcherEventsIngestAcceptsGzipBatch(t *testing.T) {
	_, svc, mock, router, cleanup := newIngestRouter(t, true)
	defer cleanup()
	if svc == nil {
		t.Fatalf("expected service to be initialised")
	}

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

	creds := mintWatcherToken(t, svc, mock, "cluster-1")
	expectWatcherLookup(t, mock, creds.WatcherID, creds.ClusterID)
	now := time.Now()
	mock.ExpectQuery(`INSERT INTO project_events`).
		WithArgs(sqlmock.AnyArg(), "proj-77", sqlmock.AnyArg(), sqlmock.AnyArg(), "K8S_DEPLOYMENT_ADDED", "INFO", "Deployment rollout", sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows([]string{"at"}).AddRow(now))
	req := httptest.NewRequest(http.MethodPost, "/v1/events/ingest", bytes.NewReader(gzBuf.Bytes()))
	req.Header.Set("Authorization", "Bearer "+creds.AccessToken)
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

func TestWatcherEventsIngestFallsBackToWatcherCluster(t *testing.T) {
	cfg, svc, mock, router, cleanup := newIngestRouter(t, true)
	defer cleanup()
	creds := mintWatcherToken(t, svc, mock, "cluster-1")
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"role":       "watcher",
		"cluster_id": creds.ClusterID,
	})
	missingCluster, err := token.SignedString([]byte(cfg.AdminJWTSecret))
	if err != nil {
		t.Fatalf("SignedString: %v", err)
	}
	req := httptest.NewRequest(http.MethodPost, "/v1/events/ingest", bytes.NewReader([]byte("[]")))
	req.Header.Set("Authorization", "Bearer "+missingCluster)

	rr := httptest.NewRecorder()
	expectWatcherLookupByCluster(t, mock, creds.ClusterID, creds.WatcherID)
	expectWatcherLookup(t, mock, creds.WatcherID, creds.ClusterID)
	router.ServeHTTP(rr, req)
	if rr.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d", rr.Code)
	}
	var resp struct {
		ClusterID string `json:"clusterId"`
	}
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.ClusterID != creds.ClusterID {
		t.Fatalf("expected clusterId %s, got %s", creds.ClusterID, resp.ClusterID)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("expectations: %v", err)
	}
}

func TestWatcherEventsIngestDisabledBridge(t *testing.T) {
	_, svc, mock, router, cleanup := newIngestRouter(t, false)
	defer cleanup()
	creds := mintWatcherToken(t, svc, mock, "cluster-1")
	expectWatcherLookup(t, mock, creds.WatcherID, creds.ClusterID)
	req := httptest.NewRequest(http.MethodPost, "/v1/events/ingest", bytes.NewReader([]byte("[]")))
	req.Header.Set("Authorization", "Bearer "+creds.AccessToken)

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
