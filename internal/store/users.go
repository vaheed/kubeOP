package store

import (
    "context"
    "database/sql"
)

func (s *Store) GetUser(ctx context.Context, id string) (User, error) {
    const q = `SELECT id, name, email, created_at FROM users WHERE id = $1 AND deleted_at IS NULL`
    var u User
    if err := s.db.QueryRowContext(ctx, q, id).Scan(&u.ID, &u.Name, &u.Email, &u.CreatedAt); err != nil {
        return User{}, err
    }
    return u, nil
}

func (s *Store) GetUserByEmail(ctx context.Context, email string) (User, error) {
    const q = `SELECT id, name, email, created_at FROM users WHERE email = $1 AND deleted_at IS NULL`
    var u User
    if err := s.db.QueryRowContext(ctx, q, email).Scan(&u.ID, &u.Name, &u.Email, &u.CreatedAt); err != nil {
        return User{}, err
    }
    return u, nil
}

func (s *Store) CreateUser(ctx context.Context, u User) (User, error) {
    const q = `INSERT INTO users (id, name, email) VALUES ($1, $2, $3) RETURNING created_at`
    var createdAt sql.NullTime
    if err := s.db.QueryRowContext(ctx, q, u.ID, u.Name, u.Email).Scan(&createdAt); err != nil {
        return User{}, err
    }
    u.CreatedAt = createdAt.Time
    return u, nil
}

func (s *Store) ListUsers(ctx context.Context, limit, offset int) ([]User, error) {
    if limit <= 0 {
        limit = 100
    }
    if offset < 0 {
        offset = 0
    }
    const q = `SELECT id, name, email, created_at FROM users WHERE deleted_at IS NULL ORDER BY created_at DESC LIMIT $1 OFFSET $2`
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

func (s *Store) SoftDeleteUser(ctx context.Context, id string) error {
    const q = `UPDATE users SET deleted_at = now() WHERE id = $1 AND deleted_at IS NULL`
    _, err := s.db.ExecContext(ctx, q, id)
    return err
}
