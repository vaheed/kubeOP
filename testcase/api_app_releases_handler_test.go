package testcase

import (
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

func TestListAppReleasesHandler(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	st := store.NewWithDB(db)
	cfg := &config.Config{DisableAuth: true, KcfgEncryptionKey: "unit-test"}
	svc, err := service.New(cfg, st, nil)
	if err != nil {
		t.Fatalf("service.New: %v", err)
	}

	mock.ExpectQuery(`SELECT id, project_id, name, status, repo, webhook_secret, external_ref, source FROM apps`).
		WithArgs("app-1").
		WillReturnRows(sqlmock.NewRows([]string{"id", "project_id", "name", "status", "repo", "webhook_secret", "external_ref", "source"}).
			AddRow("app-1", "proj-1", "web", "deployed", nil, nil, nil, []byte(`{"image":"nginx"}`)))

	specJSON := []byte(`{"name":"web"}`)
	renderedJSON := []byte(`[{"kind":"Deployment","name":"web"}]`)
	lbJSON := []byte(`{"requested":1,"existing":0,"limit":5}`)
	warnJSON := []byte(`[]`)
	helmVals := []byte(`{}`)
	now := time.Now()
	rows := sqlmock.NewRows([]string{
		"id", "project_id", "app_id", "source", "spec_digest", "render_digest",
		"spec", "rendered_objects", "load_balancers", "warnings",
		"helm_chart", "helm_values", "helm_render_sha", "manifests_sha", "repo",
		"status", "message", "created_at",
	}).
		AddRow("rel-1", "proj-1", "app-1", "image", "spec", "render", specJSON, renderedJSON, lbJSON, warnJSON, nil, helmVals, nil, nil, nil, "succeeded", "", now)

	mock.ExpectQuery(`SELECT id, project_id, app_id, source`).
		WithArgs("app-1", 2).
		WillReturnRows(rows)

	router := api.NewRouter(cfg, svc)
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/projects/proj-1/apps/app-1/releases?limit=1", nil)
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d (%s)", rr.Code, rr.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	rels, ok := resp["releases"].([]any)
	if !ok || len(rels) != 1 {
		t.Fatalf("expected releases array: %#v", resp)
	}
	first := rels[0].(map[string]any)
	if first["id"].(string) != "rel-1" {
		t.Fatalf("unexpected release id: %#v", first["id"])
	}
	if first["spec"].(map[string]any)["name"].(string) != "web" {
		t.Fatalf("spec not returned: %#v", first["spec"])
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("expectations: %v", err)
	}
}

func TestListAppReleasesHandler_InvalidLimit(t *testing.T) {
	db, _, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	st := store.NewWithDB(db)
	cfg := &config.Config{DisableAuth: true, KcfgEncryptionKey: "unit-test"}
	svc, err := service.New(cfg, st, nil)
	if err != nil {
		t.Fatalf("service.New: %v", err)
	}

	router := api.NewRouter(cfg, svc)
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/projects/proj-1/apps/app-1/releases?limit=oops", nil)
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rr.Code)
	}
}

func TestListAppReleasesHandler_ServiceError(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	st := store.NewWithDB(db)
	cfg := &config.Config{DisableAuth: true, KcfgEncryptionKey: "unit-test"}
	svc, err := service.New(cfg, st, nil)
	if err != nil {
		t.Fatalf("service.New: %v", err)
	}

	mock.ExpectQuery(`SELECT id, project_id, name, status, repo, webhook_secret, external_ref, source FROM apps`).
		WithArgs("app-1").
		WillReturnRows(sqlmock.NewRows([]string{"id", "project_id", "name", "status", "repo", "webhook_secret", "external_ref", "source"}).
			AddRow("app-1", "proj-2", "web", "deployed", nil, nil, nil, []byte(`{"image":"nginx"}`)))

	router := api.NewRouter(cfg, svc)
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/projects/proj-1/apps/app-1/releases", nil)
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d (%s)", rr.Code, rr.Body.String())
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("expectations: %v", err)
	}
}
