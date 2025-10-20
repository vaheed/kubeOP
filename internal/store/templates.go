package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
)

// CreateTemplate persists a new template definition including schema metadata and the
// delivery template used for instantiation.
func (s *Store) CreateTemplate(ctx context.Context, t Template) error {
	const q = `INSERT INTO templates (
                id, name, kind, description, schema, defaults, example, base, delivery_template
        ) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)`

	schemaJSON, err := json.Marshal(t.Schema)
	if err != nil {
		return err
	}
	defaultsJSON, err := json.Marshal(t.Defaults)
	if err != nil {
		return err
	}
	exampleJSON, err := json.Marshal(t.Example)
	if err != nil {
		return err
	}
	baseJSON, err := json.Marshal(t.Base)
	if err != nil {
		return err
	}
	_, err = s.db.ExecContext(
		ctx,
		q,
		t.ID,
		t.Name,
		t.Kind,
		t.Description,
		schemaJSON,
		defaultsJSON,
		exampleJSON,
		baseJSON,
		t.DeliveryTemplate,
	)
	return err
}

// ListTemplates returns templates ordered by creation time descending to surface the
// most recently published blueprints first.
func (s *Store) ListTemplates(ctx context.Context) ([]Template, error) {
	const q = `SELECT id, name, kind, description, schema, defaults, example, base, delivery_template, created_at
                FROM templates
                ORDER BY created_at DESC`
	rows, err := s.db.QueryContext(ctx, q)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var templates []Template
	for rows.Next() {
		tpl, err := scanTemplate(rows)
		if err != nil {
			return nil, err
		}
		templates = append(templates, tpl)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return templates, nil
}

// GetTemplate loads a template definition by identifier.
func (s *Store) GetTemplate(ctx context.Context, id string) (Template, error) {
	const q = `SELECT id, name, kind, description, schema, defaults, example, base, delivery_template, created_at
                FROM templates WHERE id = $1`
	row := s.db.QueryRowContext(ctx, q, id)
	tpl, err := scanTemplate(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Template{}, err
		}
		return Template{}, err
	}
	return tpl, nil
}

func scanTemplate(scanner interface {
	Scan(dest ...any) error
}) (Template, error) {
	var (
		tpl          Template
		schemaJSON   []byte
		defaultsJSON []byte
		exampleJSON  []byte
		baseJSON     []byte
	)
	if err := scanner.Scan(
		&tpl.ID,
		&tpl.Name,
		&tpl.Kind,
		&tpl.Description,
		&schemaJSON,
		&defaultsJSON,
		&exampleJSON,
		&baseJSON,
		&tpl.DeliveryTemplate,
		&tpl.CreatedAt,
	); err != nil {
		return Template{}, err
	}
	if len(schemaJSON) > 0 {
		if err := json.Unmarshal(schemaJSON, &tpl.Schema); err != nil {
			return Template{}, err
		}
	}
	if tpl.Schema == nil {
		tpl.Schema = map[string]any{}
	}
	if len(defaultsJSON) > 0 {
		if err := json.Unmarshal(defaultsJSON, &tpl.Defaults); err != nil {
			return Template{}, err
		}
	}
	if tpl.Defaults == nil {
		tpl.Defaults = map[string]any{}
	}
	if len(exampleJSON) > 0 {
		if err := json.Unmarshal(exampleJSON, &tpl.Example); err != nil {
			return Template{}, err
		}
	}
	if len(baseJSON) > 0 {
		if err := json.Unmarshal(baseJSON, &tpl.Base); err != nil {
			return Template{}, err
		}
	}
	if tpl.Base == nil {
		tpl.Base = map[string]any{}
	}
	return tpl, nil
}
