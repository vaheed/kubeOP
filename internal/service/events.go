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
	"kubeop/internal/sink"
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

// WatcherIngestResult summarises watcher batch processing.
type WatcherIngestResult struct {
	ClusterID string `json:"clusterId"`
	Total     int    `json:"total"`
	Accepted  int    `json:"accepted"`
	Dropped   int    `json:"dropped"`
}

// ProcessWatcherEvents normalises watcher sink events into project events.
func (s *Service) ProcessWatcherEvents(ctx context.Context, clusterID string, events []sink.Event) (WatcherIngestResult, error) {
	res := WatcherIngestResult{ClusterID: strings.TrimSpace(clusterID), Total: len(events)}
	if strings.TrimSpace(clusterID) == "" {
		return res, errors.New("cluster id required")
	}
	if s == nil || s.st == nil {
		return res, errors.New("service not initialised")
	}
	if len(events) == 0 {
		return res, nil
	}

	logger := s.logger
	if logger == nil {
		logger = logging.L().Named("service")
	}

	projectKeyOptions := []string{"kubeop.project-id", "kubeop.project.id"}
	appKeyOptions := []string{"kubeop.app-id", "kubeop.app.id"}

	for _, evt := range events {
		if strings.TrimSpace(evt.ClusterID) != "" && strings.TrimSpace(evt.ClusterID) != res.ClusterID {
			logger.Warn("watcher_event_cluster_mismatch",
				zap.String("expected_cluster_id", res.ClusterID),
				zap.String("event_cluster_id", strings.TrimSpace(evt.ClusterID)),
				zap.String("kind", evt.Kind),
				zap.String("name", evt.Name),
			)
			res.Dropped++
			continue
		}
		projectID := pickLabel(evt.Labels, projectKeyOptions...)
		if projectID == "" {
			logger.Warn("watcher_event_missing_project", zap.String("kind", evt.Kind), zap.String("name", evt.Name))
			res.Dropped++
			continue
		}
		message := strings.TrimSpace(evt.Summary)
		if message == "" {
			message = fmt.Sprintf("%s %s/%s", strings.ToUpper(strings.TrimSpace(evt.EventType)), strings.TrimSpace(evt.Namespace), strings.TrimSpace(evt.Name))
		}
		kind := buildWatcherEventKind(evt.Kind, evt.EventType)
		severity := severityForWatcherEvent(evt.EventType)
		meta := map[string]any{
			"clusterId": res.ClusterID,
			"eventType": strings.TrimSpace(evt.EventType),
			"namespace": strings.TrimSpace(evt.Namespace),
			"name":      strings.TrimSpace(evt.Name),
			"dedupKey":  strings.TrimSpace(evt.DedupKey),
			"kind":      strings.TrimSpace(evt.Kind),
		}
		if evt.Summary != "" {
			meta["summary"] = evt.Summary
		}
		if len(evt.Labels) > 0 {
			meta["labels"] = copyLabels(evt.Labels)
		}
		appID := pickLabel(evt.Labels, appKeyOptions...)
		if _, err := s.AppendProjectEvent(ctx, EventInput{
			ProjectID: projectID,
			AppID:     appID,
			Kind:      kind,
			Severity:  severity,
			Message:   message,
			Meta:      meta,
		}); err != nil {
			logger.Error("watcher_event_append_failed",
				zap.String("project_id", projectID),
				zap.String("kind", kind),
				zap.Error(err),
			)
			res.Dropped++
			continue
		}
		res.Accepted++
	}

	logger.Info("watcher_events_ingested",
		zap.String("cluster_id", res.ClusterID),
		zap.Int("accepted", res.Accepted),
		zap.Int("dropped", res.Dropped),
		zap.Int("total", res.Total),
	)
	return res, nil
}

func buildWatcherEventKind(kind, eventType string) string {
	base := strings.ToUpper(strings.TrimSpace(kind))
	if base == "" {
		base = "RESOURCE"
	}
	etype := strings.ToUpper(strings.TrimSpace(eventType))
	if etype == "" {
		etype = "UNKNOWN"
	}
	return "K8S_" + base + "_" + etype
}

func severityForWatcherEvent(eventType string) string {
	switch strings.ToUpper(strings.TrimSpace(eventType)) {
	case "DELETED":
		return SeverityWarn
	case "ERROR":
		return SeverityError
	default:
		return SeverityInfo
	}
}

func pickLabel(labels map[string]string, keys ...string) string {
	for _, key := range keys {
		if val, ok := labels[key]; ok {
			trimmed := strings.TrimSpace(val)
			if trimmed != "" {
				return trimmed
			}
		}
	}
	return ""
}

func copyLabels(labels map[string]string) map[string]string {
	if len(labels) == 0 {
		return nil
	}
	cloned := make(map[string]string, len(labels))
	for k, v := range labels {
		cloned[k] = v
	}
	return cloned
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
