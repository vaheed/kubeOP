package store

import (
	"context"
	"database/sql"
)

type CredentialFilter struct {
	UserID    string
	ProjectID string
}

func (s *Store) CreateGitCredential(ctx context.Context, c GitCredential, secretEnc []byte) (GitCredential, error) {
	const q = `INSERT INTO git_credentials (id, name, user_id, project_id, auth_type, username, secret_enc)
                VALUES ($1,$2,$3,$4,$5,$6,$7)
                RETURNING created_at, updated_at`
	var createdAt, updatedAt sql.NullTime
	if err := s.db.QueryRowContext(
		ctx,
		q,
		c.ID,
		c.Name,
		nullableStringPtr(c.UserID),
		nullableStringPtr(c.ProjectID),
		c.AuthType,
		nullableValue(c.Username),
		secretEnc,
	).Scan(&createdAt, &updatedAt); err != nil {
		return GitCredential{}, err
	}
	c.CreatedAt = createdAt.Time
	c.UpdatedAt = updatedAt.Time
	return c, nil
}

func (s *Store) GetGitCredential(ctx context.Context, id string) (GitCredential, []byte, error) {
	const q = `SELECT id, name, user_id, project_id, auth_type, username, secret_enc, created_at, updated_at
                FROM git_credentials WHERE id = $1`
	var (
		userID    sql.NullString
		projectID sql.NullString
		username  sql.NullString
		createdAt sql.NullTime
		updatedAt sql.NullTime
		secret    []byte
		out       GitCredential
	)
	if err := s.db.QueryRowContext(ctx, q, id).Scan(
		&out.ID,
		&out.Name,
		&userID,
		&projectID,
		&out.AuthType,
		&username,
		&secret,
		&createdAt,
		&updatedAt,
	); err != nil {
		return GitCredential{}, nil, err
	}
	if userID.Valid {
		out.UserID = &userID.String
	}
	if projectID.Valid {
		out.ProjectID = &projectID.String
	}
	if username.Valid {
		out.Username = username.String
	}
	out.CreatedAt = createdAt.Time
	out.UpdatedAt = updatedAt.Time
	return out, secret, nil
}

func (s *Store) ListGitCredentials(ctx context.Context, filter CredentialFilter) ([]GitCredential, error) {
	base := `SELECT id, name, user_id, project_id, auth_type, username, created_at, updated_at FROM git_credentials`
	var (
		args  []any
		query string
	)
	switch {
	case filter.UserID != "":
		query = base + ` WHERE user_id = $1 ORDER BY created_at DESC`
		args = append(args, filter.UserID)
	case filter.ProjectID != "":
		query = base + ` WHERE project_id = $1 ORDER BY created_at DESC`
		args = append(args, filter.ProjectID)
	default:
		query = base + ` ORDER BY created_at DESC`
	}
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []GitCredential
	for rows.Next() {
		var (
			rec       GitCredential
			userID    sql.NullString
			projectID sql.NullString
			username  sql.NullString
			createdAt sql.NullTime
			updatedAt sql.NullTime
		)
		if err := rows.Scan(
			&rec.ID,
			&rec.Name,
			&userID,
			&projectID,
			&rec.AuthType,
			&username,
			&createdAt,
			&updatedAt,
		); err != nil {
			return nil, err
		}
		if userID.Valid {
			rec.UserID = &userID.String
		}
		if projectID.Valid {
			rec.ProjectID = &projectID.String
		}
		if username.Valid {
			rec.Username = username.String
		}
		rec.CreatedAt = createdAt.Time
		rec.UpdatedAt = updatedAt.Time
		out = append(out, rec)
	}
	return out, rows.Err()
}

func (s *Store) DeleteGitCredential(ctx context.Context, id string) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM git_credentials WHERE id = $1`, id)
	if err != nil {
		return err
	}
	if rows, err := res.RowsAffected(); err == nil && rows == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func (s *Store) CreateRegistryCredential(ctx context.Context, c RegistryCredential, secretEnc []byte) (RegistryCredential, error) {
	const q = `INSERT INTO registry_credentials (id, name, registry, user_id, project_id, auth_type, username, secret_enc)
                VALUES ($1,$2,$3,$4,$5,$6,$7,$8)
                RETURNING created_at, updated_at`
	var createdAt, updatedAt sql.NullTime
	if err := s.db.QueryRowContext(
		ctx,
		q,
		c.ID,
		c.Name,
		c.Registry,
		nullableStringPtr(c.UserID),
		nullableStringPtr(c.ProjectID),
		c.AuthType,
		nullableValue(c.Username),
		secretEnc,
	).Scan(&createdAt, &updatedAt); err != nil {
		return RegistryCredential{}, err
	}
	c.CreatedAt = createdAt.Time
	c.UpdatedAt = updatedAt.Time
	return c, nil
}

func (s *Store) GetRegistryCredential(ctx context.Context, id string) (RegistryCredential, []byte, error) {
	const q = `SELECT id, name, registry, user_id, project_id, auth_type, username, secret_enc, created_at, updated_at
                FROM registry_credentials WHERE id = $1`
	var (
		userID    sql.NullString
		projectID sql.NullString
		username  sql.NullString
		createdAt sql.NullTime
		updatedAt sql.NullTime
		secret    []byte
		out       RegistryCredential
	)
	if err := s.db.QueryRowContext(ctx, q, id).Scan(
		&out.ID,
		&out.Name,
		&out.Registry,
		&userID,
		&projectID,
		&out.AuthType,
		&username,
		&secret,
		&createdAt,
		&updatedAt,
	); err != nil {
		return RegistryCredential{}, nil, err
	}
	if userID.Valid {
		out.UserID = &userID.String
	}
	if projectID.Valid {
		out.ProjectID = &projectID.String
	}
	if username.Valid {
		out.Username = username.String
	}
	out.CreatedAt = createdAt.Time
	out.UpdatedAt = updatedAt.Time
	return out, secret, nil
}

func (s *Store) ListRegistryCredentials(ctx context.Context, filter CredentialFilter) ([]RegistryCredential, error) {
	base := `SELECT id, name, registry, user_id, project_id, auth_type, username, created_at, updated_at FROM registry_credentials`
	var (
		args  []any
		query string
	)
	switch {
	case filter.UserID != "":
		query = base + ` WHERE user_id = $1 ORDER BY created_at DESC`
		args = append(args, filter.UserID)
	case filter.ProjectID != "":
		query = base + ` WHERE project_id = $1 ORDER BY created_at DESC`
		args = append(args, filter.ProjectID)
	default:
		query = base + ` ORDER BY created_at DESC`
	}
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []RegistryCredential
	for rows.Next() {
		var (
			rec       RegistryCredential
			userID    sql.NullString
			projectID sql.NullString
			username  sql.NullString
			createdAt sql.NullTime
			updatedAt sql.NullTime
		)
		if err := rows.Scan(
			&rec.ID,
			&rec.Name,
			&rec.Registry,
			&userID,
			&projectID,
			&rec.AuthType,
			&username,
			&createdAt,
			&updatedAt,
		); err != nil {
			return nil, err
		}
		if userID.Valid {
			rec.UserID = &userID.String
		}
		if projectID.Valid {
			rec.ProjectID = &projectID.String
		}
		if username.Valid {
			rec.Username = username.String
		}
		rec.CreatedAt = createdAt.Time
		rec.UpdatedAt = updatedAt.Time
		out = append(out, rec)
	}
	return out, rows.Err()
}

func (s *Store) DeleteRegistryCredential(ctx context.Context, id string) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM registry_credentials WHERE id = $1`, id)
	if err != nil {
		return err
	}
	if rows, err := res.RowsAffected(); err == nil && rows == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func nullableStringPtr(v *string) any {
	if v == nil {
		return nil
	}
	return *v
}

func nullableValue(v string) any {
	if v == "" {
		return nil
	}
	return v
}
