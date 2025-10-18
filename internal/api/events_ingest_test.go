package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"
	"time"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/golang-jwt/jwt/v5"
	"go.uber.org/zap"

	"kubeop/internal/config"
	"kubeop/internal/service"
	"kubeop/internal/sink"
	"kubeop/internal/store"
)

func TestIngestWatcherEventsRecoversClusterIDFromWatcher(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		AdminJWTSecret:    "secret",
		KcfgEncryptionKey: strings.Repeat("k", 32),
		K8SEventsBridge:   true,
		EventsDBEnabled:   false,
	}

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
	query := regexp.QuoteMeta("SELECT id, cluster_id, refresh_token_hash, refresh_token_expires_at, access_token_expires_at, last_seen_at, last_refresh_at, created_at, updated_at, disabled FROM watchers WHERE id = $1")
	mock.ExpectQuery(query).
		WithArgs("watcher-1").
		WillReturnRows(sqlmock.NewRows([]string{"id", "cluster_id", "refresh_token_hash", "refresh_token_expires_at", "access_token_expires_at", "last_seen_at", "last_refresh_at", "created_at", "updated_at", "disabled"}).
			AddRow("watcher-1", "cluster-1", "hash", now.Add(24*time.Hour), now.Add(time.Hour), now, now, now, now, false))

	a := &API{cfg: cfg, svc: svc}

	evt := sink.Event{
		EventType: "Added",
		Kind:      "Pod",
		Namespace: "user-demo",
		Name:      "pod-1",
		Summary:   "created",
		DedupKey:  "pod#1",
		Labels: map[string]string{
			"kubeop.project-id": "proj-1",
			"kubeop.app-id":     "app-1",
		},
	}
	payload, err := json.Marshal([]sink.Event{evt})
	if err != nil {
		t.Fatalf("marshal events: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/v1/events/ingest", bytes.NewReader(payload))
	claims := jwt.MapClaims{
		"role":       "watcher",
		"watcher_id": "watcher-1",
	}
	ctx := context.WithValue(req.Context(), ctxClaimsKey{}, claims)
	req = req.WithContext(ctx)

	rr := httptest.NewRecorder()
	a.ingestWatcherEvents(rr, req)

	if rr.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d", rr.Code)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("sql expectations: %v", err)
	}
}
