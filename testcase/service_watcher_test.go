package testcase

import (
	"context"
	"errors"
	"testing"
	"time"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	jwt "github.com/golang-jwt/jwt/v5"
	"kubeop/internal/config"
	"kubeop/internal/kube"
	"kubeop/internal/service"
	"kubeop/internal/store"
	"kubeop/internal/watcherdeploy"
)

type stubWatcher struct {
	results []error
	calls   chan error
}

func newStubWatcher(results ...error) *stubWatcher {
	if len(results) == 0 {
		results = []error{nil}
	}
	return &stubWatcher{results: results, calls: make(chan error, len(results))}
}

func (s *stubWatcher) Ensure(ctx context.Context, clusterID, clusterName string, loader watcherdeploy.Loader) error {
	if len(s.results) == 0 {
		s.calls <- nil
		return nil
	}
	err := s.results[0]
	s.results = s.results[1:]
	s.calls <- err
	return err
}

func (s *stubWatcher) waitForCall(t *testing.T, timeout time.Duration) error {
	t.Helper()
	select {
	case err := <-s.calls:
		return err
	case <-time.After(timeout):
		t.Fatalf("timed out waiting for watcher ensure call")
		return nil
	}
}

func TestRegisterClusterInvokesWatcher(t *testing.T) {
	t.Parallel()

	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	st := store.NewWithDB(db)
	cfg := &config.Config{
		KcfgEncryptionKey:          "unit-test",
		WatcherAutoDeploy:          true,
		WatcherURL:                 "https://kubeop.example.com",
		WatcherToken:               "token",
		WatcherNamespace:           "kubeop-system",
		WatcherDeploymentName:      "kubeop-watcher",
		WatcherServiceAccount:      "kubeop-watcher",
		WatcherSecretName:          "kubeop-watcher",
		WatcherImage:               "ghcr.io/vaheed/kubeop:watcher",
		WatcherStorePath:           "/var/lib/kubeop-watcher/state.db",
		WatcherReadyTimeoutSeconds: 60,
	}
	km := kube.NewManager()
	svc, err := service.New(cfg, st, km)
	if err != nil {
		t.Fatalf("service.New: %v", err)
	}
	stub := newStubWatcher(nil)
	svc.SetWatcherProvisioner(stub)

	now := time.Now().UTC()
	deadline := now.Add(3 * time.Minute)
	mock.ExpectQuery("INSERT INTO clusters").
		WithArgs(sqlmock.AnyArg(), "test", sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows([]string{"created_at", "watcher_status", "watcher_status_message", "watcher_status_updated_at", "watcher_ready_at", "watcher_health_deadline"}).
			AddRow(now, "Pending", nil, now, nil, deadline))
	mock.ExpectExec("UPDATE clusters SET watcher_status = ").
		WithArgs(sqlmock.AnyArg(), "Pending", sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("UPDATE clusters SET watcher_status = ").
		WithArgs(sqlmock.AnyArg(), "Deploying", sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("UPDATE clusters SET watcher_status = ").
		WithArgs(sqlmock.AnyArg(), "Ready", sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(0, 1))

	created, err := svc.RegisterCluster(context.Background(), "test", "kubeconfig-data")
	if err != nil {
		t.Fatalf("RegisterCluster: %v", err)
	}
	if created.WatcherStatus != "Pending" {
		t.Fatalf("expected initial watcher status Pending, got %q", created.WatcherStatus)
	}
	if created.WatcherStatusMessage == nil || *created.WatcherStatusMessage != "watcher deployment queued" {
		t.Fatalf("expected pending watcher message, got %v", created.WatcherStatusMessage)
	}
	if err := stub.waitForCall(t, time.Second); err != nil {
		t.Fatalf("expected watcher ensure call: %v", err)
	}
	time.Sleep(50 * time.Millisecond)
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("sql expectations: %v", err)
	}
}

func TestRegisterClusterWatcherError(t *testing.T) {
	t.Parallel()

	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	st := store.NewWithDB(db)
	cfg := &config.Config{
		KcfgEncryptionKey:          "unit-test",
		WatcherAutoDeploy:          true,
		WatcherURL:                 "https://kubeop.example.com",
		WatcherToken:               "token",
		WatcherNamespace:           "kubeop-system",
		WatcherDeploymentName:      "kubeop-watcher",
		WatcherServiceAccount:      "kubeop-watcher",
		WatcherSecretName:          "kubeop-watcher",
		WatcherImage:               "ghcr.io/vaheed/kubeop:watcher",
		WatcherStorePath:           "/var/lib/kubeop-watcher/state.db",
		WatcherReadyTimeoutSeconds: 60,
	}
	svc, err := service.New(cfg, st, kube.NewManager())
	if err != nil {
		t.Fatalf("service.New: %v", err)
	}
	stub := newStubWatcher(errors.New("boom"), nil)
	svc.SetWatcherProvisioner(stub)

	now := time.Now().UTC()
	deadline := now.Add(3 * time.Minute)
	mock.ExpectQuery("INSERT INTO clusters").
		WithArgs(sqlmock.AnyArg(), "broken", sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows([]string{"created_at", "watcher_status", "watcher_status_message", "watcher_status_updated_at", "watcher_ready_at", "watcher_health_deadline"}).
			AddRow(now, "Pending", nil, now, nil, deadline))
	mock.ExpectExec("UPDATE clusters SET watcher_status = ").
		WithArgs(sqlmock.AnyArg(), "Pending", sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("UPDATE clusters SET watcher_status = ").
		WithArgs(sqlmock.AnyArg(), "Deploying", sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("UPDATE clusters SET watcher_status = ").
		WithArgs(sqlmock.AnyArg(), "Failed", sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("UPDATE clusters SET watcher_status = ").
		WithArgs(sqlmock.AnyArg(), "Deploying", sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("UPDATE clusters SET watcher_status = ").
		WithArgs(sqlmock.AnyArg(), "Ready", sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(0, 1))

	if _, err := svc.RegisterCluster(context.Background(), "broken", "cfg"); err != nil {
		t.Fatalf("RegisterCluster: %v", err)
	}
	if firstErr := stub.waitForCall(t, 2*time.Second); firstErr == nil {
		t.Fatalf("expected first attempt to fail")
	}
	if secondErr := stub.waitForCall(t, 2*time.Second); secondErr != nil {
		t.Fatalf("expected second attempt to succeed, got %v", secondErr)
	}
	time.Sleep(50 * time.Millisecond)
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("sql expectations: %v", err)
	}
}

func TestGenerateWatcherTokenClaims(t *testing.T) {
	secret := "super-secret"
	clusterID := "cluster-42"
	tok, err := service.GenerateWatcherToken(secret, clusterID, time.Hour)
	if err != nil {
		t.Fatalf("GenerateWatcherToken: %v", err)
	}
	parsed, err := jwt.Parse(tok, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			t.Fatalf("unexpected signing method %T", token.Method)
		}
		return []byte(secret), nil
	})
	if err != nil {
		t.Fatalf("parse token: %v", err)
	}
	claims, ok := parsed.Claims.(jwt.MapClaims)
	if !ok || !parsed.Valid {
		t.Fatalf("expected jwt.MapClaims and valid token")
	}
	if claims["role"] != "admin" {
		t.Fatalf("expected role=admin, got %v", claims["role"])
	}
	if claims["sub"] != "watcher:"+clusterID {
		t.Fatalf("expected sub watcher:%s, got %v", clusterID, claims["sub"])
	}
	if claims["cluster_id"] != clusterID {
		t.Fatalf("expected cluster_id claim, got %v", claims["cluster_id"])
	}
	exp, ok := claims["exp"].(float64)
	if !ok {
		t.Fatalf("expected exp claim")
	}
	if time.Unix(int64(exp), 0).Before(time.Now()) {
		t.Fatalf("expected exp in the future, got %v", exp)
	}
}
