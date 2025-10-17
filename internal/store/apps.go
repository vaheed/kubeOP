package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

type App struct {
	ID            string         `json:"id"`
	ProjectID     string         `json:"project_id"`
	Name          string         `json:"name"`
	Status        string         `json:"status"`
	Repo          sql.NullString `json:"repo"`
	WebhookSecret sql.NullString `json:"webhook_secret"`
	ExternalRef   sql.NullString `json:"external_ref"`
	Source        map[string]any `json:"source"`
}

type AppDomain struct {
	ID         string    `json:"id"`
	AppID      string    `json:"app_id"`
	FQDN       string    `json:"fqdn"`
	CertStatus string    `json:"cert_status"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

func (s *Store) CreateApp(ctx context.Context, id, projectID, name, status, repo, webhookSecret, externalRef string, source map[string]any) error {
	const q = `INSERT INTO apps (id, project_id, name, status, repo, webhook_secret, external_ref, source)
VALUES ($1,$2,$3,$4,$5,$6,$7,$8)`
	b, _ := json.Marshal(source)
	_, err := s.db.ExecContext(ctx, q, id, projectID, name, status, repo, webhookSecret, externalRef, b)
	return err
}

func (s *Store) GetAppByExternalRef(ctx context.Context, ref string) (App, error) {
	const q = `SELECT id, project_id, name, status, repo, webhook_secret, external_ref, source FROM apps WHERE external_ref = $1 AND deleted_at IS NULL`
	var a App
	var b []byte
	if err := s.db.QueryRowContext(ctx, q, ref).Scan(&a.ID, &a.ProjectID, &a.Name, &a.Status, &a.Repo, &a.WebhookSecret, &a.ExternalRef, &b); err != nil {
		return App{}, err
	}
	_ = json.Unmarshal(b, &a.Source)
	return a, nil
}

func (s *Store) FindAppsByRepo(ctx context.Context, repo string) ([]App, error) {
	const q = `SELECT id, project_id, name, status, repo, webhook_secret, external_ref, source FROM apps WHERE repo = $1 AND deleted_at IS NULL`
	rows, err := s.db.QueryContext(ctx, q, repo)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []App
	for rows.Next() {
		var a App
		var b []byte
		if err := rows.Scan(&a.ID, &a.ProjectID, &a.Name, &a.Status, &a.Repo, &a.WebhookSecret, &a.ExternalRef, &b); err != nil {
			return nil, err
		}
		_ = json.Unmarshal(b, &a.Source)
		out = append(out, a)
	}
	return out, rows.Err()
}

func (s *Store) ListAppsByProject(ctx context.Context, projectID string) ([]App, error) {
	const q = `SELECT id, project_id, name, status, repo, webhook_secret, external_ref, source FROM apps WHERE project_id = $1 AND deleted_at IS NULL ORDER BY updated_at DESC`
	rows, err := s.db.QueryContext(ctx, q, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []App
	for rows.Next() {
		var a App
		var b []byte
		if err := rows.Scan(&a.ID, &a.ProjectID, &a.Name, &a.Status, &a.Repo, &a.WebhookSecret, &a.ExternalRef, &b); err != nil {
			return nil, err
		}
		_ = json.Unmarshal(b, &a.Source)
		out = append(out, a)
	}
	return out, rows.Err()
}
func (s *Store) GetApp(ctx context.Context, id string) (App, error) {
	const q = `SELECT id, project_id, name, status, repo, webhook_secret, external_ref, source FROM apps WHERE id = $1 AND deleted_at IS NULL`
	var a App
	var b []byte
	if err := s.db.QueryRowContext(ctx, q, id).Scan(&a.ID, &a.ProjectID, &a.Name, &a.Status, &a.Repo, &a.WebhookSecret, &a.ExternalRef, &b); err != nil {
		return App{}, err
	}
	_ = json.Unmarshal(b, &a.Source)
	return a, nil
}

func (s *Store) SoftDeleteApp(ctx context.Context, id string) error {
	const q = `UPDATE apps SET deleted_at = now() WHERE id = $1 AND deleted_at IS NULL`
	_, err := s.db.ExecContext(ctx, q, id)
	return err
}

func (s *Store) SoftDeleteAppsByProject(ctx context.Context, projectID string) error {
	const q = `UPDATE apps SET deleted_at = now() WHERE project_id = $1 AND deleted_at IS NULL`
	_, err := s.db.ExecContext(ctx, q, projectID)
	return err
}

func (s *Store) SoftDeleteAppsByUser(ctx context.Context, userID string) error {
	const q = `UPDATE apps SET deleted_at = now() WHERE project_id IN (SELECT id FROM projects WHERE user_id = $1) AND deleted_at IS NULL`
	_, err := s.db.ExecContext(ctx, q, userID)
	return err
}

func (s *Store) UpsertAppDomain(ctx context.Context, appID, fqdn, certStatus string) (AppDomain, error) {
	if certStatus == "" {
		certStatus = "pending"
	}
	const q = `INSERT INTO app_domains (id, app_id, fqdn, cert_status) VALUES ($1,$2,$3,$4)
ON CONFLICT (app_id, fqdn)
DO UPDATE SET cert_status = EXCLUDED.cert_status, updated_at = now()
RETURNING id, app_id, fqdn, cert_status, created_at, updated_at`
	id := uuid.New().String()
	var out AppDomain
	if err := s.db.QueryRowContext(ctx, q, id, appID, fqdn, certStatus).Scan(&out.ID, &out.AppID, &out.FQDN, &out.CertStatus, &out.CreatedAt, &out.UpdatedAt); err != nil {
		return AppDomain{}, err
	}
	return out, nil
}

func (s *Store) ListAppDomains(ctx context.Context, appID string) ([]AppDomain, error) {
	const q = `SELECT id, app_id, fqdn, cert_status, created_at, updated_at FROM app_domains WHERE app_id = $1 ORDER BY created_at`
	rows, err := s.db.QueryContext(ctx, q, appID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []AppDomain
	for rows.Next() {
		var d AppDomain
		if err := rows.Scan(&d.ID, &d.AppID, &d.FQDN, &d.CertStatus, &d.CreatedAt, &d.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

func (s *Store) DeleteAppDomains(ctx context.Context, appID string) error {
	const q = `DELETE FROM app_domains WHERE app_id = $1`
	_, err := s.db.ExecContext(ctx, q, appID)
	return err
}
