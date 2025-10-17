package watch_test

import (
	"context"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic/fake"
	k8stesting "k8s.io/client-go/testing"

	sinkpkg "kubeop/internal/sink"
	statepkg "kubeop/internal/state"
	watchpkg "kubeop/internal/watch"
)

type capturingSink struct {
	mu     sync.Mutex
	events []sinkpkg.Event
}

func (c *capturingSink) Enqueue(event sinkpkg.Event) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.events = append(c.events, event)
	return true
}

func (c *capturingSink) Events() []sinkpkg.Event {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make([]sinkpkg.Event, len(c.events))
	copy(out, c.events)
	return out
}

func TestManagerHandlePersistsEvents(t *testing.T) {
	scheme := runtime.NewScheme()
	client := fake.NewSimpleDynamicClient(scheme)
	storePath := filepath.Join(t.TempDir(), "state.db")
	store, err := statepkg.Open(storePath)
	if err != nil {
		t.Fatalf("open state store: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

	sink := &capturingSink{}
	kind := watchpkg.Kind{Name: "Pod", GVR: schema.GroupVersionResource{Group: "", Version: "v1", Resource: "pods"}}
	manager, err := watchpkg.NewManager(client, store, sink, watchpkg.Options{
		Kinds:             []watchpkg.Kind{kind},
		LabelSelector:     "kubeop.project.id",
		RequiredLabels:    []string{"kubeop.project.id"},
		NamespacePrefixes: []string{"user-"},
		ClusterID:         "cluster-123",
	})
	if err != nil {
		t.Fatalf("new manager: %v", err)
	}

	obj := &unstructured.Unstructured{}
	obj.SetAPIVersion("v1")
	obj.SetKind("Pod")
	obj.SetNamespace("user-example")
	obj.SetName("web-1")
	obj.SetUID("uid-1")
	obj.SetResourceVersion("42")
	obj.SetLabels(map[string]string{"kubeop.project.id": "proj-1"})
	obj.Object["status"] = map[string]any{"phase": "Running"}

	manager.ProcessObjectForTest(kind, "Added", obj)

	events := sink.Events()
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	ev := events[0]
	if ev.ClusterID != "cluster-123" {
		t.Fatalf("unexpected cluster id: %s", ev.ClusterID)
	}
	if ev.Kind != "Pod" || ev.Namespace != "user-example" || ev.Name != "web-1" {
		t.Fatalf("unexpected event metadata: %+v", ev)
	}
	if !strings.Contains(ev.Summary, "phase=running") {
		t.Fatalf("summary should include phase, got %q", ev.Summary)
	}
	rv, err := store.GetResourceVersion("Pod")
	if err != nil {
		t.Fatalf("read resource version: %v", err)
	}
	if rv != "42" {
		t.Fatalf("unexpected resource version %s", rv)
	}

	// Missing labels should not enqueue additional events.
	obj2 := obj.DeepCopy()
	obj2.SetUID("uid-2")
	obj2.SetResourceVersion("43")
	obj2.SetLabels(map[string]string{})
	manager.ProcessObjectForTest(kind, "Added", obj2)
	if len(sink.Events()) != 1 {
		t.Fatalf("expected no additional events when labels missing")
	}
}

func TestManagerNamespaceFilter(t *testing.T) {
	scheme := runtime.NewScheme()
	client := fake.NewSimpleDynamicClient(scheme)
	storePath := filepath.Join(t.TempDir(), "state.db")
	store, err := statepkg.Open(storePath)
	if err != nil {
		t.Fatalf("open state store: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

	sink := &capturingSink{}
	kind := watchpkg.Kind{Name: "Pod", GVR: schema.GroupVersionResource{Group: "", Version: "v1", Resource: "pods"}}
	manager, err := watchpkg.NewManager(client, store, sink, watchpkg.Options{
		Kinds:             []watchpkg.Kind{kind},
		NamespacePrefixes: []string{"user-"},
	})
	if err != nil {
		t.Fatalf("new manager: %v", err)
	}

	obj := &unstructured.Unstructured{}
	obj.SetAPIVersion("v1")
	obj.SetKind("Pod")
	obj.SetNamespace("system")
	obj.SetName("web-1")
	obj.SetUID("uid-1")
	obj.SetResourceVersion("1")

	manager.ProcessObjectForTest(kind, "Added", obj)
	if len(sink.Events()) != 0 {
		t.Fatalf("expected namespace filter to drop event")
	}
}

func TestManagerSkipsUnavailableKinds(t *testing.T) {
	scheme := runtime.NewScheme()
	client := fake.NewSimpleDynamicClientWithCustomListKinds(scheme, map[schema.GroupVersionResource]string{
		{Group: "", Version: "v1", Resource: "pods"}:                        "PodList",
		{Group: "cert-manager.io", Version: "v1", Resource: "certificates"}: "CertificateList",
	})
	client.PrependReactor("list", "certificates", func(action k8stesting.Action) (bool, runtime.Object, error) {
		return true, nil, apierrors.NewNotFound(schema.GroupResource{Group: "cert-manager.io", Resource: "certificates"}, "")
	})
	storePath := filepath.Join(t.TempDir(), "state.db")
	store, err := statepkg.Open(storePath)
	if err != nil {
		t.Fatalf("open state store: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

	sink := &capturingSink{}
	podKind := watchpkg.Kind{Name: "Pod", GVR: schema.GroupVersionResource{Group: "", Version: "v1", Resource: "pods"}}
	certKind := watchpkg.Kind{Name: "Certificate", GVR: schema.GroupVersionResource{Group: "cert-manager.io", Version: "v1", Resource: "certificates"}}

	manager, err := watchpkg.NewManager(client, store, sink, watchpkg.Options{
		Kinds:          []watchpkg.Kind{podKind, certKind},
		RequiredLabels: []string{"kubeop.project.id"},
	})
	if err != nil {
		t.Fatalf("new manager: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := make(chan error, 1)
	go func() {
		done <- manager.Start(ctx)
	}()

	waitForReady(t, manager, 2*time.Second)

	cancel()
	if err := <-done; err != nil {
		t.Fatalf("manager.Start returned error: %v", err)
	}
}

func TestManagerAllKindsUnavailableStillReady(t *testing.T) {
	scheme := runtime.NewScheme()
	client := fake.NewSimpleDynamicClientWithCustomListKinds(scheme, map[schema.GroupVersionResource]string{
		{Group: "cert-manager.io", Version: "v1", Resource: "certificates"}: "CertificateList",
	})
	client.PrependReactor("list", "certificates", func(action k8stesting.Action) (bool, runtime.Object, error) {
		return true, nil, apierrors.NewNotFound(schema.GroupResource{Group: "cert-manager.io", Resource: "certificates"}, "")
	})
	storePath := filepath.Join(t.TempDir(), "state.db")
	store, err := statepkg.Open(storePath)
	if err != nil {
		t.Fatalf("open state store: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

	sink := &capturingSink{}
	certKind := watchpkg.Kind{Name: "Certificate", GVR: schema.GroupVersionResource{Group: "cert-manager.io", Version: "v1", Resource: "certificates"}}

	manager, err := watchpkg.NewManager(client, store, sink, watchpkg.Options{
		Kinds: []watchpkg.Kind{certKind},
	})
	if err != nil {
		t.Fatalf("new manager: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := make(chan error, 1)
	go func() {
		done <- manager.Start(ctx)
	}()

	waitForReady(t, manager, 2*time.Second)

	cancel()
	if err := <-done; err != nil {
		t.Fatalf("manager.Start returned error: %v", err)
	}
}

func waitForReady(t *testing.T, manager *watchpkg.Manager, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if manager.Ready() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("manager never reported ready within %s", timeout)
}
