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
	called bool
	err    error
}

func (s *stubWatcher) Ensure(ctx context.Context, clusterID, clusterName string, loader watcherdeploy.Loader) error {
	s.called = true
	return s.err
}

type stubWatcherScheduler struct {
	called      bool
	provisioner watcherdeploy.Provisioner
	lastID      string
	lastName    string
	lastLoader  watcherdeploy.Loader
	ensureErr   error
}

func (s *stubWatcherScheduler) Schedule(ctx context.Context, clusterID, clusterName string, loader watcherdeploy.Loader) {
	s.called = true
	s.lastID = clusterID
	s.lastName = clusterName
	s.lastLoader = loader
	if s.provisioner != nil {
		if err := s.provisioner.Ensure(ctx, clusterID, clusterName, loader); err != nil {
			s.ensureErr = err
		}
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
	sched := &stubWatcherScheduler{provisioner: stub}
	svc.SetWatcherScheduler(sched)

	mock.ExpectQuery("INSERT INTO clusters").WithArgs(sqlmock.AnyArg(), "test", sqlmock.AnyArg()).WillReturnRows(sqlmock.NewRows([]string{"created_at"}).AddRow(time.Now()))

	if _, err := svc.RegisterCluster(context.Background(), "test", "kubeconfig-data"); err != nil {
		t.Fatalf("RegisterCluster: %v", err)
	}
	if !sched.called {
		t.Fatalf("expected watcher ensure scheduled")
	}
	if !stub.called {
		t.Fatalf("expected watcher ensure executed")
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
	sched := &stubWatcherScheduler{provisioner: stub}
	svc.SetWatcherScheduler(sched)

	mock.ExpectQuery("INSERT INTO clusters").WithArgs(sqlmock.AnyArg(), "broken", sqlmock.AnyArg()).WillReturnRows(sqlmock.NewRows([]string{"created_at"}).AddRow(time.Now()))

	if _, err := svc.RegisterCluster(context.Background(), "broken", "cfg"); err != nil {
		t.Fatalf("RegisterCluster: %v", err)
	}
	if !sched.called {
		t.Fatalf("expected watcher ensure scheduled")
	}
	if !stub.called {
		t.Fatalf("expected watcher ensure executed")
	}
	if sched.ensureErr == nil {
		t.Fatalf("expected ensure error recorded")
	}
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
