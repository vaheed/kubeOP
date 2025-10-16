package testcase

import (
	"context"
	"errors"
	"sync"
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
	mu        sync.Mutex
	calls     int
	err       error
	delay     time.Duration
	started   chan struct{}
	done      chan struct{}
	startOnce sync.Once
	doneOnce  sync.Once
}

func newStubWatcher(err error) *stubWatcher {
	return &stubWatcher{
		err:     err,
		started: make(chan struct{}),
		done:    make(chan struct{}),
	}
}

func (s *stubWatcher) Ensure(ctx context.Context, clusterID, clusterName string, loader watcherdeploy.Loader) error {
	s.mu.Lock()
	s.calls++
	delay := s.delay
	err := s.err
	s.mu.Unlock()
	s.startOnce.Do(func() { close(s.started) })
	if delay > 0 {
		time.Sleep(delay)
	}
	s.doneOnce.Do(func() { close(s.done) })
	return err
}

func (s *stubWatcher) Called() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.calls > 0
}

func (s *stubWatcher) SetDelay(d time.Duration) {
	s.mu.Lock()
	s.delay = d
	s.mu.Unlock()
}

func (s *stubWatcher) WaitStarted(t *testing.T, timeout time.Duration) {
	t.Helper()
	select {
	case <-s.started:
	case <-time.After(timeout):
		t.Fatalf("expected watcher ensure to start within %s", timeout)
	}
}

func (s *stubWatcher) WaitDone(t *testing.T, timeout time.Duration) {
	t.Helper()
	select {
	case <-s.done:
	case <-time.After(timeout):
		t.Fatalf("expected watcher ensure to finish within %s", timeout)
	}
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
	stub := newStubWatcher(nil)
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
	stub.WaitDone(t, time.Second)
	if !stub.Called() {
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
	stub := newStubWatcher(errors.New("boom"))
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
	stub.WaitDone(t, time.Second)
	if !stub.Called() {
		t.Fatalf("expected watcher ensure executed")
	}
	if sched.ensureErr == nil {
		t.Fatalf("expected ensure error recorded")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("sql expectations: %v", err)
	}
}

func TestRegisterClusterSchedulesWatcherWithoutScheduler(t *testing.T) {
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
	stub := newStubWatcher(nil)
	stub.SetDelay(300 * time.Millisecond)
	svc.SetWatcherProvisioner(stub)
	svc.SetWatcherScheduler(nil)

	mock.ExpectQuery("INSERT INTO clusters").WithArgs(sqlmock.AnyArg(), "async", sqlmock.AnyArg()).WillReturnRows(sqlmock.NewRows([]string{"created_at"}).AddRow(time.Now()))

	if _, err := svc.RegisterCluster(context.Background(), "async", "cfg"); err != nil {
		t.Fatalf("RegisterCluster: %v", err)
	}

	stub.WaitStarted(t, time.Second)

	select {
	case <-stub.done:
		t.Fatalf("expected watcher ensure to run asynchronously")
	default:
	}

	stub.WaitDone(t, time.Second)

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
