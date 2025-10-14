package testcase

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"kubeop/internal/service"

	"go.uber.org/zap/zapcore"
)

type stubServiceClient struct {
	mu  sync.RWMutex
	svc *corev1.Service
}

func newStubServiceClient(svc *corev1.Service) *stubServiceClient {
	c := &stubServiceClient{}
	c.setService(svc)
	return c
}

func (s *stubServiceClient) Get(_ context.Context, name string, _ metav1.GetOptions) (*corev1.Service, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.svc == nil || s.svc.Name != name {
		return nil, apierrors.NewNotFound(schema.GroupResource{Group: "", Resource: "services"}, name)
	}
	return s.svc.DeepCopy(), nil
}

func (s *stubServiceClient) setService(svc *corev1.Service) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if svc == nil {
		s.svc = nil
		return
	}
	s.svc = svc.DeepCopy()
}

func (s *stubServiceClient) updateIP(ip string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.svc == nil {
		return
	}
	cp := s.svc.DeepCopy()
	cp.Status.LoadBalancer.Ingress = []corev1.LoadBalancerIngress{{IP: ip}}
	s.svc = cp
}

func TestDNSLogFieldsIncludeContext(t *testing.T) {
	t.Parallel()

	fields := service.DNSLogFields("project-1", "app-1", "cluster-1", "demo", "web", "app.example.com")
	got := map[string]string{}
	for _, f := range fields {
		if f.Type == zapcore.StringType {
			got[f.Key] = f.String
		}
	}
	for _, key := range []string{"project_id", "app_id", "cluster_id", "namespace", "service", "host"} {
		if _, ok := got[key]; !ok {
			t.Fatalf("expected %s field to be present", key)
		}
	}
	if got["host"] != "app.example.com" {
		t.Fatalf("expected host field to be app.example.com, got %q", got["host"])
	}
}

func TestDNSErrorAnnotatesContext(t *testing.T) {
	t.Parallel()

	base := errors.New("boom")
	err := service.DNSError("ensure dns record", "cluster-1", "demo", "web", "app.example.com", base)
	if err == nil {
		t.Fatalf("expected error")
	}
	if !errors.Is(err, base) {
		t.Fatalf("expected wrapped error to contain base error")
	}
	for _, want := range []string{"cluster-1", "demo", "web", "app.example.com", "ensure dns record"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("expected error %q to contain %q", err.Error(), want)
		}
	}
}

func TestWaitForLoadBalancerIP_SucceedsAfterUpdate(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	client := newStubServiceClient(&corev1.Service{
		ObjectMeta: metav1.ObjectMeta{Name: "app", Namespace: "demo"},
		Spec:       corev1.ServiceSpec{Type: corev1.ServiceTypeLoadBalancer},
	})

	go func() {
		time.Sleep(50 * time.Millisecond)
		client.updateIP("10.0.0.10")
	}()

	ip, err := service.WaitForLoadBalancerIPForTest(ctx, client, "app", 10*time.Millisecond)
	if err != nil {
		t.Fatalf("waitForLoadBalancerIP: %v", err)
	}
	if ip != "10.0.0.10" {
		t.Fatalf("expected IP 10.0.0.10, got %s", ip)
	}
}

func TestWaitForLoadBalancerIP_ContextDeadline(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	client := newStubServiceClient(&corev1.Service{
		ObjectMeta: metav1.ObjectMeta{Name: "app", Namespace: "demo"},
		Spec:       corev1.ServiceSpec{Type: corev1.ServiceTypeLoadBalancer},
	})

	_, err := service.WaitForLoadBalancerIPForTest(ctx, client, "app", 20*time.Millisecond)
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected context deadline exceeded, got %v", err)
	}
}
