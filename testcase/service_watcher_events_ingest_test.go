package testcase

import (
	"context"
	"strings"
	"testing"
	"time"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"go.uber.org/zap"

	"kubeop/internal/config"
	"kubeop/internal/service"
	"kubeop/internal/sink"
	"kubeop/internal/store"
)

func newTestService(t *testing.T) (*service.Service, sqlmock.Sqlmock, func()) {
	t.Helper()
	cfg := &config.Config{
		AdminJWTSecret:          "secret",
		KcfgEncryptionKey:       strings.Repeat("a", 32),
		EventsDBEnabled:         true,
		ProjectsInUserNamespace: true,
		PodSecurityLevel:        "restricted",
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
	return svc, mock, cleanup
}

func TestProcessWatcherEvents_InsertsProjectEvent(t *testing.T) {
	svc, mock, cleanup := newTestService(t)
	defer cleanup()

	now := time.Now()
	mock.ExpectQuery(`INSERT INTO project_events`).
		WithArgs(sqlmock.AnyArg(), "proj-1", sqlmock.AnyArg(), sqlmock.AnyArg(), "K8S_POD_ADDED", "INFO", "Pod web-0 added", sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows([]string{"at"}).AddRow(now))

	event := sink.Event{
		ClusterID: "cluster-1",
		EventType: "Added",
		Kind:      "Pod",
		Namespace: "user-1",
		Name:      "web-0",
		Labels: map[string]string{
			"kubeop.project-id": "proj-1",
			"kubeop.app-id":     "app-99",
		},
		Summary:  "Pod web-0 added",
		DedupKey: "abc#123",
	}

	res, err := svc.ProcessWatcherEvents(context.Background(), "cluster-1", []sink.Event{event})
	if err != nil {
		t.Fatalf("ProcessWatcherEvents: %v", err)
	}
	if res.Accepted != 1 || res.Dropped != 0 || res.Total != 1 {
		t.Fatalf("unexpected result: %+v", res)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("expectations: %v", err)
	}
}

func TestProcessWatcherEvents_AllowsMissingAppLabel(t *testing.T) {
	svc, mock, cleanup := newTestService(t)
	defer cleanup()

	now := time.Now()
	mock.ExpectQuery(`INSERT INTO project_events`).
		WithArgs(sqlmock.AnyArg(), "proj-1", sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows([]string{"at"}).AddRow(now))

	event := sink.Event{
		ClusterID: "cluster-1",
		EventType: "Modified",
		Kind:      "Deployment",
		Namespace: "user-1",
		Name:      "web-1",
		Labels: map[string]string{
			"kubeop.project-id": "proj-1",
		},
		Summary:  "deployment updated",
		DedupKey: "dep#99",
	}

	res, err := svc.ProcessWatcherEvents(context.Background(), "cluster-1", []sink.Event{event})
	if err != nil {
		t.Fatalf("ProcessWatcherEvents: %v", err)
	}
	if res.Accepted != 1 || res.Dropped != 0 || res.Total != 1 {
		t.Fatalf("unexpected result: %+v", res)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("expectations: %v", err)
	}
}

func TestProcessWatcherEvents_DropsMissingProject(t *testing.T) {
	svc, mock, cleanup := newTestService(t)
	defer cleanup()

	event := sink.Event{ClusterID: "cluster-1", EventType: "Added", Kind: "Service", Name: "api"}
	res, err := svc.ProcessWatcherEvents(context.Background(), "cluster-1", []sink.Event{event})
	if err != nil {
		t.Fatalf("ProcessWatcherEvents: %v", err)
	}
	if res.Accepted != 0 || res.Dropped != 1 || res.Total != 1 {
		t.Fatalf("unexpected result: %+v", res)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("expectations: %v", err)
	}
}

func TestProcessWatcherEvents_ClusterMismatch(t *testing.T) {
	svc, mock, cleanup := newTestService(t)
	defer cleanup()

	event := sink.Event{ClusterID: "cluster-x", EventType: "Deleted", Kind: "Deployment", Labels: map[string]string{"kubeop.project-id": "proj-2"}}
	res, err := svc.ProcessWatcherEvents(context.Background(), "cluster-1", []sink.Event{event})
	if err != nil {
		t.Fatalf("ProcessWatcherEvents: %v", err)
	}
	if res.Accepted != 0 || res.Dropped != 1 || res.Total != 1 {
		t.Fatalf("unexpected result: %+v", res)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("expectations: %v", err)
	}
}
