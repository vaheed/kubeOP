package testcase

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"

	"kubeop/internal/service"
	"kubeop/internal/store"
	"kubeop/internal/util"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	netv1 "k8s.io/api/networking/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	crclient "sigs.k8s.io/controller-runtime/pkg/client"
)

type mockClient struct {
	scheme *runtime.Scheme
	getFn  func(context.Context, crclient.ObjectKey, crclient.Object) error
	listFn func(context.Context, crclient.ObjectList, ...crclient.ListOption) error
}

func (m *mockClient) Get(ctx context.Context, key crclient.ObjectKey, obj crclient.Object, opts ...crclient.GetOption) error {
	if m.getFn != nil {
		return m.getFn(ctx, key, obj)
	}
	return nil
}

func (m *mockClient) List(ctx context.Context, list crclient.ObjectList, opts ...crclient.ListOption) error {
	if m.listFn != nil {
		return m.listFn(ctx, list, opts...)
	}
	return nil
}

func (m *mockClient) Create(context.Context, crclient.Object, ...crclient.CreateOption) error {
	return errors.New("not implemented")
}

func (m *mockClient) Delete(context.Context, crclient.Object, ...crclient.DeleteOption) error {
	return errors.New("not implemented")
}

func (m *mockClient) Update(context.Context, crclient.Object, ...crclient.UpdateOption) error {
	return errors.New("not implemented")
}

func (m *mockClient) Patch(context.Context, crclient.Object, crclient.Patch, ...crclient.PatchOption) error {
	return errors.New("not implemented")
}

func (m *mockClient) DeleteAllOf(context.Context, crclient.Object, ...crclient.DeleteAllOfOption) error {
	return errors.New("not implemented")
}

func (m *mockClient) Scheme() *runtime.Scheme { return m.scheme }

func (m *mockClient) RESTMapper() apimeta.RESTMapper { return nil }

func (m *mockClient) Status() crclient.StatusWriter { return nil }

func (m *mockClient) SubResource(string) crclient.SubResourceClient { return nil }

func (m *mockClient) GroupVersionKindFor(runtime.Object) (schema.GroupVersionKind, error) {
	return schema.GroupVersionKind{}, nil
}

func (m *mockClient) IsObjectNamespaced(runtime.Object) (bool, error) {
	return true, nil
}

func newTestLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelWarn}))
}

func newScheme(t *testing.T) *runtime.Scheme {
	t.Helper()
	scheme := runtime.NewScheme()
	if err := clientgoscheme.AddToScheme(scheme); err != nil {
		t.Fatalf("add client-go scheme: %v", err)
	}
	if err := appsv1.AddToScheme(scheme); err != nil {
		t.Fatalf("add apps scheme: %v", err)
	}
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatalf("add core scheme: %v", err)
	}
	if err := netv1.AddToScheme(scheme); err != nil {
		t.Fatalf("add net scheme: %v", err)
	}
	return scheme
}

func TestCollectAppStatus_SummarizesResources(t *testing.T) {
	t.Parallel()

	scheme := newScheme(t)
	ns := "demo"
	app := store.App{ID: "app-1", Name: "My App"}
	slug := util.Slugify(app.Name)
	replicas := int32(3)

	dep := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: slug, Namespace: ns},
		Spec:       appsv1.DeploymentSpec{Replicas: &replicas},
		Status:     appsv1.DeploymentStatus{ReadyReplicas: 2, AvailableReplicas: 2},
	}
	dep.Spec.Selector = &metav1.LabelSelector{MatchLabels: map[string]string{"kubeop.app-id": app.ID}}
	dep.Spec.Template.ObjectMeta.Labels = map[string]string{"kubeop.app-id": app.ID}
	dep.Spec.Template.Spec.Containers = []corev1.Container{{Name: "app", Image: "demo"}}

	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{Name: slug, Namespace: ns, Labels: map[string]string{"kubeop.app-id": app.ID}},
		Spec: corev1.ServiceSpec{
			Type:     corev1.ServiceTypeClusterIP,
			Selector: map[string]string{"kubeop.app-id": app.ID},
			Ports:    []corev1.ServicePort{{Port: 8080}},
		},
	}

	pathType := netv1.PathTypePrefix
	ing := &netv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{Name: slug, Namespace: ns, Labels: map[string]string{"kubeop.app-id": app.ID}},
		Spec: netv1.IngressSpec{Rules: []netv1.IngressRule{{
			Host: "app.example.com",
			IngressRuleValue: netv1.IngressRuleValue{HTTP: &netv1.HTTPIngressRuleValue{Paths: []netv1.HTTPIngressPath{{
				Path:     "/",
				PathType: &pathType,
				Backend:  netv1.IngressBackend{Service: &netv1.IngressServiceBackend{Name: slug, Port: netv1.ServiceBackendPort{Number: 8080}}},
			}}}},
		}}},
	}

	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: slug + "-pod", Namespace: ns, Labels: map[string]string{"kubeop.app-id": app.ID}},
		Status:     corev1.PodStatus{Phase: corev1.PodRunning, ContainerStatuses: []corev1.ContainerStatus{{Ready: true}}},
	}

	client := &mockClient{
		scheme: scheme,
		getFn: func(_ context.Context, key crclient.ObjectKey, obj crclient.Object) error {
			if key.Name != slug {
				return apierrors.NewNotFound(appsv1.Resource("deployments"), key.Name)
			}
			if out, ok := obj.(*appsv1.Deployment); ok {
				dep.DeepCopyInto(out)
				return nil
			}
			return errors.New("unexpected type for Get")
		},
		listFn: func(_ context.Context, list crclient.ObjectList, _ ...crclient.ListOption) error {
			switch typed := list.(type) {
			case *corev1.ServiceList:
				typed.Items = append(typed.Items, *svc.DeepCopy())
			case *netv1.IngressList:
				typed.Items = append(typed.Items, *ing.DeepCopy())
			case *corev1.PodList:
				typed.Items = append(typed.Items, *pod.DeepCopy())
			default:
				return errors.New("unexpected list type")
			}
			return nil
		},
	}

	st := service.CollectAppStatus(context.Background(), client, ns, app, newTestLogger())

	if st.Desired != replicas {
		t.Fatalf("expected desired replicas %d, got %d", replicas, st.Desired)
	}
	if st.Ready != dep.Status.ReadyReplicas || st.Available != dep.Status.AvailableReplicas {
		t.Fatalf("unexpected readiness counts: %+v", st)
	}
	if st.Service == nil || st.Service.Name != slug {
		t.Fatalf("expected service summary for %s, got %#v", slug, st.Service)
	}
	if len(st.Service.Ports) != 1 || st.Service.Ports[0] != 8080 {
		t.Fatalf("expected single service port 8080, got %#v", st.Service.Ports)
	}
	if len(st.IngressHosts) != 1 || st.IngressHosts[0] != "app.example.com" {
		t.Fatalf("expected ingress host app.example.com, got %#v", st.IngressHosts)
	}
	if len(st.Pods) != 1 || !st.Pods[0].Ready {
		t.Fatalf("expected single ready pod, got %#v", st.Pods)
	}
}

func TestCollectAppStatus_HandlesMissingResources(t *testing.T) {
	t.Parallel()

	scheme := newScheme(t)
	app := store.App{ID: "app-2", Name: "Ghost"}

	client := &mockClient{
		scheme: scheme,
		getFn: func(_ context.Context, key crclient.ObjectKey, obj crclient.Object) error {
			return apierrors.NewNotFound(appsv1.Resource("deployments"), key.Name)
		},
		listFn: func(_ context.Context, list crclient.ObjectList, _ ...crclient.ListOption) error {
			return nil
		},
	}

	st := service.CollectAppStatus(context.Background(), client, "ghost-ns", app, newTestLogger())

	if st.AppID != app.ID || st.Name != app.Name {
		t.Fatalf("unexpected identity fields: %#v", st)
	}
	if st.Service != nil {
		t.Fatalf("expected no service summary, got %#v", st.Service)
	}
	if len(st.IngressHosts) != 0 {
		t.Fatalf("expected no ingress hosts, got %#v", st.IngressHosts)
	}
	if len(st.Pods) != 0 {
		t.Fatalf("expected no pods, got %#v", st.Pods)
	}
}
