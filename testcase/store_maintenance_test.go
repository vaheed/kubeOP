package testcase

import (
	"context"
	"testing"
	"time"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"kubeop/internal/store"
)

func TestStoreGetMaintenanceState_ReturnsRow(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	st := store.NewWithDB(db)
	now := time.Now()

	mock.ExpectExec(`INSERT INTO maintenance_state`).
		WithArgs("global").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery(`SELECT id, enabled, message, updated_at, updated_by FROM maintenance_state WHERE id = \$1`).
		WithArgs("global").
		WillReturnRows(sqlmock.NewRows([]string{"id", "enabled", "message", "updated_at", "updated_by"}).
			AddRow("global", true, "upgrading", now, "ops@example.com"))

	state, err := st.GetMaintenanceState(context.Background())
	if err != nil {
		t.Fatalf("GetMaintenanceState: %v", err)
	}
	if !state.Enabled || state.Message != "upgrading" || state.UpdatedBy != "ops@example.com" {
		t.Fatalf("unexpected state: %#v", state)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("expectations: %v", err)
	}
}

func TestStoreUpdateMaintenanceState_Upserts(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	st := store.NewWithDB(db)
	now := time.Now()

	mock.ExpectExec(`INSERT INTO maintenance_state`).
		WithArgs("global").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery(`UPDATE maintenance_state SET enabled = \$1, message = \$2, updated_at = NOW\(\), updated_by = \$3 WHERE id = \$4 RETURNING id, enabled, message, updated_at, updated_by`).
		WithArgs(true, "rolling update", "admin@example.com", "global").
		WillReturnRows(sqlmock.NewRows([]string{"id", "enabled", "message", "updated_at", "updated_by"}).
			AddRow("global", true, "rolling update", now, "admin@example.com"))

	state, err := st.UpdateMaintenanceState(context.Background(), true, "rolling update", "admin@example.com")
	if err != nil {
		t.Fatalf("UpdateMaintenanceState: %v", err)
	}
	if !state.Enabled || state.Message != "rolling update" || state.UpdatedBy != "admin@example.com" {
		t.Fatalf("unexpected state: %#v", state)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("expectations: %v", err)
	}
}
