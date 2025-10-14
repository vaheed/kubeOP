package watch

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync/atomic"
	"time"

	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/apis/meta/v1"
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

const defaultResync = 10 * time.Minute

// Options configures the watcher manager.
type Options struct {
	Kinds          []Kind
	LabelSelector  string
	RequiredLabels []string
	ResyncPeriod   time.Duration
	ClusterID      string
}

// Manager wires Kubernetes informers for the configured kinds and forwards
// normalised events into the sink.
type Manager struct {
	client        dynamic.Interface
	store         *state.Store
	sink          sink.Enqueuer
	labelSelector string
	requiredKeys  []string
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
	for _, kind := range m.kinds {
		informer, err := m.buildInformer(ctx, kind)
		if err != nil {
			return err
		}
		informers = append(informers, informer)
		synced = append(synced, informer.HasSynced)
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
	listWatch := &cache.ListWatch{
		ListFunc: func(options v1.ListOptions) (runtime.Object, error) {
			return m.list(ctx, kind, resource, options)
		},
		WatchFunc: func(options v1.ListOptions) (watch.Interface, error) {
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

func (m *Manager) list(ctx context.Context, kind Kind, resource dynamic.NamespaceableResourceInterface, options v1.ListOptions) (runtime.Object, error) {
	if m.labelSelector != "" {
		options.LabelSelector = m.labelSelector
	}
	if options.ResourceVersion == "" {
		if rv, err := m.store.GetResourceVersion(kind.Name); err == nil && rv != "" {
			options.ResourceVersion = rv
			options.ResourceVersionMatch = v1.ResourceVersionMatchNotOlderThan
		} else if err != nil {
			m.logger.Warn("failed to read resource version", zap.String("kind", kind.Name), zap.Error(err))
		}
	}
	options.AllowWatchBookmarks = true
	return resource.List(ctx, options)
}

func (m *Manager) watch(ctx context.Context, kind Kind, resource dynamic.NamespaceableResourceInterface, options v1.ListOptions) (watch.Interface, error) {
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

func (m *Manager) handle(kind Kind, eventType string, obj interface{}) {
	u := extractObject(obj)
	if u == nil {
		metrics.ObserveDrop("decode")
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
