package testcase

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	sqlmock "github.com/DATA-DOG/go-sqlmock"

	"kubeop/internal/store"
)

func TestStoreCreateRelease_Inserts(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	st := store.NewWithDB(db)
	helmChart := "chart"
	helmRender := "abc"
	manifests := "def"
	repo := "https://example"
	rel := store.Release{
		ID:           "rel-1",
		ProjectID:    "proj-1",
		AppID:        "app-1",
		Source:       "image",
		SpecDigest:   "spec",
		RenderDigest: "render",
		Spec: map[string]any{
			"name": "web",
		},
		RenderedObjects: []map[string]any{{"kind": "Deployment", "name": "web"}},
		LoadBalancers:   map[string]any{"requested": 1, "existing": 0, "limit": 5},
		Warnings:        []string{"note"},
		HelmChart:       &helmChart,
		HelmValues:      map[string]any{"replicaCount": 1},
		HelmRenderSHA:   &helmRender,
		ManifestsSHA:    &manifests,
		Repo:            &repo,
		Status:          "succeeded",
	}

	mock.ExpectExec(`INSERT INTO releases`).
		WithArgs(
			rel.ID,
			rel.ProjectID,
			rel.AppID,
			rel.Source,
			rel.SpecDigest,
			rel.RenderDigest,
			sqlmock.AnyArg(),
			sqlmock.AnyArg(),
			sqlmock.AnyArg(),
			sqlmock.AnyArg(),
			sqlmock.AnyArg(),
			sqlmock.AnyArg(),
			sqlmock.AnyArg(),
			sqlmock.AnyArg(),
			sqlmock.AnyArg(),
			rel.Status,
			rel.Message,
		).
		WillReturnResult(sqlmock.NewResult(0, 1))

	if err := st.CreateRelease(context.Background(), rel); err != nil {
		t.Fatalf("CreateRelease: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("expectations: %v", err)
	}
}

func TestStoreGetRelease_Scans(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	st := store.NewWithDB(db)

	specJSON, _ := json.Marshal(map[string]any{"name": "web"})
	renderedJSON, _ := json.Marshal([]map[string]any{{"kind": "Deployment", "name": "web"}})
	lbJSON, _ := json.Marshal(map[string]any{"requested": 1, "existing": 0, "limit": 5})
	warnJSON, _ := json.Marshal([]string{"note"})
	helmVals, _ := json.Marshal(map[string]any{"replicaCount": 1})
	now := time.Now()

	rows := sqlmock.NewRows([]string{
		"id", "project_id", "app_id", "source", "spec_digest", "render_digest",
		"spec", "rendered_objects", "load_balancers", "warnings",
		"helm_chart", "helm_values", "helm_render_sha", "manifests_sha", "repo",
		"status", "message", "created_at",
	}).AddRow(
		"rel-1", "proj-1", "app-1", "image", "spec", "render",
		specJSON, renderedJSON, lbJSON, warnJSON,
		"chart", helmVals, "abc", "def", "repo",
		"succeeded", "", now,
	)

	mock.ExpectQuery(`SELECT id, project_id, app_id, source`).
		WithArgs("rel-1").
		WillReturnRows(rows)

	rel, err := st.GetRelease(context.Background(), "rel-1")
	if err != nil {
		t.Fatalf("GetRelease: %v", err)
	}
	if rel.ID != "rel-1" || rel.ProjectID != "proj-1" {
		t.Fatalf("unexpected release: %#v", rel)
	}
	if rel.Spec["name"].(string) != "web" {
		t.Fatalf("unexpected spec: %#v", rel.Spec)
	}
	if len(rel.RenderedObjects) != 1 {
		t.Fatalf("expected rendered objects")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("expectations: %v", err)
	}
}

func TestStoreListReleasesByApp_WithCursor(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	st := store.NewWithDB(db)
	now := time.Now()

	specJSON, _ := json.Marshal(map[string]any{"name": "web"})
	renderedJSON, _ := json.Marshal([]map[string]any{{"kind": "Deployment", "name": "web"}})
	lbJSON, _ := json.Marshal(map[string]any{"requested": 1})
	warnJSON, _ := json.Marshal([]string{})
	helmVals, _ := json.Marshal(map[string]any{})

	rows := sqlmock.NewRows([]string{
		"id", "project_id", "app_id", "source", "spec_digest", "render_digest",
		"spec", "rendered_objects", "load_balancers", "warnings",
		"helm_chart", "helm_values", "helm_render_sha", "manifests_sha", "repo",
		"status", "message", "created_at",
	}).
		AddRow("rel-0", "proj-1", "app-1", "image", "spec0", "render0", specJSON, renderedJSON, lbJSON, warnJSON, nil, helmVals, nil, nil, nil, "succeeded", "", now.Add(-time.Minute)).
		AddRow("rel-1", "proj-1", "app-1", "image", "spec1", "render1", specJSON, renderedJSON, lbJSON, warnJSON, nil, helmVals, nil, nil, nil, "succeeded", "", now.Add(-2*time.Minute))

	mock.ExpectQuery(`FROM releases WHERE app_id = \$1 AND \(created_at < \$2 OR \(created_at = \$2 AND id < \$3\)\)`).
		WithArgs("app-1", now, "rel-2", 3).
		WillReturnRows(rows)

	rels, err := st.ListReleasesByApp(context.Background(), "app-1", 3, store.ReleaseCursor{ID: "rel-2", CreatedAt: now})
	if err != nil {
		t.Fatalf("ListReleasesByApp: %v", err)
	}
	if len(rels) != 2 {
		t.Fatalf("expected 2 releases, got %d", len(rels))
	}
	if rels[0].ID != "rel-0" {
		t.Fatalf("unexpected first release: %#v", rels[0])
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("expectations: %v", err)
	}
}
