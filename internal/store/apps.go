package store

import (
    "context"
    "database/sql"
    "encoding/json"
)

type App struct {
    ID        string            `json:"id"`
    ProjectID string            `json:"project_id"`
    Name      string            `json:"name"`
    Status    string            `json:"status"`
    Repo      sql.NullString    `json:"repo"`
    WebhookSecret sql.NullString `json:"webhook_secret"`
    Source    map[string]any    `json:"source"`
}

func (s *Store) CreateApp(ctx context.Context, id, projectID, name, status, repo, webhookSecret string, source map[string]any) error {
    const q = `INSERT INTO apps (id, project_id, name, status, repo, webhook_secret, source) VALUES ($1,$2,$3,$4,$5,$6,$7)`
    b, _ := json.Marshal(source)
    _, err := s.db.ExecContext(ctx, q, id, projectID, name, status, repo, webhookSecret, b)
    return err
}

func (s *Store) FindAppsByRepo(ctx context.Context, repo string) ([]App, error) {
    const q = `SELECT id, project_id, name, status, repo, webhook_secret, source FROM apps WHERE repo = $1`
    rows, err := s.db.QueryContext(ctx, q, repo)
    if err != nil { return nil, err }
    defer rows.Close()
    var out []App
    for rows.Next() {
        var a App
        var b []byte
        if err := rows.Scan(&a.ID, &a.ProjectID, &a.Name, &a.Status, &a.Repo, &a.WebhookSecret, &b); err != nil { return nil, err }
        _ = json.Unmarshal(b, &a.Source)
        out = append(out, a)
    }
    return out, rows.Err()
}
