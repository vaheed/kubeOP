package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

// UpsertAppTemplate stores or updates the template metadata bound to an app.
func (s *Store) UpsertAppTemplate(ctx context.Context, appID, templateID string, values, metadata map[string]any) (AppTemplate, error) {
	if s == nil || s.db == nil {
		return AppTemplate{}, errors.New("store not initialised")
	}
	if strings.TrimSpace(appID) == "" || strings.TrimSpace(templateID) == "" {
		return AppTemplate{}, errors.New("appID and templateID are required")
	}
	valuesJSON, err := json.Marshal(values)
	if err != nil {
		return AppTemplate{}, fmt.Errorf("encode values: %w", err)
	}
	metaJSON, err := json.Marshal(metadata)
	if err != nil {
		return AppTemplate{}, fmt.Errorf("encode metadata: %w", err)
	}
	const q = `INSERT INTO app_templates (app_id, template_id, values, metadata)
VALUES ($1,$2,$3,$4)
ON CONFLICT (app_id)
DO UPDATE SET template_id = EXCLUDED.template_id, values = EXCLUDED.values, metadata = EXCLUDED.metadata, updated_at = now()
RETURNING id, app_id, template_id, values, metadata, created_at, updated_at`
	var (
		tpl       AppTemplate
		valuesRaw []byte
		metaRaw   []byte
	)
	if err := s.db.QueryRowContext(ctx, q, appID, templateID, valuesJSON, metaJSON).Scan(
		&tpl.ID,
		&tpl.AppID,
		&tpl.TemplateID,
		&valuesRaw,
		&metaRaw,
		&tpl.CreatedAt,
		&tpl.UpdatedAt,
	); err != nil {
		return AppTemplate{}, err
	}
	_ = json.Unmarshal(valuesRaw, &tpl.Values)
	_ = json.Unmarshal(metaRaw, &tpl.Metadata)
	return tpl, nil
}

// GetAppTemplate retrieves template metadata for an app if present.
func (s *Store) GetAppTemplate(ctx context.Context, appID string) (AppTemplate, error) {
	if s == nil || s.db == nil {
		return AppTemplate{}, errors.New("store not initialised")
	}
	if strings.TrimSpace(appID) == "" {
		return AppTemplate{}, errors.New("appID is required")
	}
	const q = `SELECT id, app_id, template_id, values, metadata, created_at, updated_at FROM app_templates WHERE app_id = $1`
	var (
		tpl       AppTemplate
		valuesRaw []byte
		metaRaw   []byte
	)
	if err := s.db.QueryRowContext(ctx, q, appID).Scan(
		&tpl.ID,
		&tpl.AppID,
		&tpl.TemplateID,
		&valuesRaw,
		&metaRaw,
		&tpl.CreatedAt,
		&tpl.UpdatedAt,
	); err != nil {
		return AppTemplate{}, err
	}
	_ = json.Unmarshal(valuesRaw, &tpl.Values)
	_ = json.Unmarshal(metaRaw, &tpl.Metadata)
	return tpl, nil
}

// DeleteAppTemplate removes template metadata for an app.
func (s *Store) DeleteAppTemplate(ctx context.Context, appID string) error {
	if s == nil || s.db == nil {
		return errors.New("store not initialised")
	}
	if strings.TrimSpace(appID) == "" {
		return errors.New("appID is required")
	}
	const q = `DELETE FROM app_templates WHERE app_id = $1`
	_, err := s.db.ExecContext(ctx, q, appID)
	return err
}
