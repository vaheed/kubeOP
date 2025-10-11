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

