package store

import (
	"context"
	"database/sql"
	"encoding/json"
)

type App struct {
	ID            string         `json:"id"`
	ProjectID     string         `json:"project_id"`
	Name          string         `json:"name"`
	Status        string         `json:"status"`
	Repo          sql.NullString `json:"repo"`
	WebhookSecret sql.NullString `json:"webhook_secret"`
	Source        map[string]any `json:"source"`
}

func (s *Store) CreateApp(ctx context.Context, id, projectID, name, status, repo, webhookSecret string, source map[string]any) error {
	const q = `INSERT INTO apps (id, project_id, name, status, repo, webhook_secret, source) VALUES ($1,$2,$3,$4,$5,$6,$7)`
	b, _ := json.Marshal(source)
	_, err := s.db.ExecContext(ctx, q, id, projectID, name, status, repo, webhookSecret, b)
	return err
}

func (s *Store) FindAppsByRepo(ctx context.Context, repo string) ([]App, error) {
	const q = `SELECT id, project_id, name, status, repo, webhook_secret, source FROM apps WHERE repo = $1 AND deleted_at IS NULL`
	rows, err := s.db.QueryContext(ctx, q, repo)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []App
	for rows.Next() {
		var a App
		var b []byte
		if err := rows.Scan(&a.ID, &a.ProjectID, &a.Name, &a.Status, &a.Repo, &a.WebhookSecret, &b); err != nil {
			return nil, err
		}
		_ = json.Unmarshal(b, &a.Source)
		out = append(out, a)
	}
	return out, rows.Err()
}

func (s *Store) ListAppsByProject(ctx context.Context, projectID string) ([]App, error) {
	const q = `SELECT id, project_id, name, status, repo, webhook_secret, source FROM apps WHERE project_id = $1 AND deleted_at IS NULL ORDER BY updated_at DESC`
	rows, err := s.db.QueryContext(ctx, q, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []App
	for rows.Next() {
		var a App
		var b []byte
		if err := rows.Scan(&a.ID, &a.ProjectID, &a.Name, &a.Status, &a.Repo, &a.WebhookSecret, &b); err != nil {
			return nil, err
		}
		_ = json.Unmarshal(b, &a.Source)
		out = append(out, a)
	}
	return out, rows.Err()
}
func (s *Store) GetApp(ctx context.Context, id string) (App, error) {
	const q = `SELECT id, project_id, name, status, repo, webhook_secret, source FROM apps WHERE id = $1 AND deleted_at IS NULL`
	var a App
	var b []byte
	if err := s.db.QueryRowContext(ctx, q, id).Scan(&a.ID, &a.ProjectID, &a.Name, &a.Status, &a.Repo, &a.WebhookSecret, &b); err != nil {
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
