package store

import (
	"context"
	"database/sql"
)

func (s *Store) CreateKubeconfigRecord(ctx context.Context, rec KubeconfigRecord, kubeconfigEnc []byte) (KubeconfigRecord, error) {
	const q = `INSERT INTO kubeconfigs (id, cluster_id, namespace, user_id, project_id, service_account, secret_name, kubeconfig_enc)
                VALUES ($1,$2,$3,$4,$5,$6,$7,$8)
                RETURNING created_at, updated_at`
	var createdAt, updatedAt sql.NullTime
	var projectID interface{}
	if rec.ProjectID != nil {
		projectID = *rec.ProjectID
	}
	if err := s.db.QueryRowContext(ctx, q, rec.ID, rec.ClusterID, rec.Namespace, rec.UserID, projectID, rec.ServiceAccount, rec.SecretName, kubeconfigEnc).
		Scan(&createdAt, &updatedAt); err != nil {
		return KubeconfigRecord{}, err
	}
	rec.CreatedAt = createdAt.Time
	rec.UpdatedAt = updatedAt.Time
	return rec, nil
}

func (s *Store) GetKubeconfigByID(ctx context.Context, id string) (KubeconfigRecord, []byte, error) {
	const q = `SELECT id, cluster_id, namespace, user_id, project_id, service_account, secret_name, kubeconfig_enc, created_at, updated_at
                FROM kubeconfigs WHERE id=$1`
	var rec KubeconfigRecord
	var projectID sql.NullString
	var kc []byte
	if err := s.db.QueryRowContext(ctx, q, id).Scan(&rec.ID, &rec.ClusterID, &rec.Namespace, &rec.UserID, &projectID, &rec.ServiceAccount, &rec.SecretName, &kc, &rec.CreatedAt, &rec.UpdatedAt); err != nil {
		return KubeconfigRecord{}, nil, err
	}
	if projectID.Valid {
		rec.ProjectID = &projectID.String
	}
	return rec, kc, nil
}

func (s *Store) GetKubeconfigByProject(ctx context.Context, projectID string) (KubeconfigRecord, []byte, error) {
	const q = `SELECT id, cluster_id, namespace, user_id, project_id, service_account, secret_name, kubeconfig_enc, created_at, updated_at
                FROM kubeconfigs WHERE project_id=$1`
	var rec KubeconfigRecord
	var project sql.NullString
	var kc []byte
	if err := s.db.QueryRowContext(ctx, q, projectID).Scan(&rec.ID, &rec.ClusterID, &rec.Namespace, &rec.UserID, &project, &rec.ServiceAccount, &rec.SecretName, &kc, &rec.CreatedAt, &rec.UpdatedAt); err != nil {
		return KubeconfigRecord{}, nil, err
	}
	if project.Valid {
		rec.ProjectID = &project.String
	}
	return rec, kc, nil
}

func (s *Store) GetKubeconfigByUserScope(ctx context.Context, clusterID, namespace, userID string) (KubeconfigRecord, []byte, error) {
	const q = `SELECT id, cluster_id, namespace, user_id, project_id, service_account, secret_name, kubeconfig_enc, created_at, updated_at
                FROM kubeconfigs WHERE cluster_id=$1 AND namespace=$2 AND user_id=$3 AND project_id IS NULL`
	var rec KubeconfigRecord
	var project sql.NullString
	var kc []byte
	if err := s.db.QueryRowContext(ctx, q, clusterID, namespace, userID).Scan(&rec.ID, &rec.ClusterID, &rec.Namespace, &rec.UserID, &project, &rec.ServiceAccount, &rec.SecretName, &kc, &rec.CreatedAt, &rec.UpdatedAt); err != nil {
		return KubeconfigRecord{}, nil, err
	}
	if project.Valid {
		rec.ProjectID = &project.String
	}
	return rec, kc, nil
}

func (s *Store) UpdateKubeconfigRecord(ctx context.Context, id, secretName, serviceAccount string, kubeconfigEnc []byte) error {
	const q = `UPDATE kubeconfigs SET secret_name=$2, service_account=$3, kubeconfig_enc=$4, updated_at=now() WHERE id=$1`
	_, err := s.db.ExecContext(ctx, q, id, secretName, serviceAccount, kubeconfigEnc)
	return err
}

func (s *Store) DeleteKubeconfigRecord(ctx context.Context, id string) error {
	const q = `DELETE FROM kubeconfigs WHERE id=$1`
	_, err := s.db.ExecContext(ctx, q, id)
	return err
}

func (s *Store) CountKubeconfigsByServiceAccount(ctx context.Context, namespace, serviceAccount string) (int, error) {
	const q = `SELECT COUNT(1) FROM kubeconfigs WHERE namespace=$1 AND service_account=$2`
	var count int
	if err := s.db.QueryRowContext(ctx, q, namespace, serviceAccount).Scan(&count); err != nil {
		return 0, err
	}
	return count, nil
}
