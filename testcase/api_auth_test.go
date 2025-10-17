package testcase

import (
	"context"
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
	"kubeop/internal/store"
)

func TestAdminAuthMiddleware(t *testing.T) {
	cfg := &config.Config{AdminJWTSecret: "topsecret", DisableAuth: false}
	mw := api.AdminAuthMiddleware(cfg)

	// target handler records status 200
	target := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) })
	h := mw(target)

	// 1) Missing header -> 401
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/secure", nil)
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rr.Code)
	}

	// 2) Invalid token -> 401
	rr = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/v1/secure", nil)
	req.Header.Set("Authorization", "Bearer invalid")
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rr.Code)
	}

	// 3) Wrong role -> 403
	badTok := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{"role": "user"})
	badStr, _ := badTok.SignedString([]byte(cfg.AdminJWTSecret))
	rr = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/v1/secure", nil)
	req.Header.Set("Authorization", "Bearer "+badStr)
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", rr.Code)
	}

	// 4) Valid token -> 200
	goodTok := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{"role": "admin"})
	goodStr, _ := goodTok.SignedString([]byte(cfg.AdminJWTSecret))
	rr = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/v1/secure", nil)
	req.Header.Set("Authorization", "Bearer "+goodStr)
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
}

func TestWatcherHandshakeReturnsClusterID(t *testing.T) {
	cfg := &config.Config{AdminJWTSecret: "secret", KcfgEncryptionKey: strings.Repeat("k", 32)}
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	st := store.NewWithDB(db)
	svc, err := service.New(cfg, st, nil)
	if err != nil {
		t.Fatalf("service.New: %v", err)
	}
	svc.SetLogger(zap.NewNop())
	now := time.Now()
	mock.ExpectQuery("SELECT id, name, created_at FROM clusters").
		WithArgs("cluster-1").
		WillReturnRows(sqlmock.NewRows([]string{"id", "name", "created_at"}).AddRow("cluster-1", "cluster", now))
	mock.ExpectQuery("INSERT INTO watchers").
		WithArgs("cluster-1", sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows([]string{"id", "cluster_id", "refresh_token_hash", "refresh_token_expires_at", "access_token_expires_at", "last_seen_at", "last_refresh_at", "created_at", "updated_at", "disabled"}).AddRow("watcher-1", "cluster-1", "hash", now.Add(24*time.Hour), now.Add(time.Hour), now, now, now, now, false))
	creds, err := svc.RegisterWatcher(context.Background(), "cluster-1")
	if err != nil {
		t.Fatalf("RegisterWatcher: %v", err)
	}
	expectWatcherLookup(t, mock, creds.WatcherID, creds.ClusterID)
	mock.ExpectExec("UPDATE watchers SET last_seen_at").
		WithArgs(creds.WatcherID, sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(0, 1))
	router := api.NewRouter(cfg, svc)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/watchers/handshake", nil)
	req.Header.Set("Authorization", "Bearer "+creds.AccessToken)

	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected handshake 200, got %d", rr.Code)
	}
	expected := "\"cluster-1\""
	if !strings.Contains(rr.Body.String(), expected) {
		t.Fatalf("expected response to contain cluster id %s, got %s", expected, rr.Body.String())
	}
	// Body mismatch must be rejected.
	rr = httptest.NewRecorder()
	expectWatcherLookup(t, mock, creds.WatcherID, creds.ClusterID)
	req = httptest.NewRequest(http.MethodPost, "/v1/watchers/handshake", strings.NewReader(`{"cluster_id":"cluster-2"}`))
	req.Header.Set("Authorization", "Bearer "+creds.AccessToken)
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for mismatch, got %d", rr.Code)
	}

	// Cluster-only tokens should be resolved via stored watcher metadata.
	clusterOnly := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"role":       "watcher",
		"cluster_id": creds.ClusterID,
	})
	clusterOnlyStr, _ := clusterOnly.SignedString([]byte(cfg.AdminJWTSecret))
	rr = httptest.NewRecorder()
	expectWatcherLookupByCluster(t, mock, creds.ClusterID, creds.WatcherID)
	expectWatcherLookup(t, mock, creds.WatcherID, creds.ClusterID)
	mock.ExpectExec("UPDATE watchers SET last_seen_at").
		WithArgs(creds.WatcherID, sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(0, 1))
	req = httptest.NewRequest(http.MethodPost, "/v1/watchers/handshake", nil)
	req.Header.Set("Authorization", "Bearer "+clusterOnlyStr)
	router.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 for cluster token, got %d", rr.Code)
	}
	if !strings.Contains(rr.Body.String(), expected) {
		t.Fatalf("expected response to contain cluster id %s, got %s", expected, rr.Body.String())
	}

	// Legacy tokens missing cluster_id should still succeed using persisted metadata.
	legacyToken := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"role":       "watcher",
		"watcher_id": "watcher-1",
		"sub":        "watcher:watcher-1",
	})
	legacyStr, _ := legacyToken.SignedString([]byte(cfg.AdminJWTSecret))
	rr = httptest.NewRecorder()
	expectWatcherLookup(t, mock, creds.WatcherID, creds.ClusterID)
	req = httptest.NewRequest(http.MethodPost, "/v1/watchers/handshake", nil)
	req.Header.Set("Authorization", "Bearer "+legacyStr)
	router.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 for legacy token, got %d", rr.Code)
	}
	if !strings.Contains(rr.Body.String(), expected) {
		t.Fatalf("expected response to contain cluster id %s, got %s", expected, rr.Body.String())
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("sql expectations: %v", err)
	}
}
