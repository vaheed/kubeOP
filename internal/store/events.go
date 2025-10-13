package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

type ProjectEvent struct {
	ID          string         `json:"id"`
	ProjectID   string         `json:"projectId"`
	AppID       string         `json:"appId,omitempty"`
	ActorUserID string         `json:"actorUserId,omitempty"`
	Kind        string         `json:"kind"`
	Severity    string         `json:"severity"`
	Message     string         `json:"message"`
	Meta        map[string]any `json:"meta,omitempty"`
	At          time.Time      `json:"at"`
}

type ProjectEventFilter struct {
	Kinds       []string
	Severities  []string
	ActorUserID string
	Since       time.Time
	Limit       int
	Cursor      string
	Search      string
}

type ProjectEventPage struct {
	Events     []ProjectEvent `json:"events"`
	NextCursor string         `json:"nextCursor,omitempty"`
}

// InsertProjectEvent persists a project event row and returns the stored record.
func (s *Store) InsertProjectEvent(ctx context.Context, evt ProjectEvent) (ProjectEvent, error) {
	if s == nil || s.db == nil {
		return ProjectEvent{}, errors.New("store not initialised")
	}
	if strings.TrimSpace(evt.ID) == "" {
		return ProjectEvent{}, errors.New("event id required")
	}
	if strings.TrimSpace(evt.ProjectID) == "" {
		return ProjectEvent{}, errors.New("project id required")
	}
	if strings.TrimSpace(evt.Kind) == "" {
		return ProjectEvent{}, errors.New("kind required")
	}
	if strings.TrimSpace(evt.Severity) == "" {
		return ProjectEvent{}, errors.New("severity required")
	}
	if strings.TrimSpace(evt.Message) == "" {
		return ProjectEvent{}, errors.New("message required")
	}
	metaJSON, err := json.Marshal(evt.Meta)
	if err != nil {
		return ProjectEvent{}, fmt.Errorf("encode meta: %w", err)
	}
	if string(metaJSON) == "null" {
		metaJSON = nil
	}
	var appID, actor sql.NullString
	if strings.TrimSpace(evt.AppID) != "" {
		appID = sql.NullString{String: evt.AppID, Valid: true}
	}
	if strings.TrimSpace(evt.ActorUserID) != "" {
		actor = sql.NullString{String: evt.ActorUserID, Valid: true}
	}
	const q = `INSERT INTO project_events (id, project_id, app_id, actor_user_id, kind, severity, message, meta)
                   VALUES ($1,$2,$3,$4,$5,$6,$7,$8)
                   RETURNING at`
	if err := s.db.QueryRowContext(ctx, q, evt.ID, evt.ProjectID, appID, actor, evt.Kind, evt.Severity, evt.Message, metaJSON).Scan(&evt.At); err != nil {
		return ProjectEvent{}, err
	}
	return evt, nil
}

// ListProjectEvents returns a page of events filtered by the provided constraints.
func (s *Store) ListProjectEvents(ctx context.Context, projectID string, filter ProjectEventFilter) (ProjectEventPage, error) {
	if s == nil || s.db == nil {
		return ProjectEventPage{}, errors.New("store not initialised")
	}
	if strings.TrimSpace(projectID) == "" {
		return ProjectEventPage{}, errors.New("project id required")
	}
	limit := filter.Limit
	if limit <= 0 {
		limit = 50
	}
	if limit > 500 {
		limit = 500
	}
	builder := strings.Builder{}
	builder.WriteString("SELECT id, project_id, COALESCE(app_id::text, '') AS app_id, COALESCE(actor_user_id, '') AS actor_user_id, kind, severity, message, meta, at FROM project_events WHERE project_id = $1")
	args := []any{projectID}
	if len(filter.Kinds) > 0 {
		placeholders := make([]string, 0, len(filter.Kinds))
		for _, kind := range filter.Kinds {
			if strings.TrimSpace(kind) == "" {
				continue
			}
			args = append(args, kind)
			placeholders = append(placeholders, fmt.Sprintf("$%d", len(args)))
		}
		if len(placeholders) > 0 {
			builder.WriteString(" AND kind IN (" + strings.Join(placeholders, ",") + ")")
		}
	}
	if len(filter.Severities) > 0 {
		placeholders := make([]string, 0, len(filter.Severities))
		for _, sev := range filter.Severities {
			if strings.TrimSpace(sev) == "" {
				continue
			}
			args = append(args, sev)
			placeholders = append(placeholders, fmt.Sprintf("$%d", len(args)))
		}
		if len(placeholders) > 0 {
			builder.WriteString(" AND severity IN (" + strings.Join(placeholders, ",") + ")")
		}
	}
	if strings.TrimSpace(filter.ActorUserID) != "" {
		args = append(args, filter.ActorUserID)
		builder.WriteString(" AND actor_user_id = $" + fmt.Sprint(len(args)))
	}
	if !filter.Since.IsZero() {
		args = append(args, filter.Since)
		builder.WriteString(" AND at >= $" + fmt.Sprint(len(args)))
	}
	if strings.TrimSpace(filter.Cursor) != "" {
		if at, id, err := decodeEventCursor(filter.Cursor); err == nil {
			args = append(args, at)
			atPlaceholder := fmt.Sprintf("$%d", len(args))
			args = append(args, id)
			idPlaceholder := fmt.Sprintf("$%d", len(args))
			builder.WriteString(fmt.Sprintf(" AND (at < %s OR (at = %s AND id < %s))", atPlaceholder, atPlaceholder, idPlaceholder))
		}
	}
	if strings.TrimSpace(filter.Search) != "" {
		term := "%" + strings.ToLower(strings.TrimSpace(filter.Search)) + "%"
		args = append(args, term)
		termPlaceholder := fmt.Sprintf("$%d", len(args))
		args = append(args, term)
		term2Placeholder := fmt.Sprintf("$%d", len(args))
		builder.WriteString(fmt.Sprintf(" AND (LOWER(message) LIKE %s OR LOWER(kind) LIKE %s)", termPlaceholder, term2Placeholder))
	}
	builder.WriteString(" ORDER BY at DESC, id DESC")
	args = append(args, limit+1)
	builder.WriteString(" LIMIT $" + fmt.Sprint(len(args)))

	rows, err := s.db.QueryContext(ctx, builder.String(), args...)
	if err != nil {
		return ProjectEventPage{}, err
	}
	defer rows.Close()

	page := ProjectEventPage{}
	for rows.Next() {
		var evt ProjectEvent
		var metaBytes []byte
		if err := rows.Scan(&evt.ID, &evt.ProjectID, &evt.AppID, &evt.ActorUserID, &evt.Kind, &evt.Severity, &evt.Message, &metaBytes, &evt.At); err != nil {
			return ProjectEventPage{}, err
		}
		if len(metaBytes) > 0 {
			var meta map[string]any
			if err := json.Unmarshal(metaBytes, &meta); err == nil {
				evt.Meta = meta
			}
		}
		page.Events = append(page.Events, evt)
	}
	if err := rows.Err(); err != nil {
		return ProjectEventPage{}, err
	}
	if len(page.Events) > limit {
		last := page.Events[limit]
		page.Events = page.Events[:limit]
		page.NextCursor = encodeEventCursor(last.At, last.ID)
	}
	return page, nil
}

func encodeEventCursor(at time.Time, id string) string {
	if id == "" {
		return ""
	}
	return at.UTC().Format(time.RFC3339Nano) + "|" + id
}

func decodeEventCursor(raw string) (time.Time, string, error) {
	parts := strings.Split(raw, "|")
	if len(parts) != 2 {
		return time.Time{}, "", fmt.Errorf("invalid cursor")
	}
	at, err := time.Parse(time.RFC3339Nano, parts[0])
	if err != nil {
		return time.Time{}, "", err
	}
	id := strings.TrimSpace(parts[1])
	if id == "" {
		return time.Time{}, "", fmt.Errorf("invalid cursor id")
	}
	return at, id, nil
}
