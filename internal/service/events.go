package service

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	crclient "sigs.k8s.io/controller-runtime/pkg/client"

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
		var project store.Project
		autoProject := false
		if projectID == "" {
			p, err := s.ensureKubectlProject(ctx, res.ClusterID, strings.TrimSpace(evt.Namespace))
			if err != nil {
				logger.Warn("watcher_event_project_ensure_failed",
					zap.String("kind", evt.Kind),
					zap.String("name", evt.Name),
					zap.String("namespace", evt.Namespace),
					zap.Error(err),
				)
				res.Dropped++
				continue
			}
			project = p
			projectID = p.ID
			autoProject = true
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
		if appID == "" && autoProject {
			ensuredAppID, err := s.ensureKubectlApp(ctx, res.ClusterID, project, evt.Kind, strings.TrimSpace(evt.Namespace), strings.TrimSpace(evt.Name))
			if err != nil {
				logger.Warn("watcher_event_app_ensure_failed",
					zap.String("kind", evt.Kind),
					zap.String("name", evt.Name),
					zap.String("namespace", evt.Namespace),
					zap.Error(err),
				)
				res.Dropped++
				continue
			}
			if ensuredAppID == "" {
				res.Dropped++
				continue
			}
			appID = ensuredAppID
		}
		if appID != "" {
			meta["appId"] = appID
		}
		if projectID != "" {
			meta["projectId"] = projectID
		}
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

func (s *Service) ensureKubectlProject(ctx context.Context, clusterID, namespace string) (store.Project, error) {
	if s == nil || s.st == nil {
		return store.Project{}, errors.New("service not initialised")
	}
	ns := strings.TrimSpace(namespace)
	if ns == "" {
		return store.Project{}, errors.New("namespace required")
	}
	if existing, err := s.st.GetProjectByNamespace(ctx, clusterID, ns); err == nil {
		return existing, nil
	} else if !errors.Is(err, sql.ErrNoRows) {
		return store.Project{}, err
	}
	if s.cfg != nil && !s.cfg.ProjectsInUserNamespace {
		return store.Project{}, errors.New("kubectl mirroring requires shared user namespaces")
	}
	userID := parseUserIDFromNamespace(ns)
	if userID == "" {
		return store.Project{}, fmt.Errorf("unable to infer user for namespace %s", ns)
	}
	if _, err := s.st.GetUser(ctx, userID); err != nil {
		return store.Project{}, err
	}
	out, err := s.CreateProject(ctx, ProjectCreateInput{UserID: userID, ClusterID: clusterID, Name: "kubectl"})
	if err != nil {
		if existing, err2 := s.st.GetProjectByNamespace(ctx, clusterID, ns); err2 == nil {
			return existing, nil
		}
		return store.Project{}, err
	}
	return out.Project, nil
}

func (s *Service) ensureKubectlApp(ctx context.Context, clusterID string, project store.Project, kind, namespace, name string) (string, error) {
	ns := strings.TrimSpace(namespace)
	resource := strings.TrimSpace(name)
	if ns == "" || resource == "" {
		return "", errors.New("namespace and name required")
	}
	kindLower := strings.ToLower(strings.TrimSpace(kind))
	switch kindLower {
	case "deployment":
		return s.ensureKubectlDeploymentApp(ctx, clusterID, project, ns, resource)
	case "service":
		return s.ensureKubectlServiceApp(ctx, clusterID, project, ns, resource)
	default:
		ref := kubectlExternalRef(clusterID, ns, "deployment", resource)
		if app, err := s.st.GetAppByExternalRef(ctx, ref); err == nil {
			return app.ID, nil
		} else if errors.Is(err, sql.ErrNoRows) {
			return "", fmt.Errorf("no imported app for %s/%s", kind, resource)
		} else {
			return "", err
		}
	}
}

func (s *Service) ensureKubectlDeploymentApp(ctx context.Context, clusterID string, project store.Project, namespace, name string) (string, error) {
	ref := kubectlExternalRef(clusterID, namespace, "deployment", name)
	if app, err := s.st.GetAppByExternalRef(ctx, ref); err == nil {
		return app.ID, nil
	} else if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return "", err
	}
	if s.km == nil {
		return "", errors.New("kube manager unavailable")
	}
	loader := func(inner context.Context) ([]byte, error) { return s.DecryptClusterKubeconfig(inner, clusterID) }
	client, err := s.km.GetOrCreate(ctx, clusterID, loader)
	if err != nil {
		return "", err
	}
	var dep appsv1.Deployment
	if err := client.Get(ctx, types.NamespacedName{Namespace: namespace, Name: name}, &dep); err != nil {
		return "", err
	}
	appID := uuid.New().String()
	original := dep.DeepCopy()
	changed := ensureObjectLabels(&dep.ObjectMeta, project, appID)
	changed = ensurePodTemplateLabels(&dep.Spec.Template, project, appID) || changed
	defaults := DefaultContainerSecurityContext(s.cfg.PodSecurityLevel)
	changed = applySecurityDefaults(&dep.Spec.Template, defaults) || changed
	if changed {
		if err := client.Patch(ctx, &dep, crclient.MergeFrom(original)); err != nil {
			return "", err
		}
	}
	source := map[string]any{
		"mode":      "kubectl",
		"kind":      "deployment",
		"namespace": namespace,
		"name":      name,
		"kubeName":  name,
	}
	if err := s.st.CreateApp(ctx, appID, project.ID, name, "imported", "", "", ref, source); err != nil {
		return "", err
	}
	if err := s.ensureKubectlServiceLabels(ctx, clusterID, project, namespace, name, appID); err != nil && !errors.Is(err, sql.ErrNoRows) {
		logging.ProjectLogger(project.ID).Warn("kubectl_service_label_failed",
			zap.String("namespace", namespace),
			zap.String("service", name),
			zap.Error(err),
		)
	}
	return appID, nil
}

func (s *Service) ensureKubectlServiceApp(ctx context.Context, clusterID string, project store.Project, namespace, name string) (string, error) {
	ref := kubectlExternalRef(clusterID, namespace, "deployment", name)
	if app, err := s.st.GetAppByExternalRef(ctx, ref); err == nil {
		if err := s.ensureKubectlServiceLabels(ctx, clusterID, project, namespace, name, app.ID); err != nil && !errors.Is(err, sql.ErrNoRows) {
			return "", err
		}
		return app.ID, nil
	} else if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return "", err
	}
	return "", sql.ErrNoRows
}

func (s *Service) ensureKubectlServiceLabels(ctx context.Context, clusterID string, project store.Project, namespace, name, appID string) error {
	if s.km == nil {
		return errors.New("kube manager unavailable")
	}
	loader := func(inner context.Context) ([]byte, error) { return s.DecryptClusterKubeconfig(inner, clusterID) }
	client, err := s.km.GetOrCreate(ctx, clusterID, loader)
	if err != nil {
		return err
	}
	var svc corev1.Service
	if err := client.Get(ctx, types.NamespacedName{Namespace: namespace, Name: name}, &svc); err != nil {
		if apierrors.IsNotFound(err) {
			return sql.ErrNoRows
		}
		return err
	}
	original := svc.DeepCopy()
	if !ensureObjectLabels(&svc.ObjectMeta, project, appID) {
		return nil
	}
	return client.Patch(ctx, &svc, crclient.MergeFrom(original))
}

func ensureObjectLabels(meta *metav1.ObjectMeta, project store.Project, appID string) bool {
	if meta == nil {
		return false
	}
	if meta.Labels == nil {
		meta.Labels = map[string]string{}
	}
	changed := false
	entries := map[string]string{
		"kubeop.project-id": project.ID,
		"kubeop.project.id": project.ID,
		"kubeop.app-id":     appID,
		"kubeop.app.id":     appID,
	}
	if project.UserID != "" {
		entries["kubeop.tenant-id"] = project.UserID
		entries["kubeop.tenant.id"] = project.UserID
	}
	for key, value := range entries {
		if meta.Labels[key] != strings.TrimSpace(value) {
			meta.Labels[key] = strings.TrimSpace(value)
			changed = true
		}
	}
	return changed
}

func ensurePodTemplateLabels(tpl *corev1.PodTemplateSpec, project store.Project, appID string) bool {
	if tpl == nil {
		return false
	}
	if tpl.Labels == nil {
		tpl.Labels = map[string]string{}
	}
	return ensureObjectLabels(&tpl.ObjectMeta, project, appID)
}

func applySecurityDefaults(tpl *corev1.PodTemplateSpec, defaults *corev1.SecurityContext) bool {
	if tpl == nil || defaults == nil {
		return false
	}
	changed := false
	for i := range tpl.Spec.Containers {
		if applyContainerSecurityDefaults(&tpl.Spec.Containers[i], defaults) {
			changed = true
		}
	}
	return changed
}

func applyContainerSecurityDefaults(container *corev1.Container, defaults *corev1.SecurityContext) bool {
	if container == nil || defaults == nil {
		return false
	}
	changed := false
	if container.SecurityContext == nil {
		container.SecurityContext = defaults.DeepCopy()
		return true
	}
	sc := container.SecurityContext
	if defaults.AllowPrivilegeEscalation != nil && sc.AllowPrivilegeEscalation == nil {
		val := *defaults.AllowPrivilegeEscalation
		sc.AllowPrivilegeEscalation = &val
		changed = true
	}
	if defaults.RunAsNonRoot != nil && sc.RunAsNonRoot == nil {
		val := *defaults.RunAsNonRoot
		sc.RunAsNonRoot = &val
		changed = true
	}
	if defaults.ReadOnlyRootFilesystem != nil && sc.ReadOnlyRootFilesystem == nil {
		val := *defaults.ReadOnlyRootFilesystem
		sc.ReadOnlyRootFilesystem = &val
		changed = true
	}
	if defaults.Capabilities != nil {
		if sc.Capabilities == nil {
			sc.Capabilities = defaults.Capabilities.DeepCopy()
			changed = true
		} else {
			for _, drop := range defaults.Capabilities.Drop {
				if !hasCapability(sc.Capabilities.Drop, drop) {
					sc.Capabilities.Drop = append(sc.Capabilities.Drop, drop)
					changed = true
				}
			}
		}
	}
	if defaults.SeccompProfile != nil && sc.SeccompProfile == nil {
		sc.SeccompProfile = defaults.SeccompProfile.DeepCopy()
		changed = true
	}
	return changed
}

func hasCapability(list []corev1.Capability, value corev1.Capability) bool {
	for _, existing := range list {
		if string(existing) == string(value) {
			return true
		}
	}
	return false
}

func kubectlExternalRef(clusterID, namespace, kind, name string) string {
	return fmt.Sprintf("kubectl:%s:%s:%s/%s", strings.TrimSpace(clusterID), strings.TrimSpace(namespace), strings.ToLower(strings.TrimSpace(kind)), strings.TrimSpace(name))
}

func parseUserIDFromNamespace(namespace string) string {
	ns := strings.TrimSpace(namespace)
	const prefix = "user-"
	if !strings.HasPrefix(ns, prefix) {
		return ""
	}
	candidate := strings.TrimSpace(ns[len(prefix):])
	if candidate == "" {
		return ""
	}
	return candidate
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
