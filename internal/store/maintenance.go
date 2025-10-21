package store

import (
	"context"
	"database/sql"
)

const maintenanceRowID = "global"

// GetMaintenanceState returns the persisted global maintenance toggle state.
func (s *Store) GetMaintenanceState(ctx context.Context) (MaintenanceState, error) {
	if err := s.ensureMaintenanceRow(ctx); err != nil {
		return MaintenanceState{}, err
	}
	const query = `SELECT id, enabled, message, updated_at, updated_by FROM maintenance_state WHERE id = $1`
	row := s.db.QueryRowContext(ctx, query, maintenanceRowID)
	var state MaintenanceState
	if err := row.Scan(&state.ID, &state.Enabled, &state.Message, &state.UpdatedAt, &state.UpdatedBy); err != nil {
		return MaintenanceState{}, err
	}
	return state, nil
}

// UpdateMaintenanceState flips the maintenance toggle and records the actor/message.
func (s *Store) UpdateMaintenanceState(ctx context.Context, enabled bool, message, actor string) (MaintenanceState, error) {
	if err := s.ensureMaintenanceRow(ctx); err != nil {
		return MaintenanceState{}, err
	}
	const query = `UPDATE maintenance_state SET enabled = $1, message = $2, updated_at = NOW(), updated_by = $3 WHERE id = $4 RETURNING id, enabled, message, updated_at, updated_by`
	row := s.db.QueryRowContext(ctx, query, enabled, message, actor, maintenanceRowID)
	var state MaintenanceState
	if err := row.Scan(&state.ID, &state.Enabled, &state.Message, &state.UpdatedAt, &state.UpdatedBy); err != nil {
		if err == sql.ErrNoRows {
			if err := s.ensureMaintenanceRow(ctx); err != nil {
				return MaintenanceState{}, err
			}
			return s.UpdateMaintenanceState(ctx, enabled, message, actor)
		}
		return MaintenanceState{}, err
	}
	return state, nil
}

func (s *Store) ensureMaintenanceRow(ctx context.Context) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO maintenance_state (id, enabled, message, updated_by) VALUES ($1, FALSE, '', '') ON CONFLICT (id) DO NOTHING`, maintenanceRowID)
	return err
}
