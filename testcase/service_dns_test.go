package testcase

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"kubeop/internal/service"
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
