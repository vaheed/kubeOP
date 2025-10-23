package store

import (
	"context"
	"database/sql"
)

func (s *Store) CreateProject(ctx context.Context, p Project, quotaOverridesJSON []byte, kubeconfigEnc []byte) (Project, error) {
	const q = `INSERT INTO projects (id, user_id, cluster_id, name, namespace, suspended, quota_overrides, kubeconfig_enc)
               VALUES ($1,$2,$3,$4,$5,$6,$7,$8) RETURNING created_at`
	var createdAt sql.NullTime
	if err := s.db.QueryRowContext(ctx, q, p.ID, p.UserID, p.ClusterID, p.Name, p.Namespace, p.Suspended, quotaOverridesJSON, kubeconfigEnc).Scan(&createdAt); err != nil {
		return Project{}, err
	}
	p.CreatedAt = createdAt.Time
	return p, nil
}

func (s *Store) GetProject(ctx context.Context, id string) (Project, []byte, []byte, error) {
	const q = `SELECT id, user_id, cluster_id, name, namespace, suspended, created_at, quota_overrides, kubeconfig_enc FROM projects WHERE id = $1 AND deleted_at IS NULL`
	var p Project
	var qo, kc []byte
	if err := s.db.QueryRowContext(ctx, q, id).Scan(&p.ID, &p.UserID, &p.ClusterID, &p.Name, &p.Namespace, &p.Suspended, &p.CreatedAt, &qo, &kc); err != nil {
		return Project{}, nil, nil, err
	}
	return p, qo, kc, nil
}

func (s *Store) UpdateProjectSuspended(ctx context.Context, id string, suspended bool) error {
	const q = `UPDATE projects SET suspended=$2 WHERE id=$1`
	_, err := s.db.ExecContext(ctx, q, id, suspended)
	return err
}

func (s *Store) UpdateProjectQuotaOverrides(ctx context.Context, id string, quotaOverridesJSON []byte) error {
	const q = `UPDATE projects SET quota_overrides=$2 WHERE id=$1`
	_, err := s.db.ExecContext(ctx, q, id, quotaOverridesJSON)
	return err
}

func (s *Store) UpdateProjectKubeconfig(ctx context.Context, id string, kubeconfigEnc []byte) error {
	const q = `UPDATE projects SET kubeconfig_enc=$2 WHERE id=$1`
	_, err := s.db.ExecContext(ctx, q, id, kubeconfigEnc)
	return err
}

// ListProjects returns projects ordered by created_at desc with pagination.
func (s *Store) ListProjects(ctx context.Context, limit, offset int) ([]Project, error) {
	if limit <= 0 {
		limit = 100
	}
	if offset < 0 {
		offset = 0
	}
	const q = `SELECT id, user_id, cluster_id, name, namespace, COALESCE(suspended,false), created_at FROM projects WHERE deleted_at IS NULL ORDER BY created_at DESC LIMIT $1 OFFSET $2`
	rows, err := s.db.QueryContext(ctx, q, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Project
	for rows.Next() {
		var p Project
		if err := rows.Scan(&p.ID, &p.UserID, &p.ClusterID, &p.Name, &p.Namespace, &p.Suspended, &p.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// ListProjectsByUser returns all projects for a given user ordered by created_at desc.
func (s *Store) ListProjectsByUser(ctx context.Context, userID string, limit, offset int) ([]Project, error) {
	if limit <= 0 {
		limit = 100
	}
	if offset < 0 {
		offset = 0
	}
	const q = `SELECT id, user_id, cluster_id, name, namespace, COALESCE(suspended,false), created_at FROM projects WHERE user_id = $1 AND deleted_at IS NULL ORDER BY created_at DESC LIMIT $2 OFFSET $3`
	rows, err := s.db.QueryContext(ctx, q, userID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Project
	for rows.Next() {
		var p Project
		if err := rows.Scan(&p.ID, &p.UserID, &p.ClusterID, &p.Name, &p.Namespace, &p.Suspended, &p.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

func (s *Store) GetProjectByNamespace(ctx context.Context, clusterID, namespace string) (Project, error) {
	const q = `SELECT id, user_id, cluster_id, name, namespace, COALESCE(suspended,false), created_at
FROM projects WHERE cluster_id=$1 AND namespace=$2 AND deleted_at IS NULL ORDER BY created_at LIMIT 1`
	var p Project
	if err := s.db.QueryRowContext(ctx, q, clusterID, namespace).Scan(&p.ID, &p.UserID, &p.ClusterID, &p.Name, &p.Namespace, &p.Suspended, &p.CreatedAt); err != nil {
		return Project{}, err
	}
	return p, nil
}

// SoftDeleteProject marks a project as deleted.
func (s *Store) SoftDeleteProject(ctx context.Context, id string) error {
	const q = `UPDATE projects SET deleted_at = now() WHERE id = $1 AND deleted_at IS NULL`
	_, err := s.db.ExecContext(ctx, q, id)
	return err
}

// SoftDeleteProjectsByUser marks all projects for a user as deleted.
func (s *Store) SoftDeleteProjectsByUser(ctx context.Context, userID string) error {
	const q = `UPDATE projects SET deleted_at = now() WHERE user_id = $1 AND deleted_at IS NULL`
	_, err := s.db.ExecContext(ctx, q, userID)
	return err
}
