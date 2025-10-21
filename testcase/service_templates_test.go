package testcase

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"go.uber.org/zap"
	"kubeop/internal/config"
	"kubeop/internal/service"
	"kubeop/internal/store"
)

func TestServiceCreateTemplate_StoresAndValidates(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	st := store.NewWithDB(db)
	cfg := &config.Config{KcfgEncryptionKey: "unit-test"}
	svc, err := service.New(cfg, st, nil)
	if err != nil {
		t.Fatalf("service.New: %v", err)
	}
	disableMaintenance(t, svc)
	svc.SetLogger(zap.NewNop())

	schema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"name": map[string]any{"type": "string"},
		},
		"required": []any{"name"},
	}
	defaults := map[string]any{"name": "web"}
	base := map[string]any{"image": "nginx:1"}
	delivery := "{\n  \"name\": \"{{ .values.name }}\",\n  \"image\": \"{{ .base.image }}\"\n}"

	mock.ExpectExec(`INSERT INTO templates`).
		WithArgs(sqlmock.AnyArg(), "starter", "helm", "Baseline", sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), delivery).
		WillReturnResult(sqlmock.NewResult(0, 1))

	schemaJSON, _ := json.Marshal(schema)
	defaultsJSON, _ := json.Marshal(defaults)
	baseJSON, _ := json.Marshal(base)
	now := time.Now()
	rows := sqlmock.NewRows([]string{"id", "name", "kind", "description", "schema", "defaults", "example", "base", "delivery_template", "created_at"}).
		AddRow("tpl-abc", "starter", "helm", "Baseline", schemaJSON, defaultsJSON, []byte("null"), baseJSON, delivery, now)
	mock.ExpectQuery(`SELECT id, name, kind, description, schema, defaults, example, base, delivery_template, created_at FROM templates WHERE id = \$1`).
		WithArgs(sqlmock.AnyArg()).
		WillReturnRows(rows)

	out, err := svc.CreateTemplate(context.Background(), service.TemplateCreateInput{
		Name:             "starter",
		Kind:             "helm",
		Description:      "Baseline",
		Schema:           schema,
		Defaults:         defaults,
		Base:             base,
		DeliveryTemplate: delivery,
	})
	if err != nil {
		t.Fatalf("CreateTemplate: %v", err)
	}
	if out.ID != "tpl-abc" {
		t.Fatalf("expected stored id tpl-abc, got %s", out.ID)
	}
	if out.Defaults["name"].(string) != "web" {
		t.Fatalf("expected default name web, got %#v", out.Defaults["name"])
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("expectations: %v", err)
	}
}

func TestServiceRenderTemplate_MergesValues(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	st := store.NewWithDB(db)
	cfg := &config.Config{KcfgEncryptionKey: "unit-test"}
	svc, err := service.New(cfg, st, nil)
	if err != nil {
		t.Fatalf("service.New: %v", err)
	}
	disableMaintenance(t, svc)
	svc.SetLogger(zap.NewNop())

	schema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"name":     map[string]any{"type": "string"},
			"replicas": map[string]any{"type": "integer"},
		},
		"required": []any{"name"},
	}
	defaults := map[string]any{"name": "web", "replicas": 1}
	base := map[string]any{"image": "nginx:1"}
	delivery := "{\n  \"name\": \"{{ .values.name }}\",\n  \"replicas\": {{ .values.replicas }},\n  \"image\": \"{{ .base.image }}\"\n}"

	schemaJSON, _ := json.Marshal(schema)
	defaultsJSON, _ := json.Marshal(defaults)
	baseJSON, _ := json.Marshal(base)
	now := time.Now()
	rows := sqlmock.NewRows([]string{"id", "name", "kind", "description", "schema", "defaults", "example", "base", "delivery_template", "created_at"}).
		AddRow("tpl-abc", "starter", "helm", "Baseline", schemaJSON, defaultsJSON, []byte("null"), baseJSON, delivery, now)
	mock.ExpectQuery(`SELECT id, name, kind, description, schema, defaults, example, base, delivery_template, created_at FROM templates WHERE id = \$1`).
		WithArgs("tpl-abc").
		WillReturnRows(rows)

	render, err := svc.RenderTemplate(context.Background(), "tpl-abc", map[string]any{"replicas": 3})
	if err != nil {
		t.Fatalf("RenderTemplate: %v", err)
	}
	if render.App.Replicas == nil || *render.App.Replicas != 3 {
		t.Fatalf("expected replicas 3, got %#v", render.App.Replicas)
	}
	if render.App.Image != "nginx:1" {
		t.Fatalf("expected image merged from base, got %s", render.App.Image)
	}
	if render.Values["name"].(string) != "web" {
		t.Fatalf("expected default name web, got %#v", render.Values["name"])
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("expectations: %v", err)
	}
}

func TestServiceDeployTemplate_UsesHook(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	st := store.NewWithDB(db)
	cfg := &config.Config{KcfgEncryptionKey: "unit-test"}
	svc, err := service.New(cfg, st, nil)
	if err != nil {
		t.Fatalf("service.New: %v", err)
	}
	disableMaintenance(t, svc)
	svc.SetLogger(zap.NewNop())

	schema := map[string]any{"type": "object", "properties": map[string]any{"name": map[string]any{"type": "string"}}, "required": []any{"name"}}
	defaults := map[string]any{"name": "web"}
	delivery := "{\n  \"name\": \"{{ .values.name }}\",\n  \"image\": \"nginx:1\"\n}"

	schemaJSON, _ := json.Marshal(schema)
	defaultsJSON, _ := json.Marshal(defaults)
	now := time.Now()
	rows := sqlmock.NewRows([]string{"id", "name", "kind", "description", "schema", "defaults", "example", "base", "delivery_template", "created_at"}).
		AddRow("tpl-abc", "starter", "helm", "Baseline", schemaJSON, defaultsJSON, []byte("null"), []byte("{}"), delivery, now)
	mock.ExpectQuery(`SELECT id, name, kind, description, schema, defaults, example, base, delivery_template, created_at FROM templates WHERE id = \$1`).
		WithArgs("tpl-abc").
		WillReturnRows(rows)

	var captured service.AppDeployInput
	svc.SetDeployAppFunc(func(ctx context.Context, in service.AppDeployInput) (service.AppDeployOutput, error) {
		captured = in
		return service.AppDeployOutput{AppID: "app-123", Name: in.Name}, nil
	})

	out, err := svc.DeployTemplate(context.Background(), "proj-1", "tpl-abc", nil)
	if err != nil {
		t.Fatalf("DeployTemplate: %v", err)
	}
	if captured.ProjectID != "proj-1" {
		t.Fatalf("expected projectId proj-1, got %s", captured.ProjectID)
	}
	if captured.Name != "web" {
		t.Fatalf("expected name web, got %s", captured.Name)
	}
	if out.AppID != "app-123" {
		t.Fatalf("expected app id app-123, got %s", out.AppID)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("expectations: %v", err)
	}
}

func TestServiceCreateTemplate_InvalidDefaults(t *testing.T) {
	db, _, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	st := store.NewWithDB(db)
	cfg := &config.Config{KcfgEncryptionKey: "unit-test"}
	svc, err := service.New(cfg, st, nil)
	if err != nil {
		t.Fatalf("service.New: %v", err)
	}
	disableMaintenance(t, svc)
	svc.SetLogger(zap.NewNop())

	_, err = svc.CreateTemplate(context.Background(), service.TemplateCreateInput{
		Name:        "starter",
		Kind:        "helm",
		Description: "Baseline",
		Schema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"name": map[string]any{"type": "string"},
			},
			"required": []any{"name"},
		},
		Defaults:         map[string]any{"name": 123},
		DeliveryTemplate: "{\n  \"name\": \"{{ .values.name }}\"\n}",
	})
	if err == nil {
		t.Fatalf("expected error for invalid defaults")
	}
	if !strings.Contains(err.Error(), "defaults do not satisfy schema") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestServiceRenderTemplate_InvalidValues(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	st := store.NewWithDB(db)
	cfg := &config.Config{KcfgEncryptionKey: "unit-test"}
	svc, err := service.New(cfg, st, nil)
	if err != nil {
		t.Fatalf("service.New: %v", err)
	}
	disableMaintenance(t, svc)
	svc.SetLogger(zap.NewNop())

	schema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"name":     map[string]any{"type": "string"},
			"replicas": map[string]any{"type": "integer"},
		},
		"required": []any{"name"},
	}
	defaults := map[string]any{"name": "web", "replicas": 1}
	base := map[string]any{"image": "nginx:1"}
	delivery := "{\n  \"name\": \"{{ .values.name }}\",\n  \"replicas\": {{ .values.replicas }},\n  \"image\": \"{{ .base.image }}\"\n}"

	schemaJSON, _ := json.Marshal(schema)
	defaultsJSON, _ := json.Marshal(defaults)
	baseJSON, _ := json.Marshal(base)
	now := time.Now()
	rows := sqlmock.NewRows([]string{"id", "name", "kind", "description", "schema", "defaults", "example", "base", "delivery_template", "created_at"}).
		AddRow("tpl-abc", "starter", "helm", "Baseline", schemaJSON, defaultsJSON, []byte("null"), baseJSON, delivery, now)
	mock.ExpectQuery(`SELECT id, name, kind, description, schema, defaults, example, base, delivery_template, created_at FROM templates WHERE id = \$1`).
		WithArgs("tpl-abc").
		WillReturnRows(rows)

	_, err = svc.RenderTemplate(context.Background(), "tpl-abc", map[string]any{"replicas": "nope"})
	if err == nil {
		t.Fatalf("expected validation error")
	}
	if !strings.Contains(err.Error(), "values do not satisfy schema") {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("expectations: %v", err)
	}
}

func TestServiceCreateTemplate_InvalidDeliveryTemplate(t *testing.T) {
	db, _, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	st := store.NewWithDB(db)
	cfg := &config.Config{KcfgEncryptionKey: "unit-test"}
	svc, err := service.New(cfg, st, nil)
	if err != nil {
		t.Fatalf("service.New: %v", err)
	}
	disableMaintenance(t, svc)
	svc.SetLogger(zap.NewNop())

	_, err = svc.CreateTemplate(context.Background(), service.TemplateCreateInput{
		Name:        "starter",
		Kind:        "helm",
		Description: "Baseline",
		Schema: map[string]any{
			"type":       "object",
			"properties": map[string]any{"name": map[string]any{"type": "string"}},
			"required":   []any{"name"},
		},
		Defaults:         map[string]any{"name": "web"},
		DeliveryTemplate: "{{ .values.name",
	})
	if err == nil {
		t.Fatalf("expected template parse error")
	}
	if !strings.Contains(err.Error(), "render defaults") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestServiceDeployTemplate_PropagatesHookError(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	st := store.NewWithDB(db)
	cfg := &config.Config{KcfgEncryptionKey: "unit-test"}
	svc, err := service.New(cfg, st, nil)
	if err != nil {
		t.Fatalf("service.New: %v", err)
	}
	disableMaintenance(t, svc)
	svc.SetLogger(zap.NewNop())

	schema := map[string]any{"type": "object", "properties": map[string]any{"name": map[string]any{"type": "string"}}, "required": []any{"name"}}
	defaults := map[string]any{"name": "web"}
	delivery := "{\n  \"name\": \"{{ .values.name }}\",\n  \"image\": \"nginx:1\"\n}"

	schemaJSON, _ := json.Marshal(schema)
	defaultsJSON, _ := json.Marshal(defaults)
	now := time.Now()
	rows := sqlmock.NewRows([]string{"id", "name", "kind", "description", "schema", "defaults", "example", "base", "delivery_template", "created_at"}).
		AddRow("tpl-abc", "starter", "helm", "Baseline", schemaJSON, defaultsJSON, []byte("null"), []byte("{}"), delivery, now)
	mock.ExpectQuery(`SELECT id, name, kind, description, schema, defaults, example, base, delivery_template, created_at FROM templates WHERE id = \$1`).
		WithArgs("tpl-abc").
		WillReturnRows(rows)

	svc.SetDeployAppFunc(func(ctx context.Context, in service.AppDeployInput) (service.AppDeployOutput, error) {
		return service.AppDeployOutput{}, errors.New("boom")
	})

	_, err = svc.DeployTemplate(context.Background(), "proj-1", "tpl-abc", nil)
	if err == nil {
		t.Fatalf("expected deploy hook error")
	}
	if !strings.Contains(err.Error(), "boom") {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("expectations: %v", err)
	}
}

func TestServiceRenderTemplate_MissingTemplate(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	st := store.NewWithDB(db)
	cfg := &config.Config{KcfgEncryptionKey: "unit-test"}
	svc, err := service.New(cfg, st, nil)
	if err != nil {
		t.Fatalf("service.New: %v", err)
	}
	disableMaintenance(t, svc)
	svc.SetLogger(zap.NewNop())

	mock.ExpectQuery(`SELECT id, name, kind, description, schema, defaults, example, base, delivery_template, created_at FROM templates WHERE id = \$1`).
		WithArgs("tpl-missing").
		WillReturnError(sql.ErrNoRows)

	_, err = svc.RenderTemplate(context.Background(), "tpl-missing", nil)
	if !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("expected sql.ErrNoRows, got %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("expectations: %v", err)
	}
}
