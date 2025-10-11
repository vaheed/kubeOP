package store

import (
    "context"
)

func (s *Store) GetUser(ctx context.Context, id string) (User, error) {
    const q = `SELECT id, name, email, created_at FROM users WHERE id = $1`
    var u User
    if err := s.db.QueryRowContext(ctx, q, id).Scan(&u.ID, &u.Name, &u.Email, &u.CreatedAt); err != nil {
        return User{}, err
    }
    return u, nil
}

func (s *Store) ListUsers(ctx context.Context, limit, offset int) ([]User, error) {
    if limit <= 0 {
        limit = 100
    }
    if offset < 0 {
        offset = 0
    }
    const q = `SELECT id, name, email, created_at FROM users ORDER BY created_at DESC LIMIT $1 OFFSET $2`
    rows, err := s.db.QueryContext(ctx, q, limit, offset)
    if err != nil {
        return nil, err
    }
    defer rows.Close()
    var out []User
    for rows.Next() {
        var u User
        if err := rows.Scan(&u.ID, &u.Name, &u.Email, &u.CreatedAt); err != nil {
            return nil, err
        }
        out = append(out, u)
    }
    return out, rows.Err()
}

