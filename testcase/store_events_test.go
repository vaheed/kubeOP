package testcase

import (
	"context"
	"testing"
	"time"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"kubeop/internal/store"
)

func TestInsertProjectEvent_Success(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	defer db.Close()
	st := store.NewWithDB(db)

	now := time.Now()
	mock.ExpectQuery(`INSERT INTO project_events`).
		WithArgs(sqlmock.AnyArg(), "proj-1", sqlmock.AnyArg(), sqlmock.AnyArg(), "app_deployed", "INFO", "app deployed", sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows([]string{"at"}).AddRow(now))

	evt := store.ProjectEvent{ID: "evt-1", ProjectID: "proj-1", Kind: "app_deployed", Severity: "INFO", Message: "app deployed"}
	stored, err := st.InsertProjectEvent(context.Background(), evt)
	if err != nil {
		t.Fatalf("insert: %v", err)
	}
	if !stored.At.Equal(now) {
		t.Fatalf("expected at %v, got %v", now, stored.At)
	}
	if stored.Meta != nil {
		t.Fatalf("expected nil meta, got %#v", stored.Meta)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("expectations: %v", err)
	}
}

func TestListProjectEvents_Filters(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	defer db.Close()
	st := store.NewWithDB(db)

	since := time.Now().Add(-1 * time.Hour)
	metaJSON := []byte(`{"details":{"key":"value"}}`)
	rows := sqlmock.NewRows([]string{"id", "project_id", "app_id", "actor_user_id", "kind", "severity", "message", "meta", "at"}).
		AddRow("evt-2", "proj-1", "app-9", "user-7", "APP_DEPLOYED", "INFO", "deployed", metaJSON, time.Now()).
		AddRow("evt-3", "proj-1", "", "user-7", "APP_SCALED", "INFO", "scaled", []byte(`{"replicas":3}`), time.Now().Add(-time.Minute))
	mock.ExpectQuery(`SELECT id, project_id,`).
		WithArgs("proj-1", "APP_DEPLOYED", "INFO", "user-7", since, "%deploy%", "%deploy%", 6).
		WillReturnRows(rows)

	page, err := st.ListProjectEvents(context.Background(), "proj-1", store.ProjectEventFilter{
		Kinds:       []string{"APP_DEPLOYED"},
		Severities:  []string{"INFO"},
		ActorUserID: "user-7",
		Since:       since,
		Search:      "deploy",
		Limit:       5,
	})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(page.Events) != 2 {
		t.Fatalf("expected 2 events, got %d", len(page.Events))
	}
	details, ok := page.Events[0].Meta["details"].(map[string]any)
	if !ok || details["key"] != "value" {
		t.Fatalf("expected meta details key value")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("expectations: %v", err)
	}
}
