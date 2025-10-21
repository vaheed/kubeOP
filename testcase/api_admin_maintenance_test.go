package testcase

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"kubeop/internal/api"
	"kubeop/internal/config"
	"kubeop/internal/service"
	"kubeop/internal/store"
)

func TestAdminMaintenanceGet(t *testing.T) {
	db, _, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	cfg := &config.Config{DisableAuth: true, KcfgEncryptionKey: "test"}
	svc, err := service.New(cfg, store.NewWithDB(db), nil)
	if err != nil {
		t.Fatalf("service.New: %v", err)
	}
	now := time.Now().UTC()
	svc.SetMaintenanceLoader(func(ctx context.Context) (store.MaintenanceState, error) {
		return store.MaintenanceState{Enabled: true, Message: "upgrade", UpdatedAt: now, UpdatedBy: "ops"}, nil
	})

	router := api.NewRouter(cfg, svc)
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/admin/maintenance", nil)
	router.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	var resp map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp["message"].(string) != "upgrade" || resp["updatedBy"].(string) != "ops" {
		t.Fatalf("unexpected response: %#v", resp)
	}
}

func TestAdminMaintenancePut(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	cfg := &config.Config{DisableAuth: true, KcfgEncryptionKey: "test"}
	svc, err := service.New(cfg, store.NewWithDB(db), nil)
	if err != nil {
		t.Fatalf("service.New: %v", err)
	}
	now := time.Now()

	mock.ExpectExec(`INSERT INTO maintenance_state`).
		WithArgs("global").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery(`UPDATE maintenance_state SET enabled = \$1, message = \$2, updated_at = NOW\(\), updated_by = \$3 WHERE id = \$4 RETURNING id, enabled, message, updated_at, updated_by`).
		WithArgs(true, "rolling restart", "system", "global").
		WillReturnRows(sqlmock.NewRows([]string{"id", "enabled", "message", "updated_at", "updated_by"}).
			AddRow("global", true, "rolling restart", now, "system"))

	router := api.NewRouter(cfg, svc)
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/v1/admin/maintenance", bytes.NewBufferString(`{"enabled":true,"message":"rolling restart"}`))
	router.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	var resp map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp["message"].(string) != "rolling restart" || resp["updatedBy"].(string) != "system" {
		t.Fatalf("unexpected response: %#v", resp)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("expectations: %v", err)
	}
}
