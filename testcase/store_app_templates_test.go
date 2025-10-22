package testcase

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"kubeop/internal/store"
)

func TestStoreAppTemplateUpsertAndGet(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	st := store.NewWithDB(db)

	now := time.Now()
	valuesJSON, _ := json.Marshal(map[string]any{"name": "web"})
	metadataJSON, _ := json.Marshal(map[string]any{"version": "1"})

	mock.ExpectQuery(`INSERT INTO app_templates`).
		WithArgs("app-1", "tpl-1", sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows([]string{"id", "app_id", "template_id", "values", "metadata", "created_at", "updated_at"}).
			AddRow("apptpl-1", "app-1", "tpl-1", valuesJSON, metadataJSON, now, now))

	tpl, err := st.UpsertAppTemplate(context.Background(), "app-1", "tpl-1", map[string]any{"name": "web"}, map[string]any{"version": "1"})
	if err != nil {
		t.Fatalf("UpsertAppTemplate: %v", err)
	}
	if tpl.TemplateID != "tpl-1" || tpl.Values["name"].(string) != "web" {
		t.Fatalf("unexpected template: %#v", tpl)
	}

	mock.ExpectQuery(`SELECT id, app_id, template_id, values, metadata`).
		WithArgs("app-1").
		WillReturnRows(sqlmock.NewRows([]string{"id", "app_id", "template_id", "values", "metadata", "created_at", "updated_at"}).
			AddRow("apptpl-1", "app-1", "tpl-1", valuesJSON, metadataJSON, now, now))

	fetched, err := st.GetAppTemplate(context.Background(), "app-1")
	if err != nil {
		t.Fatalf("GetAppTemplate: %v", err)
	}
	if fetched.TemplateID != "tpl-1" || fetched.Metadata["version"].(string) != "1" {
		t.Fatalf("unexpected fetched template: %#v", fetched)
	}

	mock.ExpectExec(`DELETE FROM app_templates`).
		WithArgs("app-1").
		WillReturnResult(sqlmock.NewResult(0, 1))

	if err := st.DeleteAppTemplate(context.Background(), "app-1"); err != nil {
		t.Fatalf("DeleteAppTemplate: %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("expectations: %v", err)
	}
}
