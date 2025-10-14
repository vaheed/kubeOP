package testcase

import (
	"context"
	"errors"
	"testing"
	"time"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"kubeop/internal/config"
	"kubeop/internal/kube"
	"kubeop/internal/service"
	"kubeop/internal/store"
	"kubeop/internal/watcherdeploy"
)

type stubWatcher struct {
	called bool
	err    error
}

func (s *stubWatcher) Ensure(ctx context.Context, clusterID, clusterName string, loader watcherdeploy.Loader) error {
	s.called = true
	return s.err
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
		WatcherEventsURL:           "https://kubeop.example.com/v1/events/ingest",
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
	stub := &stubWatcher{}
	svc.SetWatcherProvisioner(stub)

	mock.ExpectQuery("INSERT INTO clusters").WithArgs(sqlmock.AnyArg(), "test", sqlmock.AnyArg()).WillReturnRows(sqlmock.NewRows([]string{"created_at"}).AddRow(time.Now()))

	if _, err := svc.RegisterCluster(context.Background(), "test", "kubeconfig-data"); err != nil {
		t.Fatalf("RegisterCluster: %v", err)
	}
	if !stub.called {
		t.Fatalf("expected watcher ensure called")
	}
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
		WatcherEventsURL:           "https://kubeop.example.com/v1/events/ingest",
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
	stub := &stubWatcher{err: errors.New("boom")}
	svc.SetWatcherProvisioner(stub)

	mock.ExpectQuery("INSERT INTO clusters").WithArgs(sqlmock.AnyArg(), "broken", sqlmock.AnyArg()).WillReturnRows(sqlmock.NewRows([]string{"created_at"}).AddRow(time.Now()))

	if _, err := svc.RegisterCluster(context.Background(), "broken", "cfg"); err == nil {
		t.Fatalf("expected error")
	}
	if !stub.called {
		t.Fatalf("expected watcher ensure called")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("sql expectations: %v", err)
	}
}
