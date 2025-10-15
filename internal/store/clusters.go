package store

import (
	"context"
	"database/sql"
	"strings"
	"time"
)

func (s *Store) CreateCluster(ctx context.Context, c Cluster, kubeconfigEnc []byte) (Cluster, error) {
	const q = `INSERT INTO clusters (id, name, kubeconfig_enc) VALUES ($1, $2, $3) RETURNING created_at, watcher_status, watcher_status_message, watcher_status_updated_at, watcher_ready_at, watcher_health_deadline`
	var statusMessage sql.NullString
	var readyAt sql.NullTime
	if err := s.db.QueryRowContext(ctx, q, c.ID, c.Name, kubeconfigEnc).Scan(&c.CreatedAt, &c.WatcherStatus, &statusMessage, &c.WatcherStatusUpdatedAt, &readyAt, &c.WatcherHealthDeadline); err != nil {
		return Cluster{}, err
	}
	if statusMessage.Valid {
		c.WatcherStatusMessage = &statusMessage.String
	}
	if readyAt.Valid {
		c.WatcherReadyAt = &readyAt.Time
	}
	return c, nil
}

func (s *Store) ListClusters(ctx context.Context) ([]Cluster, error) {
	const q = `SELECT id, name, created_at, watcher_status, watcher_status_message, watcher_status_updated_at, watcher_ready_at, watcher_health_deadline FROM clusters ORDER BY created_at DESC`
	rows, err := s.db.QueryContext(ctx, q)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Cluster
	for rows.Next() {
		var c Cluster
		var statusMessage sql.NullString
		var readyAt sql.NullTime
		if err := rows.Scan(&c.ID, &c.Name, &c.CreatedAt, &c.WatcherStatus, &statusMessage, &c.WatcherStatusUpdatedAt, &readyAt, &c.WatcherHealthDeadline); err != nil {
			return nil, err
		}
		if statusMessage.Valid {
			c.WatcherStatusMessage = &statusMessage.String
		}
		if readyAt.Valid {
			c.WatcherReadyAt = &readyAt.Time
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

// UpdateClusterWatcherStatus updates the watcher status, optional message, and ready timestamp.
func (s *Store) UpdateClusterWatcherStatus(ctx context.Context, id, status string, message *string, readyAt *time.Time) error {
	const q = `UPDATE clusters SET watcher_status = $2, watcher_status_message = $3, watcher_status_updated_at = now(), watcher_ready_at = $4 WHERE id = $1`
	var msg sql.NullString
	if message != nil {
		trimmed := strings.TrimSpace(*message)
		if trimmed != "" {
			msg.Valid = true
			msg.String = trimmed
		}
	}
	var ready sql.NullTime
	if readyAt != nil {
		ready.Valid = true
		ready.Time = readyAt.UTC()
	}
	res, err := s.db.ExecContext(ctx, q, id, status, msg, ready)
	if err != nil {
		return err
	}
	if rows, _ := res.RowsAffected(); rows == 0 {
		return sql.ErrNoRows
	}
	return nil
}
