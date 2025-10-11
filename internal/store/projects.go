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
    const q = `SELECT id, user_id, cluster_id, name, namespace, suspended, created_at, quota_overrides, kubeconfig_enc FROM projects WHERE id = $1`
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

func (s *Store) DeleteProject(ctx context.Context, id string) error {
    const q = `DELETE FROM projects WHERE id = $1`
    _, err := s.db.ExecContext(ctx, q, id)
    return err
}

