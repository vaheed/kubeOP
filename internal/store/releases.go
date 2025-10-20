package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

// CreateRelease persists a release record capturing deployment digests and summaries.
func (s *Store) CreateRelease(ctx context.Context, r Release) error {
	if s == nil || s.db == nil {
		return errors.New("store not initialised")
	}
	if strings.TrimSpace(r.ID) == "" {
		return errors.New("release id required")
	}
	if strings.TrimSpace(r.ProjectID) == "" {
		return errors.New("project id required")
	}
	if strings.TrimSpace(r.AppID) == "" {
		return errors.New("app id required")
	}
	if strings.TrimSpace(r.Source) == "" {
		return errors.New("source required")
	}
	if strings.TrimSpace(r.SpecDigest) == "" {
		return errors.New("spec digest required")
	}
	if strings.TrimSpace(r.RenderDigest) == "" {
		return errors.New("render digest required")
	}
	spec := r.Spec
	if spec == nil {
		spec = map[string]any{}
	}
	specJSON, err := json.Marshal(spec)
	if err != nil {
		return fmt.Errorf("encode spec: %w", err)
	}
	rendered := r.RenderedObjects
	if rendered == nil {
		rendered = []map[string]any{}
	}
	renderedJSON, err := json.Marshal(rendered)
	if err != nil {
		return fmt.Errorf("encode rendered objects: %w", err)
	}
	lbs := r.LoadBalancers
	if lbs == nil {
		lbs = map[string]any{}
	}
	lbsJSON, err := json.Marshal(lbs)
	if err != nil {
		return fmt.Errorf("encode load balancers: %w", err)
	}
	warnings := r.Warnings
	if warnings == nil {
		warnings = []string{}
	}
	warningsJSON, err := json.Marshal(warnings)
	if err != nil {
		return fmt.Errorf("encode warnings: %w", err)
	}
	helmValues := r.HelmValues
	if helmValues == nil {
		helmValues = map[string]any{}
	}
	helmValuesJSON, err := json.Marshal(helmValues)
	if err != nil {
		return fmt.Errorf("encode helm values: %w", err)
	}
	status := strings.TrimSpace(r.Status)
	if status == "" {
		status = "succeeded"
	}
	message := r.Message
	var helmChart, helmRenderSHA, manifestsSHA, repo sql.NullString
	if r.HelmChart != nil && strings.TrimSpace(*r.HelmChart) != "" {
		helmChart = sql.NullString{String: strings.TrimSpace(*r.HelmChart), Valid: true}
	}
	if r.HelmRenderSHA != nil && strings.TrimSpace(*r.HelmRenderSHA) != "" {
		helmRenderSHA = sql.NullString{String: strings.TrimSpace(*r.HelmRenderSHA), Valid: true}
	}
	if r.ManifestsSHA != nil && strings.TrimSpace(*r.ManifestsSHA) != "" {
		manifestsSHA = sql.NullString{String: strings.TrimSpace(*r.ManifestsSHA), Valid: true}
	}
	if r.Repo != nil && strings.TrimSpace(*r.Repo) != "" {
		repo = sql.NullString{String: strings.TrimSpace(*r.Repo), Valid: true}
	}
	const q = `INSERT INTO releases (
                id, project_id, app_id, source, spec_digest, render_digest,
                spec, rendered_objects, load_balancers, warnings,
                helm_chart, helm_values, helm_render_sha, manifests_sha, repo,
                status, message)
                VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17)`
	_, err = s.db.ExecContext(ctx, q,
		r.ID,
		r.ProjectID,
		r.AppID,
		r.Source,
		r.SpecDigest,
		r.RenderDigest,
		specJSON,
		renderedJSON,
		lbsJSON,
		warningsJSON,
		helmChart,
		helmValuesJSON,
		helmRenderSHA,
		manifestsSHA,
		repo,
		status,
		message,
	)
	if err != nil {
		return err
	}
	return nil
}

// GetRelease returns a single release record by identifier.
func (s *Store) GetRelease(ctx context.Context, id string) (Release, error) {
	if s == nil || s.db == nil {
		return Release{}, errors.New("store not initialised")
	}
	if strings.TrimSpace(id) == "" {
		return Release{}, errors.New("release id required")
	}
	const q = `SELECT id, project_id, app_id, source, spec_digest, render_digest,
                spec, rendered_objects, load_balancers, warnings,
                helm_chart, helm_values, helm_render_sha, manifests_sha, repo,
                status, message, created_at
                FROM releases WHERE id = $1`
	row := s.db.QueryRowContext(ctx, q, id)
	return scanRelease(row)
}

// ListReleasesByApp returns releases for an app ordered by newest first.
func (s *Store) ListReleasesByApp(ctx context.Context, projectID, appID string, limit int, cursor ReleaseCursor) ([]Release, error) {
	if s == nil || s.db == nil {
		return nil, errors.New("store not initialised")
	}
	if strings.TrimSpace(projectID) == "" {
		return nil, errors.New("project id required")
	}
	if strings.TrimSpace(appID) == "" {
		return nil, errors.New("app id required")
	}
	if limit <= 0 {
		limit = 20
	}
	if limit > 200 {
		limit = 200
	}
	baseSelect := `SELECT id, project_id, app_id, source, spec_digest, render_digest,
                spec, rendered_objects, load_balancers, warnings,
                helm_chart, helm_values, helm_render_sha, manifests_sha, repo,
                status, message, created_at
                FROM releases WHERE project_id = $1 AND app_id = $2`
	var (
		rows *sql.Rows
		err  error
	)
	if strings.TrimSpace(cursor.ID) != "" && !cursor.CreatedAt.IsZero() {
		query := baseSelect + ` AND (created_at < $3 OR (created_at = $3 AND id < $4)) ORDER BY created_at DESC, id DESC LIMIT $5`
		rows, err = s.db.QueryContext(ctx, query, projectID, appID, cursor.CreatedAt, cursor.ID, limit)
	} else {
		query := baseSelect + ` ORDER BY created_at DESC, id DESC LIMIT $3`
		rows, err = s.db.QueryContext(ctx, query, projectID, appID, limit)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var releases []Release
	for rows.Next() {
		rel, err := scanRelease(rows)
		if err != nil {
			return nil, err
		}
		releases = append(releases, rel)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return releases, nil
}

func scanRelease(scanner interface{ Scan(dest ...any) error }) (Release, error) {
	var (
		rel            Release
		specJSON       []byte
		renderedJSON   []byte
		lbsJSON        []byte
		warningsJSON   []byte
		helmValuesJSON []byte
		helmChart      sql.NullString
		helmRenderSHA  sql.NullString
		manifestsSHA   sql.NullString
		repo           sql.NullString
	)
	if err := scanner.Scan(
		&rel.ID,
		&rel.ProjectID,
		&rel.AppID,
		&rel.Source,
		&rel.SpecDigest,
		&rel.RenderDigest,
		&specJSON,
		&renderedJSON,
		&lbsJSON,
		&warningsJSON,
		&helmChart,
		&helmValuesJSON,
		&helmRenderSHA,
		&manifestsSHA,
		&repo,
		&rel.Status,
		&rel.Message,
		&rel.CreatedAt,
	); err != nil {
		return Release{}, err
	}
	if len(specJSON) > 0 {
		if err := json.Unmarshal(specJSON, &rel.Spec); err != nil {
			return Release{}, fmt.Errorf("decode spec: %w", err)
		}
	} else {
		rel.Spec = map[string]any{}
	}
	if len(renderedJSON) > 0 {
		if err := json.Unmarshal(renderedJSON, &rel.RenderedObjects); err != nil {
			return Release{}, fmt.Errorf("decode rendered objects: %w", err)
		}
	} else {
		rel.RenderedObjects = []map[string]any{}
	}
	if len(lbsJSON) > 0 {
		if err := json.Unmarshal(lbsJSON, &rel.LoadBalancers); err != nil {
			return Release{}, fmt.Errorf("decode load balancers: %w", err)
		}
	} else {
		rel.LoadBalancers = map[string]any{}
	}
	if len(warningsJSON) > 0 {
		if err := json.Unmarshal(warningsJSON, &rel.Warnings); err != nil {
			return Release{}, fmt.Errorf("decode warnings: %w", err)
		}
	}
	if len(helmValuesJSON) > 0 {
		if err := json.Unmarshal(helmValuesJSON, &rel.HelmValues); err != nil {
			return Release{}, fmt.Errorf("decode helm values: %w", err)
		}
	} else {
		rel.HelmValues = map[string]any{}
	}
	if helmChart.Valid {
		v := helmChart.String
		rel.HelmChart = &v
	}
	if helmRenderSHA.Valid {
		v := helmRenderSHA.String
		rel.HelmRenderSHA = &v
	}
	if manifestsSHA.Valid {
		v := manifestsSHA.String
		rel.ManifestsSHA = &v
	}
	if repo.Valid {
		v := repo.String
		rel.Repo = &v
	}
	if rel.Warnings == nil {
		rel.Warnings = []string{}
	}
	if rel.Spec == nil {
		rel.Spec = map[string]any{}
	}
	if rel.RenderedObjects == nil {
		rel.RenderedObjects = []map[string]any{}
	}
	if rel.LoadBalancers == nil {
		rel.LoadBalancers = map[string]any{}
	}
	if rel.HelmValues == nil {
		rel.HelmValues = map[string]any{}
	}
	rel.Status = strings.TrimSpace(rel.Status)
	if rel.Status == "" {
		rel.Status = "succeeded"
	}
	rel.Message = strings.TrimSpace(rel.Message)
	return rel, nil
}
