package testcase

import (
	"context"
	"encoding/json"
	"testing"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"kubeop/internal/config"
	"kubeop/internal/service"
	"kubeop/internal/store"
)

func TestServiceGetAppDelivery(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	st := store.NewWithDB(db)
	svc, err := service.New(&config.Config{KcfgEncryptionKey: "key"}, st, nil)
	if err != nil {
		t.Fatalf("service.New: %v", err)
	}

	sourceJSON, _ := json.Marshal(map[string]any{"image": "nginx"})
	deliveryJSON, _ := json.Marshal(map[string]any{"type": "image", "sbom": map[string]any{"sourceType": "image"}})

	mock.ExpectQuery(`SELECT id, project_id, name, status, repo, webhook_secret, external_ref, source, delivery FROM apps`).
		WithArgs("app-1").
		WillReturnRows(sqlmock.NewRows([]string{"id", "project_id", "name", "status", "repo", "webhook_secret", "external_ref", "source", "delivery"}).
			AddRow("app-1", "proj-1", "web", "deployed", nil, nil, nil, sourceJSON, deliveryJSON))

	info, err := svc.GetAppDelivery(context.Background(), "proj-1", "app-1")
	if err != nil {
		t.Fatalf("GetAppDelivery: %v", err)
	}
	if info.AppID != "app-1" || info.Delivery["type"].(string) != "image" {
		t.Fatalf("unexpected delivery info: %#v", info)
	}
	if info.SBOM["sourceType"].(string) != "image" {
		t.Fatalf("expected sbom sourceType image, got %#v", info.SBOM)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("expectations: %v", err)
	}
}
