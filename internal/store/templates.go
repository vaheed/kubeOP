package store

import (
    "context"
    "encoding/json"
)

func (s *Store) CreateTemplate(ctx context.Context, id, name, kind string, spec map[string]any) error {
    const q = `INSERT INTO templates (id, name, kind, spec) VALUES ($1,$2,$3,$4)`
    b, _ := json.Marshal(spec)
    _, err := s.db.ExecContext(ctx, q, id, name, kind, b)
    return err
}

