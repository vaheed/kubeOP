package testcase

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"kubeop/internal/config"
	"kubeop/internal/service"
	"kubeop/internal/store"
)

func TestServiceGetMaintenanceState_UsesLoader(t *testing.T) {
	db, _, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	cfg := &config.Config{KcfgEncryptionKey: "test"}
	svc, err := service.New(cfg, store.NewWithDB(db), nil)
	if err != nil {
		t.Fatalf("service.New: %v", err)
	}
	now := time.Now().UTC()
	svc.SetMaintenanceLoader(func(ctx context.Context) (store.MaintenanceState, error) {
		return store.MaintenanceState{Enabled: true, Message: "upgrade", UpdatedAt: now, UpdatedBy: "ops"}, nil
	})

	state, err := svc.GetMaintenanceState(context.Background())
	if err != nil {
		t.Fatalf("GetMaintenanceState: %v", err)
	}
	if !state.Enabled || state.Message != "upgrade" || !state.UpdatedAt.Equal(now) || state.UpdatedBy != "ops" {
		t.Fatalf("unexpected state: %#v", state)
	}
}

func TestServiceUpdateMaintenanceState_Validation(t *testing.T) {
	db, _, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	cfg := &config.Config{KcfgEncryptionKey: "test"}
	svc, err := service.New(cfg, store.NewWithDB(db), nil)
	if err != nil {
		t.Fatalf("service.New: %v", err)
	}
	longMsg := strings.Repeat("a", 513)
	_, err = svc.UpdateMaintenanceState(context.Background(), service.MaintenanceUpdateInput{Enabled: true, Message: longMsg})
	if !errors.Is(err, service.ErrInvalidMaintenanceInput) {
		t.Fatalf("expected ErrInvalidMaintenanceInput, got %v", err)
	}
}

func TestServiceUpdateMaintenanceState_Persists(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	cfg := &config.Config{KcfgEncryptionKey: "test"}
	svc, err := service.New(cfg, store.NewWithDB(db), nil)
	if err != nil {
		t.Fatalf("service.New: %v", err)
	}
	now := time.Now()

	mock.ExpectExec(`INSERT INTO maintenance_state`).
		WithArgs("global").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery(`UPDATE maintenance_state SET enabled = \$1, message = \$2, updated_at = NOW\(\), updated_by = \$3 WHERE id = \$4 RETURNING id, enabled, message, updated_at, updated_by`).
		WithArgs(true, "scheduled upgrade", "admin@example.com", "global").
		WillReturnRows(sqlmock.NewRows([]string{"id", "enabled", "message", "updated_at", "updated_by"}).
			AddRow("global", true, "scheduled upgrade", now, "admin@example.com"))

	ctx := service.ContextWithActor(context.Background(), "admin@example.com")
	state, err := svc.UpdateMaintenanceState(ctx, service.MaintenanceUpdateInput{Enabled: true, Message: "scheduled upgrade"})
	if err != nil {
		t.Fatalf("UpdateMaintenanceState: %v", err)
	}
	if !state.Enabled || state.Message != "scheduled upgrade" || state.UpdatedBy != "admin@example.com" {
		t.Fatalf("unexpected state: %#v", state)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("expectations: %v", err)
	}
}

func TestServiceCreateProject_BlockedByMaintenance(t *testing.T) {
	db, _, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	cfg := &config.Config{KcfgEncryptionKey: "test"}
	svc, err := service.New(cfg, store.NewWithDB(db), nil)
	if err != nil {
		t.Fatalf("service.New: %v", err)
	}
	svc.SetMaintenanceLoader(func(ctx context.Context) (store.MaintenanceState, error) {
		return store.MaintenanceState{Enabled: true, Message: "platform upgrade"}, nil
	})

	_, err = svc.CreateProject(context.Background(), service.ProjectCreateInput{UserID: "user-1", ClusterID: "cluster-1", Name: "demo"})
	if err == nil {
		t.Fatalf("expected maintenance error")
	}
	if !errors.Is(err, service.ErrMaintenanceEnabled) {
		t.Fatalf("expected ErrMaintenanceEnabled, got %v", err)
	}
	if !strings.Contains(err.Error(), "platform upgrade") {
		t.Fatalf("expected message in error, got %v", err)
	}
}
