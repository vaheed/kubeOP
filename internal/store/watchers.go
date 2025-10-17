package store

import (
	"context"
	"errors"
	"time"
)

// UpsertWatcher creates or updates the watcher record associated with the provided cluster.
func (s *Store) UpsertWatcher(ctx context.Context, clusterID string, refreshHash string, refreshExpires, accessExpires time.Time) (Watcher, error) {
	if s == nil {
		return Watcher{}, errors.New("store not initialised")
	}
	const q = `INSERT INTO watchers (cluster_id, refresh_token_hash, refresh_token_expires_at, access_token_expires_at, last_refresh_at)
VALUES ($1, $2, $3, $4, NOW())
ON CONFLICT (cluster_id) DO UPDATE
SET refresh_token_hash = EXCLUDED.refresh_token_hash,
    refresh_token_expires_at = EXCLUDED.refresh_token_expires_at,
    access_token_expires_at = EXCLUDED.access_token_expires_at,
    last_refresh_at = NOW(),
    updated_at = NOW(),
    disabled = FALSE
RETURNING id, cluster_id, refresh_token_hash, refresh_token_expires_at, access_token_expires_at, last_seen_at, last_refresh_at, created_at, updated_at, disabled`
	var w Watcher
	if err := s.db.QueryRowContext(ctx, q, clusterID, refreshHash, refreshExpires.UTC(), accessExpires.UTC()).Scan(
		&w.ID,
		&w.ClusterID,
		&w.RefreshTokenHash,
		&w.RefreshTokenExpiresAt,
		&w.AccessTokenExpiresAt,
		&w.LastSeenAt,
		&w.LastRefreshAt,
		&w.CreatedAt,
		&w.UpdatedAt,
		&w.Disabled,
	); err != nil {
		return Watcher{}, err
	}
	return w, nil
}

// RotateWatcherTokens updates the stored refresh token hash and expiry for the given watcher.
func (s *Store) RotateWatcherTokens(ctx context.Context, watcherID, refreshHash string, refreshExpires, accessExpires time.Time) (Watcher, error) {
	if s == nil {
		return Watcher{}, errors.New("store not initialised")
	}
	const q = `UPDATE watchers
SET refresh_token_hash = $2,
    refresh_token_expires_at = $3,
    access_token_expires_at = $4,
    last_refresh_at = NOW(),
    updated_at = NOW(),
    disabled = FALSE
WHERE id = $1
RETURNING id, cluster_id, refresh_token_hash, refresh_token_expires_at, access_token_expires_at, last_seen_at, last_refresh_at, created_at, updated_at, disabled`
	var w Watcher
	if err := s.db.QueryRowContext(ctx, q, watcherID, refreshHash, refreshExpires.UTC(), accessExpires.UTC()).Scan(
		&w.ID,
		&w.ClusterID,
		&w.RefreshTokenHash,
		&w.RefreshTokenExpiresAt,
		&w.AccessTokenExpiresAt,
		&w.LastSeenAt,
		&w.LastRefreshAt,
		&w.CreatedAt,
		&w.UpdatedAt,
		&w.Disabled,
	); err != nil {
		return Watcher{}, err
	}
	return w, nil
}

// GetWatcher fetches a watcher by identifier.
func (s *Store) GetWatcher(ctx context.Context, watcherID string) (Watcher, error) {
	if s == nil {
		return Watcher{}, errors.New("store not initialised")
	}
	const q = `SELECT id, cluster_id, refresh_token_hash, refresh_token_expires_at, access_token_expires_at, last_seen_at, last_refresh_at, created_at, updated_at, disabled
FROM watchers WHERE id = $1`
	var w Watcher
	if err := s.db.QueryRowContext(ctx, q, watcherID).Scan(
		&w.ID,
		&w.ClusterID,
		&w.RefreshTokenHash,
		&w.RefreshTokenExpiresAt,
		&w.AccessTokenExpiresAt,
		&w.LastSeenAt,
		&w.LastRefreshAt,
		&w.CreatedAt,
		&w.UpdatedAt,
		&w.Disabled,
	); err != nil {
		return Watcher{}, err
	}
	return w, nil
}

// MarkWatcherSeen updates the last seen timestamp for the watcher.
func (s *Store) MarkWatcherSeen(ctx context.Context, watcherID string, ts time.Time) error {
	if s == nil {
		return errors.New("store not initialised")
	}
	const q = `UPDATE watchers SET last_seen_at = $2, updated_at = NOW() WHERE id = $1`
	if _, err := s.db.ExecContext(ctx, q, watcherID, ts.UTC()); err != nil {
		return err
	}
	return nil
}

// DisableWatcher marks a watcher record as disabled.
func (s *Store) DisableWatcher(ctx context.Context, watcherID string) error {
	if s == nil {
		return errors.New("store not initialised")
	}
	const q = `UPDATE watchers SET disabled = TRUE, updated_at = NOW() WHERE id = $1`
	if _, err := s.db.ExecContext(ctx, q, watcherID); err != nil {
		return err
	}
	return nil
}
