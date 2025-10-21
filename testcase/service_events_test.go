package testcase

import (
	"context"
	"errors"
	"testing"
	"time"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"kubeop/internal/config"
	"kubeop/internal/service"
	"kubeop/internal/store"
)

func TestAppendProjectEvent_PersistsAndRedacts(t *testing.T) {
	t.Parallel()

	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	st := store.NewWithDB(db)
	cfg := &config.Config{KcfgEncryptionKey: "unit-test", EventsDBEnabled: true}
	svc, err := service.New(cfg, st, nil)
	if err != nil {
		t.Fatalf("service.New: %v", err)
	}
	disableMaintenance(t, svc)

	now := time.Now()
	mock.ExpectQuery(`INSERT INTO project_events`).
		WithArgs(sqlmock.AnyArg(), "proj-1", sqlmock.AnyArg(), sqlmock.AnyArg(), "app_deployed", "INFO", "app deployed", sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows([]string{"at"}).AddRow(now))

	ctx := service.ContextWithActor(context.Background(), "user-123")
	evt, err := svc.AppendProjectEvent(ctx, service.EventInput{
		ProjectID: "proj-1",
		AppID:     "app-77",
		Kind:      "app_deployed",
		Severity:  "info",
		Message:   "app deployed",
		Meta: map[string]any{
			"token": "secret-value",
			"count": 3,
		},
	})
	if err != nil {
		t.Fatalf("AppendProjectEvent: %v", err)
	}
	if evt.ProjectID != "proj-1" || evt.AppID != "app-77" {
		t.Fatalf("unexpected project/app ids: %#v", evt)
	}
	if evt.Kind != "app_deployed" {
		t.Fatalf("expected kind app_deployed, got %q", evt.Kind)
	}
	if evt.Severity != "INFO" {
		t.Fatalf("expected severity INFO, got %q", evt.Severity)
	}
	if evt.ActorUserID != "user-123" {
		t.Fatalf("expected actor user-123, got %q", evt.ActorUserID)
	}
	if evt.Meta["token"] != "[redacted]" {
		t.Fatalf("expected token redacted, got %#v", evt.Meta["token"])
	}
	if evt.Meta["count"] != 3 {
		t.Fatalf("expected count 3, got %#v", evt.Meta["count"])
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("expectations: %v", err)
	}
}

func TestAppendProjectEvent_DisabledSkipsStore(t *testing.T) {
	t.Parallel()

	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	st := store.NewWithDB(db)
	cfg := &config.Config{KcfgEncryptionKey: "unit-test", EventsDBEnabled: false}
	svc, err := service.New(cfg, st, nil)
	if err != nil {
		t.Fatalf("service.New: %v", err)
	}
	disableMaintenance(t, svc)

	evt, err := svc.AppendProjectEvent(context.Background(), service.EventInput{
		ProjectID: "proj-2",
		Kind:      "app_deleted",
		Severity:  "warn",
		Message:   "app removed",
		Meta: map[string]any{
			"password": "should-hide",
		},
	})
	if err != nil {
		t.Fatalf("AppendProjectEvent: %v", err)
	}
	if evt.At.IsZero() {
		t.Fatalf("expected timestamp to be set")
	}
	if evt.Meta["password"] != "[redacted]" {
		t.Fatalf("expected password redacted, got %#v", evt.Meta["password"])
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("expectations: %v", err)
	}
}

func TestListProjectEvents_Disabled(t *testing.T) {
	t.Parallel()

	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	st := store.NewWithDB(db)
	cfg := &config.Config{KcfgEncryptionKey: "unit-test", EventsDBEnabled: false}
	svc, err := service.New(cfg, st, nil)
	if err != nil {
		t.Fatalf("service.New: %v", err)
	}
	disableMaintenance(t, svc)

	if _, err := svc.ListProjectEvents(context.Background(), "proj-3", store.ProjectEventFilter{}); err == nil {
		t.Fatalf("expected error when events db disabled")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("expectations: %v", err)
	}
}

func TestIngestProjectEvents_Disabled(t *testing.T) {
	t.Parallel()

	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	st := store.NewWithDB(db)
	cfg := &config.Config{KcfgEncryptionKey: "unit-test", EventsDBEnabled: true, EventsBridgeEnabled: false}
	svc, err := service.New(cfg, st, nil)
	if err != nil {
		t.Fatalf("service.New: %v", err)
	}
	disableMaintenance(t, svc)

	summary, ingestErr := svc.IngestProjectEvents(context.Background(), "cluster-a", []service.EventInput{{
		ProjectID: "proj-1",
		Kind:      "DEPLOY",
		Message:   "deployed",
	}})
	if !errors.Is(ingestErr, service.ErrEventBridgeDisabled) {
		t.Fatalf("expected ErrEventBridgeDisabled, got %v", ingestErr)
	}
	if summary.Status != "ignored" {
		t.Fatalf("expected status ignored, got %q", summary.Status)
	}
	if summary.Dropped != summary.Total || summary.Total != 1 {
		t.Fatalf("expected dropped=total=1, got %#v", summary)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("expectations: %v", err)
	}
}

func TestIngestProjectEvents_PartialSuccess(t *testing.T) {
	t.Parallel()

	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	st := store.NewWithDB(db)
	cfg := &config.Config{KcfgEncryptionKey: "unit-test", EventsDBEnabled: true, EventsBridgeEnabled: true}
	svc, err := service.New(cfg, st, nil)
	if err != nil {
		t.Fatalf("service.New: %v", err)
	}
	disableMaintenance(t, svc)

	now := time.Now()
	mock.ExpectQuery(`INSERT INTO project_events`).
		WithArgs(sqlmock.AnyArg(), "proj-ok", sqlmock.AnyArg(), sqlmock.AnyArg(), "DEPLOY", "INFO", "ok", sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows([]string{"at"}).AddRow(now))

	summary, ingestErr := svc.IngestProjectEvents(context.Background(), "cluster-b", []service.EventInput{
		{
			ProjectID: "proj-ok",
			Kind:      "DEPLOY",
			Severity:  "info",
			Message:   "ok",
		},
		{
			ProjectID: "",
			Kind:      "DEPLOY",
			Message:   "missing project",
		},
	})
	if ingestErr != nil {
		t.Fatalf("unexpected error: %v", ingestErr)
	}
	if summary.Total != 2 {
		t.Fatalf("expected total 2, got %d", summary.Total)
	}
	if summary.Accepted != 1 || summary.Dropped != 1 {
		t.Fatalf("expected accepted=1 dropped=1, got %#v", summary)
	}
	if len(summary.Errors) != 1 || summary.Errors[0].Index != 1 {
		t.Fatalf("expected one error at index 1, got %#v", summary.Errors)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("expectations: %v", err)
	}
}
