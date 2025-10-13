package service

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"
	"kubeop/internal/logging"
	"kubeop/internal/store"
)

const (
	SeverityInfo  = "INFO"
	SeverityWarn  = "WARN"
	SeverityError = "ERROR"
)

type EventInput struct {
	ProjectID   string
	AppID       string
	ActorUserID string
	Kind        string
	Severity    string
	Message     string
	Meta        map[string]any
}

func (s *Service) AppendProjectEvent(ctx context.Context, in EventInput) (store.ProjectEvent, error) {
	if s == nil || s.st == nil {
		return store.ProjectEvent{}, errors.New("service not initialised")
	}
	projectID := strings.TrimSpace(in.ProjectID)
	if projectID == "" {
		return store.ProjectEvent{}, errors.New("project id required")
	}
	kind := strings.TrimSpace(in.Kind)
	if kind == "" {
		return store.ProjectEvent{}, errors.New("kind required")
	}
	message := strings.TrimSpace(in.Message)
	if message == "" {
		return store.ProjectEvent{}, errors.New("message required")
	}
	severity := strings.ToUpper(strings.TrimSpace(in.Severity))
	if severity == "" {
		severity = SeverityInfo
	}
	actor := strings.TrimSpace(in.ActorUserID)
	if actor == "" {
		actor = actorFromContext(ctx)
	}
	appID := strings.TrimSpace(in.AppID)
	meta := cloneMeta(in.Meta)

	evt := store.ProjectEvent{
		ID:          uuid.New().String(),
		ProjectID:   projectID,
		AppID:       appID,
		ActorUserID: actor,
		Kind:        kind,
		Severity:    severity,
		Message:     message,
		Meta:        meta,
	}
	eventsEnabled := true
	if s.cfg != nil {
		eventsEnabled = s.cfg.EventsDBEnabled
	}
	if !eventsEnabled {
		evt.At = time.Now().UTC()
		logProjectEvent(evt)
		evt.Meta = redactMeta(evt.Meta)
		return evt, nil
	}
	stored, err := s.st.InsertProjectEvent(ctx, evt)
	if err != nil {
		logging.L().Error("append_project_event_failed",
			zap.String("project_id", projectID),
			zap.String("kind", kind),
			zap.String("severity", severity),
			zap.Error(err),
		)
		return store.ProjectEvent{}, err
	}
	logProjectEvent(stored)
	stored.Meta = redactMeta(stored.Meta)
	return stored, nil
}

func (s *Service) ListProjectEvents(ctx context.Context, projectID string, filter store.ProjectEventFilter) (store.ProjectEventPage, error) {
	if s == nil || s.st == nil {
		return store.ProjectEventPage{}, errors.New("service not initialised")
	}
	eventsEnabled := true
	if s.cfg != nil {
		eventsEnabled = s.cfg.EventsDBEnabled
	}
	if !eventsEnabled {
		return store.ProjectEventPage{}, errors.New("events db disabled")
	}
	page, err := s.st.ListProjectEvents(ctx, projectID, filter)
	if err != nil {
		return store.ProjectEventPage{}, err
	}
	for i := range page.Events {
		page.Events[i].Severity = strings.ToUpper(strings.TrimSpace(page.Events[i].Severity))
		page.Events[i].Kind = strings.TrimSpace(page.Events[i].Kind)
		page.Events[i].Meta = redactMeta(page.Events[i].Meta)
	}
	return page, nil
}

func logProjectEvent(evt store.ProjectEvent) {
	fields := []zap.Field{
		zap.String("event_id", evt.ID),
		zap.String("severity", evt.Severity),
		zap.String("message", evt.Message),
		zap.Time("at", evt.At),
	}
	if evt.AppID != "" {
		fields = append(fields, zap.String("app_id", evt.AppID))
	}
	if evt.ActorUserID != "" {
		fields = append(fields, zap.String("actor_user_id", evt.ActorUserID))
	}
	if len(evt.Meta) > 0 {
		fields = append(fields, zap.Any("meta", redactMeta(evt.Meta)))
	}
	logging.ProjectEventsLogger(evt.ProjectID).Info(evt.Kind, fields...)
}

func cloneMeta(meta map[string]any) map[string]any {
	if len(meta) == 0 {
		return nil
	}
	out := make(map[string]any, len(meta))
	for k, v := range meta {
		out[k] = v
	}
	return out
}

func redactMeta(meta map[string]any) map[string]any {
	if len(meta) == 0 {
		return nil
	}
	out := make(map[string]any, len(meta))
	for k, v := range meta {
		if shouldRedactKey(k) {
			out[k] = "[redacted]"
			continue
		}
		if nested, ok := v.(map[string]any); ok {
			out[k] = redactMeta(nested)
			continue
		}
		out[k] = v
	}
	return out
}

func shouldRedactKey(key string) bool {
	lowered := strings.ToLower(key)
	return strings.Contains(lowered, "secret") || strings.Contains(lowered, "token") || strings.Contains(lowered, "password")
}
