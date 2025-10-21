package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"kubeop/internal/logging"
	"kubeop/internal/store"
)

var (
	// ErrEventBridgeDisabled indicates the event ingestion endpoint is disabled by configuration.
	ErrEventBridgeDisabled = errors.New("event bridge disabled")
)

const maxEventIngestBatch = 500

// EventIngestError reports why an individual event in the ingest payload was skipped.
type EventIngestError struct {
	Index int    `json:"index"`
	Error string `json:"error"`
}

// EventIngestSummary summarises a bridge upload.
type EventIngestSummary struct {
	ClusterID string             `json:"clusterId,omitempty"`
	Total     int                `json:"total"`
	Accepted  int                `json:"accepted"`
	Dropped   int                `json:"dropped"`
	Status    string             `json:"status,omitempty"`
	Errors    []EventIngestError `json:"errors,omitempty"`
}

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

// IngestProjectEvents stores a batch of project events originating from an external
// bridge (for example, watchers tailing Kubernetes events). The method accepts
// best-effort uploads and drops invalid records while returning a summary.
func (s *Service) IngestProjectEvents(ctx context.Context, clusterID string, events []EventInput) (EventIngestSummary, error) {
	summary := EventIngestSummary{
		ClusterID: strings.TrimSpace(clusterID),
		Total:     len(events),
	}
	if s == nil || s.st == nil || s.cfg == nil {
		return summary, errors.New("service not initialised")
	}
	logger := s.logger
	if logger == nil {
		logger = zap.NewNop()
	}
	if !s.cfg.EventsBridgeEnabled {
		summary.Status = "ignored"
		summary.Dropped = summary.Total
		logger.Info("event_bridge_disabled",
			zap.String("cluster_id", summary.ClusterID),
			zap.Int("total", summary.Total),
		)
		return summary, ErrEventBridgeDisabled
	}
	if len(events) == 0 {
		return summary, nil
	}
	if summary.Total > maxEventIngestBatch {
		dropped := summary.Total - maxEventIngestBatch
		summary.Dropped += dropped
		summary.Errors = append(summary.Errors, EventIngestError{
			Index: -1,
			Error: fmt.Sprintf("batch exceeds limit of %d; dropping %d events", maxEventIngestBatch, dropped),
		})
		logger.Warn("event_ingest_batch_truncated",
			zap.Int("total", summary.Total),
			zap.Int("accepted_limit", maxEventIngestBatch),
			zap.Int("dropped", dropped),
		)
		events = events[:maxEventIngestBatch]
		summary.Total = len(events) + summary.Dropped
	}
	for idx, evt := range events {
		evt.ProjectID = strings.TrimSpace(evt.ProjectID)
		evt.AppID = strings.TrimSpace(evt.AppID)
		evt.ActorUserID = strings.TrimSpace(evt.ActorUserID)
		evt.Kind = strings.TrimSpace(evt.Kind)
		evt.Severity = strings.TrimSpace(evt.Severity)
		evt.Message = strings.TrimSpace(evt.Message)
		if evt.ProjectID == "" || evt.Kind == "" || evt.Message == "" {
			summary.Dropped++
			summary.Errors = append(summary.Errors, EventIngestError{
				Index: idx,
				Error: "projectId, kind, and message are required",
			})
			logger.Warn("event_ingest_skipped_missing_fields",
				zap.Int("index", idx),
				zap.String("project_id", evt.ProjectID),
			)
			continue
		}
		if _, err := s.AppendProjectEvent(ctx, evt); err != nil {
			summary.Dropped++
			summary.Errors = append(summary.Errors, EventIngestError{
				Index: idx,
				Error: err.Error(),
			})
			logger.Warn("event_ingest_append_failed",
				zap.Int("index", idx),
				zap.String("project_id", evt.ProjectID),
				zap.Error(err),
			)
			continue
		}
		summary.Accepted++
	}
	return summary, nil
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
