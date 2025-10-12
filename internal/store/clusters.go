package store

import (
	"context"
)

func (s *Store) CreateCluster(ctx context.Context, c Cluster, kubeconfigEnc []byte) (Cluster, error) {
	const q = `INSERT INTO clusters (id, name, kubeconfig_enc) VALUES ($1, $2, $3) RETURNING created_at`
	if err := s.db.QueryRowContext(ctx, q, c.ID, c.Name, kubeconfigEnc).Scan(&c.CreatedAt); err != nil {
		return Cluster{}, err
	}
	return c, nil
}

func (s *Store) ListClusters(ctx context.Context) ([]Cluster, error) {
	const q = `SELECT id, name, created_at FROM clusters ORDER BY created_at DESC`
	rows, err := s.db.QueryContext(ctx, q)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Cluster
	for rows.Next() {
		var c Cluster
		if err := rows.Scan(&c.ID, &c.Name, &c.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func (s *Store) GetClusterKubeconfigEnc(ctx context.Context, id string) ([]byte, error) {
	const q = `SELECT kubeconfig_enc FROM clusters WHERE id = $1`
	var b []byte
	if err := s.db.QueryRowContext(ctx, q, id).Scan(&b); err != nil {
		return nil, err
	}
	return b, nil
}
