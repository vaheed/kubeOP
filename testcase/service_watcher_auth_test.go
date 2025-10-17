package testcase

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"testing"
	"time"

	sqlmock "github.com/DATA-DOG/go-sqlmock"

	"kubeop/internal/config"
	"kubeop/internal/service"
	"kubeop/internal/store"
)

func TestRegisterWatcher(t *testing.T) {
	t.Parallel()

	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	st := store.NewWithDB(db)
	cfg := &config.Config{AdminJWTSecret: "secret", KcfgEncryptionKey: "key"}
	svc, err := service.New(cfg, st, nil)
	if err != nil {
		t.Fatalf("service.New: %v", err)
	}

	now := time.Now()
	mock.ExpectQuery("SELECT id, name, created_at FROM clusters").WithArgs("cluster-1").WillReturnRows(sqlmock.NewRows([]string{"id", "name", "created_at"}).AddRow("cluster-1", "cluster", now))
	mock.ExpectQuery("INSERT INTO watchers").WithArgs("cluster-1", sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg()).WillReturnRows(sqlmock.NewRows([]string{"id", "cluster_id", "refresh_token_hash", "refresh_token_expires_at", "access_token_expires_at", "last_seen_at", "last_refresh_at", "created_at", "updated_at", "disabled"}).AddRow("watcher-1", "cluster-1", "hash", now.Add(24*time.Hour), now.Add(time.Hour), now, now, now, now, false))

	creds, err := svc.RegisterWatcher(context.Background(), "cluster-1")
	if err != nil {
		t.Fatalf("RegisterWatcher: %v", err)
	}
	if creds.WatcherID == "" || creds.AccessToken == "" || creds.RefreshToken == "" {
		t.Fatalf("expected credentials to be populated: %+v", creds)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("expectations: %v", err)
	}
}

func TestRefreshWatcherTokens(t *testing.T) {
	t.Parallel()

	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	st := store.NewWithDB(db)
	cfg := &config.Config{AdminJWTSecret: "secret", KcfgEncryptionKey: "key"}
	svc, err := service.New(cfg, st, nil)
	if err != nil {
		t.Fatalf("service.New: %v", err)
	}

	refreshToken := "initial-refresh"
	sum := sha256.Sum256([]byte(refreshToken))
	hashed := hex.EncodeToString(sum[:])
	now := time.Now()

	mock.ExpectQuery("SELECT id, cluster_id, refresh_token_hash").WithArgs("watcher-1").WillReturnRows(sqlmock.NewRows([]string{"id", "cluster_id", "refresh_token_hash", "refresh_token_expires_at", "access_token_expires_at", "last_seen_at", "last_refresh_at", "created_at", "updated_at", "disabled"}).AddRow("watcher-1", "cluster-1", hashed, now.Add(time.Hour), now.Add(10*time.Minute), now, now, now, now, false))
	mock.ExpectQuery("UPDATE watchers SET refresh_token_hash").WithArgs("watcher-1", sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg()).WillReturnRows(sqlmock.NewRows([]string{"id", "cluster_id", "refresh_token_hash", "refresh_token_expires_at", "access_token_expires_at", "last_seen_at", "last_refresh_at", "created_at", "updated_at", "disabled"}).AddRow("watcher-1", "cluster-1", hashed, now.Add(2*time.Hour), now.Add(time.Hour), now, now, now, now, false))

	creds, err := svc.RefreshWatcherTokens(context.Background(), "watcher-1", "cluster-1", refreshToken)
	if err != nil {
		t.Fatalf("RefreshWatcherTokens: %v", err)
	}
	if creds.RefreshToken == refreshToken {
		t.Fatalf("expected new refresh token")
	}
	if creds.AccessToken == "" {
		t.Fatalf("expected access token")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("expectations: %v", err)
	}
}

func TestGetWatcherByCluster(t *testing.T) {
	t.Parallel()

	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	st := store.NewWithDB(db)
	cfg := &config.Config{AdminJWTSecret: "secret", KcfgEncryptionKey: "key"}
	svc, err := service.New(cfg, st, nil)
	if err != nil {
		t.Fatalf("service.New: %v", err)
	}

	now := time.Now()
	mock.ExpectQuery("SELECT id, cluster_id, refresh_token_hash, refresh_token_expires_at, access_token_expires_at, last_seen_at, last_refresh_at, created_at, updated_at, disabled\\s+FROM watchers\\s+WHERE cluster_id = \\$1\\s+ORDER BY updated_at DESC\\s+LIMIT 1").
		WithArgs("cluster-1").
		WillReturnRows(sqlmock.NewRows([]string{"id", "cluster_id", "refresh_token_hash", "refresh_token_expires_at", "access_token_expires_at", "last_seen_at", "last_refresh_at", "created_at", "updated_at", "disabled"}).
			AddRow("watcher-1", "cluster-1", "hash", now.Add(time.Hour), now.Add(time.Minute), now, now, now, now, false))

	watcher, err := svc.GetWatcherByCluster(context.Background(), "cluster-1")
	if err != nil {
		t.Fatalf("GetWatcherByCluster: %v", err)
	}
	if watcher.ID != "watcher-1" {
		t.Fatalf("expected watcher id watcher-1, got %s", watcher.ID)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("expectations: %v", err)
	}
}
