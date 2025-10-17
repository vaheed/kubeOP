package watch

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync/atomic"
	"time"

	"go.uber.org/zap"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/tools/cache"

	"kubeop/internal/logging"
	"kubeop/internal/metrics"
	"kubeop/internal/sink"
	"kubeop/internal/state"
)

const (
	defaultResync          = 10 * time.Minute
	availabilityProbeLimit = 1
	availabilityTimeout    = 5 * time.Second
)

var errKindUnavailable = errors.New("kind unavailable")

// Options configures the watcher manager.
type Options struct {
	Kinds             []Kind
	LabelSelector     string
	RequiredLabels    []string
	NamespacePrefixes []string
	ResyncPeriod      time.Duration
	ClusterID         string
}

// Manager wires Kubernetes informers for the configured kinds and forwards
// normalised events into the sink.
type Manager struct {
	client        dynamic.Interface
	store         *state.Store
	sink          sink.Enqueuer
	labelSelector string
	requiredKeys  []string
	namespacePref []string
	resyncPeriod  time.Duration
	logger        *zap.Logger
	kinds         []Kind
	ready         atomic.Bool
	clusterID     string
}

// NewManager validates inputs and prepares informer plumbing. The caller must
// invoke Start with a context to begin processing.
func NewManager(client dynamic.Interface, store *state.Store, sink sink.Enqueuer, opts Options) (*Manager, error) {
	if client == nil {
		return nil, errors.New("dynamic client is required")
	}
	if store == nil {
		return nil, errors.New("state store is required")
	}
	if sink == nil {
		return nil, errors.New("sink is required")
	}
	if len(opts.Kinds) == 0 {
		return nil, errors.New("no watch kinds configured")
	}
	resync := opts.ResyncPeriod
	if resync <= 0 {
		resync = defaultResync
	}
	required := compactLabelKeys(opts.RequiredLabels)
	prefixes := normalisePrefixes(opts.NamespacePrefixes)
	clusterID := strings.TrimSpace(opts.ClusterID)
	logger := logging.L().With(
		zap.String("component", "watcher"),
		zap.String("cluster_id", clusterID),
	)
	return &Manager{
		client:        client,
		store:         store,
		sink:          sink,
		labelSelector: strings.TrimSpace(opts.LabelSelector),
		requiredKeys:  required,
		namespacePref: prefixes,
		resyncPeriod:  resync,
		logger:        logger,
		kinds:         opts.Kinds,
		clusterID:     clusterID,
	}, nil
}

// Ready reports whether all configured informers have synchronised.
func (m *Manager) Ready() bool {
	return m.ready.Load()
}

// Start initialises informers for the configured kinds and blocks until the
// context is cancelled.
func (m *Manager) Start(ctx context.Context) error {
	if len(m.kinds) == 0 {
		return errors.New("no kinds configured")
	}
	m.ready.Store(false)
	defer m.ready.Store(false)
	informers := make([]cache.SharedIndexInformer, 0, len(m.kinds))
	synced := make([]cache.InformerSynced, 0, len(m.kinds))
	skipped := make([]string, 0)
	for _, kind := range m.kinds {
		informer, err := m.buildInformer(ctx, kind)
		if errors.Is(err, errKindUnavailable) {
			skipped = append(skipped, kind.Name)
			m.logger.Warn(
				"skipping unavailable kind",
				zap.String("kind", kind.Name),
				zap.String("detail", err.Error()),
			)
			continue
		}
		if err != nil {
			return err
		}
		informers = append(informers, informer)
		synced = append(synced, informer.HasSynced)
	}
	if len(informers) == 0 {
		if len(skipped) > 0 {
			m.logger.Warn("no informers started; all kinds unavailable", zap.Strings("kinds", skipped))
		} else {
			m.logger.Warn("no informers started; no kinds configured")
		}
		m.ready.Store(true)
		<-ctx.Done()
		return nil
	}
	for _, inf := range informers {
		go inf.Run(ctx.Done())
	}
	if !cache.WaitForCacheSync(ctx.Done(), synced...) {
		return errors.New("failed to sync informer cache")
	}
	m.ready.Store(true)
	m.logger.Info("informers synchronised", zap.Int("count", len(informers)))
	<-ctx.Done()
	return nil
}

func (m *Manager) buildInformer(ctx context.Context, kind Kind) (cache.SharedIndexInformer, error) {
	resource := m.client.Resource(kind.GVR)
	if err := m.ensureKindAvailable(ctx, kind, resource); err != nil {
		return nil, err
	}
	listWatch := &cache.ListWatch{
		ListFunc: func(options metav1.ListOptions) (runtime.Object, error) {
			return m.list(ctx, kind, resource, options)
		},
		WatchFunc: func(options metav1.ListOptions) (watch.Interface, error) {
			return m.watch(ctx, kind, resource, options)
		},
	}
	informer := cache.NewSharedIndexInformer(listWatch, &unstructured.Unstructured{}, m.resyncPeriod, cache.Indexers{})
	informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    func(obj interface{}) { m.handle(kind, "Added", obj) },
		UpdateFunc: func(_, newObj interface{}) { m.handle(kind, "Modified", newObj) },
		DeleteFunc: func(obj interface{}) { m.handle(kind, "Deleted", obj) },
	})
	return informer, nil
}

func (m *Manager) list(ctx context.Context, kind Kind, resource dynamic.NamespaceableResourceInterface, options metav1.ListOptions) (runtime.Object, error) {
	if m.labelSelector != "" {
		options.LabelSelector = m.labelSelector
	}
	if options.ResourceVersion == "" {
		if rv, err := m.store.GetResourceVersion(kind.Name); err == nil && rv != "" {
			options.ResourceVersion = rv
			options.ResourceVersionMatch = metav1.ResourceVersionMatchNotOlderThan
		} else if err != nil {
			m.logger.Warn("failed to read resource version", zap.String("kind", kind.Name), zap.Error(err))
		}
	}
	options.AllowWatchBookmarks = true
	return resource.List(ctx, options)
}

func (m *Manager) watch(ctx context.Context, kind Kind, resource dynamic.NamespaceableResourceInterface, options metav1.ListOptions) (watch.Interface, error) {
	if m.labelSelector != "" {
		options.LabelSelector = m.labelSelector
	}
	if options.ResourceVersion == "" {
		if rv, err := m.store.GetResourceVersion(kind.Name); err == nil && rv != "" {
			options.ResourceVersion = rv
		} else if err != nil {
			m.logger.Warn("failed to read resource version", zap.String("kind", kind.Name), zap.Error(err))
		}
	}
	options.AllowWatchBookmarks = true
	return resource.Watch(ctx, options)
}

func (m *Manager) ensureKindAvailable(ctx context.Context, kind Kind, resource dynamic.NamespaceableResourceInterface) error {
	probeCtx, cancel := context.WithTimeout(ctx, availabilityTimeout)
	defer cancel()
	if _, err := resource.List(probeCtx, metav1.ListOptions{Limit: availabilityProbeLimit}); err != nil {
		switch {
		case apierrors.IsNotFound(err), apierrors.IsGone(err), apierrors.IsMethodNotSupported(err), meta.IsNoMatchError(err):
			return fmt.Errorf("%w: %v", errKindUnavailable, err)
		case errors.Is(err, context.DeadlineExceeded):
			return fmt.Errorf("%w: availability probe timed out: %v", errKindUnavailable, err)
		default:
			return err
		}
	}
	return nil
}

func (m *Manager) handle(kind Kind, eventType string, obj interface{}) {
	u := extractObject(obj)
	if u == nil {
		metrics.ObserveDrop("decode")
		return
	}
	if !m.matchesNamespace(u.GetNamespace()) {
		metrics.ObserveDrop("namespace_filter")
		return
	}
	labels := u.GetLabels()
	if !m.matchesRequiredLabels(labels) {
		metrics.ObserveDrop("missing_labels")
		return
	}
	uid := strings.TrimSpace(string(u.GetUID()))
	rv := strings.TrimSpace(u.GetResourceVersion())
	if uid == "" || rv == "" {
		metrics.ObserveDrop("missing_resource_version")
		return
	}
	dedupKey := fmt.Sprintf("%s#%s", uid, rv)
	event := sink.Event{
		ClusterID: m.clusterID,
		EventType: eventType,
		Kind:      kind.Name,
		Namespace: u.GetNamespace(),
		Name:      u.GetName(),
		Labels:    sanitiseLabels(labels),
		Summary:   summarise(kind.Name, eventType, u),
		DedupKey:  dedupKey,
	}
	if ok := m.sink.Enqueue(event); !ok {
		return
	}
	if err := m.store.SetResourceVersion(kind.Name, rv); err != nil {
		m.logger.Warn("failed to persist resource version", zap.String("kind", kind.Name), zap.Error(err))
	}
}

func (m *Manager) matchesRequiredLabels(labels map[string]string) bool {
	if len(m.requiredKeys) == 0 {
		return true
	}
	for _, key := range m.requiredKeys {
		if val, ok := labels[key]; !ok || strings.TrimSpace(val) == "" {
			return false
		}
	}
	return true
}

func (m *Manager) matchesNamespace(ns string) bool {
	if len(m.namespacePref) == 0 {
		return true
	}
	trimmed := strings.TrimSpace(ns)
	if trimmed == "" {
		return false
	}
	for _, prefix := range m.namespacePref {
		if strings.HasPrefix(trimmed, prefix) {
			return true
		}
	}
	return false
}

// ProcessObjectForTest injects an object into the watcher pipeline. It is
// intended for unit tests and bypasses informer plumbing.
func (m *Manager) ProcessObjectForTest(kind Kind, eventType string, obj interface{}) {
	m.handle(kind, eventType, obj)
}

func extractObject(obj interface{}) *unstructured.Unstructured {
	if obj == nil {
		return nil
	}
	switch typed := obj.(type) {
	case *unstructured.Unstructured:
		return typed.DeepCopy()
	case cache.DeletedFinalStateUnknown:
		if u, ok := typed.Obj.(*unstructured.Unstructured); ok {
			return u.DeepCopy()
		}
	}
	return nil
}

func sanitiseLabels(labels map[string]string) map[string]string {
	if len(labels) == 0 {
		return nil
	}
	clean := make(map[string]string, len(labels))
	for k, v := range labels {
		clean[k] = v
	}
	return clean
}

func compactLabelKeys(keys []string) []string {
	if len(keys) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(keys))
	result := make([]string, 0, len(keys))
	for _, key := range keys {
		k := strings.TrimSpace(key)
		if k == "" {
			continue
		}
		if strings.ContainsAny(k, "=!") {
			// Only support existence checks for now; more complex selectors are handled server-side.
			continue
		}
		if _, ok := seen[k]; ok {
			continue
		}
		seen[k] = struct{}{}
		result = append(result, k)
	}
	return result
}

func normalisePrefixes(prefixes []string) []string {
	if len(prefixes) == 0 {
		return nil
	}
	result := make([]string, 0, len(prefixes))
	seen := make(map[string]struct{}, len(prefixes))
	for _, prefix := range prefixes {
		p := strings.TrimSpace(prefix)
		if p == "" {
			continue
		}
		key := strings.ToLower(p)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		result = append(result, p)
	}
	return result
}
