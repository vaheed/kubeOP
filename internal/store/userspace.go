package store

import (
	"context"
	"database/sql"
)

func (s *Store) CreateUserSpace(ctx context.Context, us UserSpace, kubeconfigEnc []byte) (UserSpace, error) {
	const q = `INSERT INTO user_spaces (id, user_id, cluster_id, namespace, kubeconfig_enc)
               VALUES ($1,$2,$3,$4,$5) RETURNING created_at`
	var createdAt sql.NullTime
	if err := s.db.QueryRowContext(ctx, q, us.ID, us.UserID, us.ClusterID, us.Namespace, kubeconfigEnc).Scan(&createdAt); err != nil {
		return UserSpace{}, err
	}
	us.CreatedAt = createdAt.Time
	return us, nil
}

func (s *Store) GetUserSpace(ctx context.Context, userID, clusterID string) (UserSpace, []byte, error) {
	const q = `SELECT id, user_id, cluster_id, namespace, created_at, kubeconfig_enc FROM user_spaces WHERE user_id=$1 AND cluster_id=$2`
	var us UserSpace
	var kc []byte
	if err := s.db.QueryRowContext(ctx, q, userID, clusterID).Scan(&us.ID, &us.UserID, &us.ClusterID, &us.Namespace, &us.CreatedAt, &kc); err != nil {
		return UserSpace{}, nil, err
	}
	return us, kc, nil
}

func (s *Store) UpdateUserSpaceKubeconfig(ctx context.Context, id string, kubeconfigEnc []byte) error {
	const q = `UPDATE user_spaces SET kubeconfig_enc=$2 WHERE id=$1`
	_, err := s.db.ExecContext(ctx, q, id, kubeconfigEnc)
	return err
}

func (s *Store) ListUserSpacesByUser(ctx context.Context, userID string) ([]UserSpace, error) {
	const q = `SELECT id, user_id, cluster_id, namespace, created_at, kubeconfig_enc FROM user_spaces WHERE user_id=$1`
	rows, err := s.db.QueryContext(ctx, q, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []UserSpace
	for rows.Next() {
		var us UserSpace
		var kc []byte
		if err := rows.Scan(&us.ID, &us.UserID, &us.ClusterID, &us.Namespace, &us.CreatedAt, &kc); err != nil {
			return nil, err
		}
		out = append(out, us)
	}
	return out, rows.Err()
}
