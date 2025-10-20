package testcase

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"kubeop/internal/config"
	"kubeop/internal/crypto"
	"kubeop/internal/service"
	"kubeop/internal/store"
)

func TestCreateGitCredential_Token(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	defer db.Close()
	st := store.NewWithDB(db)
	cfg := &config.Config{KcfgEncryptionKey: "unit-test"}
	svc, err := service.New(cfg, st, nil)
	if err != nil {
		t.Fatalf("service.New: %v", err)
	}

	mock.ExpectQuery(`SELECT id, name, email, created_at FROM users WHERE id = \$1 AND deleted_at IS NULL`).
		WithArgs("user-1").
		WillReturnRows(sqlmock.NewRows([]string{"id", "name", "email", "created_at"}).AddRow("user-1", "Alice", "alice@example.com", time.Now()))

	mock.ExpectQuery(`INSERT INTO git_credentials`).
		WithArgs(sqlmock.AnyArg(), "prod-git", "user-1", nil, "TOKEN", nil, sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows([]string{"created_at", "updated_at"}).AddRow(time.Now(), time.Now()))

	out, err := svc.CreateGitCredential(context.Background(), service.GitCredentialCreateInput{
		Name: "prod-git",
		Scope: service.CredentialScopeInput{
			Type: "user",
			ID:   "user-1",
		},
		AuthType: "token",
		Token:    "abcd",
	})
	if err != nil {
		t.Fatalf("CreateGitCredential: %v", err)
	}
	if out.ScopeType != service.CredentialScopeUser {
		t.Fatalf("expected user scope, got %s", out.ScopeType)
	}
	if out.AuthType != "TOKEN" {
		t.Fatalf("expected auth type TOKEN, got %s", out.AuthType)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("expectations: %v", err)
	}
}

func TestGetGitCredential_Decrypts(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	defer db.Close()
	st := store.NewWithDB(db)
	cfg := &config.Config{KcfgEncryptionKey: "unit-test"}
	svc, err := service.New(cfg, st, nil)
	if err != nil {
		t.Fatalf("service.New: %v", err)
	}
	payload, _ := json.Marshal(map[string]string{"token": "abcd"})
	enc, err := crypto.EncryptAESGCM(payload, crypto.DeriveKey(cfg.KcfgEncryptionKey))
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}
	now := time.Now()
	mock.ExpectQuery(`SELECT id, name, user_id, project_id, auth_type, username, secret_enc, created_at, updated_at FROM git_credentials WHERE id = \$1`).
		WithArgs("cred-1").
		WillReturnRows(sqlmock.NewRows([]string{"id", "name", "user_id", "project_id", "auth_type", "username", "secret_enc", "created_at", "updated_at"}).AddRow("cred-1", "git", "user-1", nil, "TOKEN", "", enc, now, now))

	out, err := svc.GetGitCredential(context.Background(), "cred-1")
	if err != nil {
		t.Fatalf("GetGitCredential: %v", err)
	}
	if out.Secret.Token != "abcd" {
		t.Fatalf("expected token abcd, got %s", out.Secret.Token)
	}
	if out.ScopeID != "user-1" {
		t.Fatalf("expected scope id user-1, got %s", out.ScopeID)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("expectations: %v", err)
	}
}

func TestDeleteGitCredential_Succeeds(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	defer db.Close()
	st := store.NewWithDB(db)
	cfg := &config.Config{KcfgEncryptionKey: "unit-test"}
	svc, err := service.New(cfg, st, nil)
	if err != nil {
		t.Fatalf("service.New: %v", err)
	}
	payload, _ := json.Marshal(map[string]string{"token": "abcd"})
	enc, err := crypto.EncryptAESGCM(payload, crypto.DeriveKey(cfg.KcfgEncryptionKey))
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}
	now := time.Now()
	mock.ExpectQuery(`SELECT id, name, user_id, project_id, auth_type, username, secret_enc, created_at, updated_at FROM git_credentials WHERE id = \$1`).
		WithArgs("cred-1").
		WillReturnRows(sqlmock.NewRows([]string{"id", "name", "user_id", "project_id", "auth_type", "username", "secret_enc", "created_at", "updated_at"}).AddRow("cred-1", "git", "user-1", nil, "TOKEN", "", enc, now, now))
	mock.ExpectExec(`DELETE FROM git_credentials WHERE id = \$1`).
		WithArgs("cred-1").
		WillReturnResult(sqlmock.NewResult(0, 1))

	if err := svc.DeleteGitCredential(context.Background(), "cred-1"); err != nil {
		t.Fatalf("DeleteGitCredential: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("expectations: %v", err)
	}
}

func TestCreateRegistryCredential_Basic(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	defer db.Close()
	st := store.NewWithDB(db)
	cfg := &config.Config{KcfgEncryptionKey: "unit-test"}
	svc, err := service.New(cfg, st, nil)
	if err != nil {
		t.Fatalf("service.New: %v", err)
	}

	mock.ExpectQuery(`SELECT id, user_id, cluster_id, name, namespace, suspended, created_at, quota_overrides, kubeconfig_enc FROM projects WHERE id = \$1 AND deleted_at IS NULL`).
		WithArgs("proj-1").
		WillReturnRows(sqlmock.NewRows([]string{"id", "user_id", "cluster_id", "name", "namespace", "suspended", "created_at", "quota_overrides", "kubeconfig_enc"}).
			AddRow("proj-1", "user-1", "cluster-1", "proj", "ns", false, time.Now(), []byte(`{}`), []byte("enc")))

	mock.ExpectQuery(`INSERT INTO registry_credentials`).
		WithArgs(sqlmock.AnyArg(), "dockerhub", "https://index.docker.io/v1/", nil, "proj-1", "BASIC", "user", sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows([]string{"created_at", "updated_at"}).AddRow(time.Now(), time.Now()))

	out, err := svc.CreateRegistryCredential(context.Background(), service.RegistryCredentialCreateInput{
		Name:     "dockerhub",
		Registry: "https://index.docker.io/v1/",
		Scope: service.CredentialScopeInput{
			Type: "project",
			ID:   "proj-1",
		},
		AuthType: "basic",
		Username: "user",
		Password: "pass",
	})
	if err != nil {
		t.Fatalf("CreateRegistryCredential: %v", err)
	}
	if out.ScopeType != service.CredentialScopeProject {
		t.Fatalf("expected project scope, got %s", out.ScopeType)
	}
	if out.Registry != "https://index.docker.io/v1/" {
		t.Fatalf("unexpected registry %s", out.Registry)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("expectations: %v", err)
	}
}

func TestGetRegistryCredential_Decrypts(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	defer db.Close()
	st := store.NewWithDB(db)
	cfg := &config.Config{KcfgEncryptionKey: "unit-test"}
	svc, err := service.New(cfg, st, nil)
	if err != nil {
		t.Fatalf("service.New: %v", err)
	}
	payload, _ := json.Marshal(map[string]string{"password": "pass"})
	enc, err := crypto.EncryptAESGCM(payload, crypto.DeriveKey(cfg.KcfgEncryptionKey))
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}
	now := time.Now()
	mock.ExpectQuery(`SELECT id, name, registry, user_id, project_id, auth_type, username, secret_enc, created_at, updated_at FROM registry_credentials WHERE id = \$1`).
		WithArgs("cred-2").
		WillReturnRows(sqlmock.NewRows([]string{"id", "name", "registry", "user_id", "project_id", "auth_type", "username", "secret_enc", "created_at", "updated_at"}).
			AddRow("cred-2", "dockerhub", "https://index.docker.io/v1/", nil, "proj-1", "BASIC", "user", enc, now, now))

	out, err := svc.GetRegistryCredential(context.Background(), "cred-2")
	if err != nil {
		t.Fatalf("GetRegistryCredential: %v", err)
	}
	if out.Secret.Password != "pass" {
		t.Fatalf("expected password pass, got %s", out.Secret.Password)
	}
	if out.ScopeID != "proj-1" {
		t.Fatalf("expected scope proj-1, got %s", out.ScopeID)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("expectations: %v", err)
	}
}

func TestListGitCredentials_InvalidFilter(t *testing.T) {
	svc := &service.Service{}
	if _, err := svc.ListGitCredentials(context.Background(), service.CredentialListInput{UserID: "a", ProjectID: "b"}); err == nil {
		t.Fatalf("expected error when both userId and projectId provided")
	}
}

func TestListRegistryCredentials_InvalidFilter(t *testing.T) {
	svc := &service.Service{}
	if _, err := svc.ListRegistryCredentials(context.Background(), service.CredentialListInput{UserID: "a", ProjectID: "b"}); err == nil {
		t.Fatalf("expected error when both userId and projectId provided")
	}
}
