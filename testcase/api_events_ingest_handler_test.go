package testcase

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	sqlmock "github.com/DATA-DOG/go-sqlmock"

	"kubeop/internal/api"
	"kubeop/internal/config"
	"kubeop/internal/service"
	"kubeop/internal/store"
)

func TestEventsIngest_InvalidJSON(t *testing.T) {
	t.Parallel()

	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	st := store.NewWithDB(db)
	cfg := &config.Config{DisableAuth: true, KcfgEncryptionKey: "unit-test", EventsDBEnabled: true, EventsBridgeEnabled: true}
	svc, err := service.New(cfg, st, nil)
	if err != nil {
		t.Fatalf("service.New: %v", err)
	}
	disableMaintenance(t, svc)

	router := api.NewRouter(cfg, svc)
	req := httptest.NewRequest(http.MethodPost, "/v1/events/ingest", strings.NewReader("{"))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rr.Code)
	}
	if !strings.Contains(rr.Body.String(), "decode json") {
		t.Fatalf("expected decode error in body, got %q", rr.Body.String())
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("expectations: %v", err)
	}
}

func TestEventsIngest_Disabled(t *testing.T) {
	t.Parallel()

	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	st := store.NewWithDB(db)
	cfg := &config.Config{DisableAuth: true, KcfgEncryptionKey: "unit-test", EventsDBEnabled: true, EventsBridgeEnabled: false}
	svc, err := service.New(cfg, st, nil)
	if err != nil {
		t.Fatalf("service.New: %v", err)
	}
	disableMaintenance(t, svc)

	router := api.NewRouter(cfg, svc)
	req := httptest.NewRequest(http.MethodPost, "/v1/events/ingest?clusterId=cluster-a", strings.NewReader(`[{"projectId":"proj","kind":"note","message":"hello"}]`))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d", rr.Code)
	}
	var body map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if status, _ := body["status"].(string); status != "ignored" {
		t.Fatalf("expected status ignored, got %q", status)
	}
	if total, _ := body["total"].(float64); total != 1 {
		t.Fatalf("expected total 1, got %v", total)
	}
	if accepted, _ := body["accepted"].(float64); accepted != 0 {
		t.Fatalf("expected accepted 0, got %v", accepted)
	}
	if dropped, _ := body["dropped"].(float64); dropped != 1 {
		t.Fatalf("expected dropped 1, got %v", dropped)
	}
	if cluster, _ := body["clusterId"].(string); cluster != "cluster-a" {
		t.Fatalf("expected clusterId cluster-a, got %q", cluster)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("expectations: %v", err)
	}
}

func TestEventsIngest_Success(t *testing.T) {
	t.Parallel()

	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	st := store.NewWithDB(db)
	cfg := &config.Config{DisableAuth: true, KcfgEncryptionKey: "unit-test", EventsDBEnabled: true, EventsBridgeEnabled: true}
	svc, err := service.New(cfg, st, nil)
	if err != nil {
		t.Fatalf("service.New: %v", err)
	}
	disableMaintenance(t, svc)

	now := time.Now()
	mock.ExpectQuery(`INSERT INTO project_events`).
		WithArgs(sqlmock.AnyArg(), "proj-1", sqlmock.AnyArg(), sqlmock.AnyArg(), "APP_DEPLOYED", "INFO", "deployment complete", sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows([]string{"at"}).AddRow(now))

	router := api.NewRouter(cfg, svc)
	payload := `[{"projectId":"proj-1","kind":"APP_DEPLOYED","message":"deployment complete","severity":"info"}]`
	req := httptest.NewRequest(http.MethodPost, "/v1/events/ingest?clusterId=cluster-z", strings.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d", rr.Code)
	}
	var body map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if total, _ := body["total"].(float64); total != 1 {
		t.Fatalf("expected total 1, got %v", total)
	}
	if accepted, _ := body["accepted"].(float64); accepted != 1 {
		t.Fatalf("expected accepted 1, got %v", accepted)
	}
	if dropped, _ := body["dropped"].(float64); dropped != 0 {
		t.Fatalf("expected dropped 0, got %v", dropped)
	}
	if cluster, _ := body["clusterId"].(string); cluster != "cluster-z" {
		t.Fatalf("expected clusterId cluster-z, got %q", cluster)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("expectations: %v", err)
	}
}
