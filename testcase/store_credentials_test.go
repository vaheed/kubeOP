package testcase

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"kubeop/internal/store"
)

func TestCreateGitCredential_Inserts(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	defer db.Close()
	st := store.NewWithDB(db)

	now := time.Now()
	userID := "user-1"
	cred := store.GitCredential{ID: "cred-1", Name: "my-git", UserID: &userID, AuthType: "TOKEN", Username: ""}
	secret := []byte("secret")
	mock.ExpectQuery(`INSERT INTO git_credentials`).
		WithArgs("cred-1", "my-git", "user-1", nil, "TOKEN", nil, secret).
		WillReturnRows(sqlmock.NewRows([]string{"created_at", "updated_at"}).AddRow(now, now))

	created, err := st.CreateGitCredential(context.Background(), cred, secret)
	if err != nil {
		t.Fatalf("CreateGitCredential: %v", err)
	}
	if created.CreatedAt.IsZero() || created.UpdatedAt.IsZero() {
		t.Fatalf("expected timestamps to be set")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("expectations: %v", err)
	}
}

func TestListGitCredentials_ByUser(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	defer db.Close()
	st := store.NewWithDB(db)

	rows := sqlmock.NewRows([]string{"id", "name", "user_id", "project_id", "auth_type", "username", "created_at", "updated_at"}).
		AddRow("cred-1", "git", "user-1", nil, "TOKEN", "", time.Now(), time.Now())
	mock.ExpectQuery(`SELECT id, name, user_id, project_id, auth_type, username, created_at, updated_at FROM git_credentials WHERE user_id = \$1 ORDER BY created_at DESC`).
		WithArgs("user-1").
		WillReturnRows(rows)

	creds, err := st.ListGitCredentials(context.Background(), store.CredentialFilter{UserID: "user-1"})
	if err != nil {
		t.Fatalf("ListGitCredentials: %v", err)
	}
	if len(creds) != 1 {
		t.Fatalf("expected 1 credential, got %d", len(creds))
	}
	if creds[0].UserID == nil || *creds[0].UserID != "user-1" {
		t.Fatalf("expected user scope user-1, got %#v", creds[0].UserID)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("expectations: %v", err)
	}
}

func TestGetGitCredential_ReturnsSecret(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	defer db.Close()
	st := store.NewWithDB(db)

	now := time.Now()
	rows := sqlmock.NewRows([]string{"id", "name", "user_id", "project_id", "auth_type", "username", "secret_enc", "created_at", "updated_at"}).
		AddRow("cred-1", "git", "user-1", nil, "TOKEN", "", []byte("enc"), now, now)
	mock.ExpectQuery(`SELECT id, name, user_id, project_id, auth_type, username, secret_enc, created_at, updated_at FROM git_credentials WHERE id = \$1`).
		WithArgs("cred-1").
		WillReturnRows(rows)

	cred, secret, err := st.GetGitCredential(context.Background(), "cred-1")
	if err != nil {
		t.Fatalf("GetGitCredential: %v", err)
	}
	if cred.ID != "cred-1" {
		t.Fatalf("expected id cred-1, got %s", cred.ID)
	}
	if string(secret) != "enc" {
		t.Fatalf("expected secret 'enc', got %q", string(secret))
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("expectations: %v", err)
	}
}

func TestDeleteGitCredential_NoRows(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	defer db.Close()
	st := store.NewWithDB(db)

	mock.ExpectExec(`DELETE FROM git_credentials WHERE id = \$1`).
		WithArgs("missing").
		WillReturnResult(sqlmock.NewResult(0, 0))

	if err := st.DeleteGitCredential(context.Background(), "missing"); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("expected sql.ErrNoRows, got %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("expectations: %v", err)
	}
}

func TestCreateRegistryCredential_Inserts(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	defer db.Close()
	st := store.NewWithDB(db)

	now := time.Now()
	projectID := "proj-1"
	cred := store.RegistryCredential{ID: "cred-2", Name: "dockerhub", Registry: "https://index.docker.io/v1/", ProjectID: &projectID, AuthType: "BASIC", Username: "user"}
	secret := []byte("secret")
	mock.ExpectQuery(`INSERT INTO registry_credentials`).
		WithArgs("cred-2", "dockerhub", "https://index.docker.io/v1/", nil, "proj-1", "BASIC", "user", secret).
		WillReturnRows(sqlmock.NewRows([]string{"created_at", "updated_at"}).AddRow(now, now))

	created, err := st.CreateRegistryCredential(context.Background(), cred, secret)
	if err != nil {
		t.Fatalf("CreateRegistryCredential: %v", err)
	}
	if created.Registry != "https://index.docker.io/v1/" {
		t.Fatalf("unexpected registry %s", created.Registry)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("expectations: %v", err)
	}
}

func TestListRegistryCredentials_All(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	defer db.Close()
	st := store.NewWithDB(db)

	rows := sqlmock.NewRows([]string{"id", "name", "registry", "user_id", "project_id", "auth_type", "username", "created_at", "updated_at"}).
		AddRow("cred-2", "dockerhub", "https://index.docker.io/v1/", nil, "proj-1", "BASIC", "user", time.Now(), time.Now())
	mock.ExpectQuery(`SELECT id, name, registry, user_id, project_id, auth_type, username, created_at, updated_at FROM registry_credentials ORDER BY created_at DESC`).
		WillReturnRows(rows)

	creds, err := st.ListRegistryCredentials(context.Background(), store.CredentialFilter{})
	if err != nil {
		t.Fatalf("ListRegistryCredentials: %v", err)
	}
	if len(creds) != 1 {
		t.Fatalf("expected 1 credential, got %d", len(creds))
	}
	if creds[0].ProjectID == nil || *creds[0].ProjectID != "proj-1" {
		t.Fatalf("expected project scope proj-1")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("expectations: %v", err)
	}
}

func TestDeleteRegistryCredential_NoRows(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	defer db.Close()
	st := store.NewWithDB(db)

	mock.ExpectExec(`DELETE FROM registry_credentials WHERE id = \$1`).
		WithArgs("missing").
		WillReturnResult(sqlmock.NewResult(0, 0))

	if err := st.DeleteRegistryCredential(context.Background(), "missing"); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("expected sql.ErrNoRows, got %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("expectations: %v", err)
	}
}
