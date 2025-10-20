package testcase

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"kubeop/internal/store"
)

func TestStoreCreateTemplate_Inserts(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	st := store.NewWithDB(db)
	tpl := store.Template{
		ID:               "tpl-1",
		Name:             "nginx",
		Kind:             "helm",
		Description:      "nginx baseline",
		Schema:           map[string]any{"type": "object"},
		Defaults:         map[string]any{"name": "web"},
		Base:             map[string]any{"image": "nginx:1"},
		DeliveryTemplate: "{\n  \"name\": \"{{ .values.name }}\"\n}",
	}

	mock.ExpectExec(`INSERT INTO templates`).
		WithArgs(
			tpl.ID,
			tpl.Name,
			tpl.Kind,
			tpl.Description,
			sqlmock.AnyArg(),
			sqlmock.AnyArg(),
			sqlmock.AnyArg(),
			sqlmock.AnyArg(),
			tpl.DeliveryTemplate,
		).
		WillReturnResult(sqlmock.NewResult(0, 1))

	if err := st.CreateTemplate(context.Background(), tpl); err != nil {
		t.Fatalf("CreateTemplate: %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("expectations: %v", err)
	}
}

func TestStoreListTemplates_Scans(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	st := store.NewWithDB(db)
	schemaJSON, _ := json.Marshal(map[string]any{"type": "object"})
	defaultsJSON, _ := json.Marshal(map[string]any{"name": "web"})
	exampleJSON, _ := json.Marshal(map[string]any{"name": "web"})
	baseJSON, _ := json.Marshal(map[string]any{"image": "nginx:1"})
	now := time.Now()

	rows := sqlmock.NewRows([]string{"id", "name", "kind", "description", "schema", "defaults", "example", "base", "delivery_template", "created_at"}).
		AddRow("tpl-1", "nginx", "helm", "desc", schemaJSON, defaultsJSON, exampleJSON, baseJSON, "{}", now)
	mock.ExpectQuery(`SELECT id, name, kind, description, schema, defaults, example, base, delivery_template, created_at FROM templates\s+ORDER BY created_at DESC`).
		WillReturnRows(rows)

	templates, err := st.ListTemplates(context.Background())
	if err != nil {
		t.Fatalf("ListTemplates: %v", err)
	}
	if len(templates) != 1 {
		t.Fatalf("expected 1 template, got %d", len(templates))
	}
	if templates[0].Defaults["name"].(string) != "web" {
		t.Fatalf("unexpected defaults: %#v", templates[0].Defaults)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("expectations: %v", err)
	}
}

func TestStoreListTemplates_Empty(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	st := store.NewWithDB(db)
	rows := sqlmock.NewRows([]string{"id", "name", "kind", "description", "schema", "defaults", "example", "base", "delivery_template", "created_at"})
	mock.ExpectQuery(`SELECT id, name, kind, description, schema, defaults, example, base, delivery_template, created_at FROM templates\s+ORDER BY created_at DESC`).
		WillReturnRows(rows)

	templates, err := st.ListTemplates(context.Background())
	if err != nil {
		t.Fatalf("ListTemplates: %v", err)
	}
	if len(templates) != 0 {
		t.Fatalf("expected 0 templates, got %d", len(templates))
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("expectations: %v", err)
	}
}
