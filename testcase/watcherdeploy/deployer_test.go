package watcherdeploy_test

import (
	"context"
	"crypto/sha256"
	"fmt"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
	"kubeop/internal/watcherdeploy"
)

func TestEnsureCreatesResources(t *testing.T) {
	t.Parallel()

	cfg := watcherdeploy.Config{
		Namespace:          "kubeop-system",
		CreateNamespace:    true,
		DeploymentName:     "kubeop-watcher",
		ServiceAccountName: "kubeop-watcher",
		SecretName:         "kubeop-watcher",
		PVCName:            "kubeop-watcher-state",
		PVCSize:            "1Gi",
		Image:              "ghcr.io/vaheed/kubeop-watcher:latest",
		EventsURL:          "https://kubeop.example.com/v1/events/ingest",
		Token:              "test-token",
		StorePath:          "/var/lib/kubeop-watcher/state.db",
		WaitForReady:       false,
	}

	clientset := fake.NewSimpleClientset()
	deployer, err := watcherdeploy.New(cfg, func(ctx context.Context, clusterID string, loader watcherdeploy.Loader) (kubernetes.Interface, error) {
		return clientset, nil
	})
	if err != nil {
		t.Fatalf("watcherdeploy.New: %v", err)
	}

	if err := deployer.Ensure(context.Background(), "cluster-1", "primary", func(context.Context) ([]byte, error) {
		return []byte("kubeconfig"), nil
	}); err != nil {
		t.Fatalf("Ensure: %v", err)
	}

	if _, err := clientset.CoreV1().Namespaces().Get(context.Background(), cfg.Namespace, metav1.GetOptions{}); err != nil {
		t.Fatalf("expected namespace created: %v", err)
	}
	if _, err := clientset.CoreV1().ServiceAccounts(cfg.Namespace).Get(context.Background(), cfg.ServiceAccountName, metav1.GetOptions{}); err != nil {
		t.Fatalf("expected service account: %v", err)
	}
	if _, err := clientset.RbacV1().ClusterRoles().Get(context.Background(), cfg.ServiceAccountName, metav1.GetOptions{}); err != nil {
		t.Fatalf("expected cluster role: %v", err)
	}
	if _, err := clientset.RbacV1().ClusterRoleBindings().Get(context.Background(), cfg.ServiceAccountName, metav1.GetOptions{}); err != nil {
		t.Fatalf("expected cluster role binding: %v", err)
	}
	secret, err := clientset.CoreV1().Secrets(cfg.Namespace).Get(context.Background(), cfg.SecretName, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("expected secret: %v", err)
	}
	if string(secret.Data["token"]) != cfg.Token {
		t.Fatalf("expected secret token stored, got %q", string(secret.Data["token"]))
	}
	expectedHash := fmt.Sprintf("%x", sha256.Sum256([]byte(cfg.Token)))
	if secret.Annotations["kubeop.io/token-sha256"] != expectedHash {
		t.Fatalf("expected token hash annotation, got %q", secret.Annotations["kubeop.io/token-sha256"])
	}
	if _, err := clientset.CoreV1().PersistentVolumeClaims(cfg.Namespace).Get(context.Background(), cfg.PVCName, metav1.GetOptions{}); err != nil {
		t.Fatalf("expected pvc: %v", err)
	}
	dep, err := clientset.AppsV1().Deployments(cfg.Namespace).Get(context.Background(), cfg.DeploymentName, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("expected deployment: %v", err)
	}
	if len(dep.Spec.Template.Spec.Containers) != 1 {
		t.Fatalf("expected single container, got %d", len(dep.Spec.Template.Spec.Containers))
	}
	podSpec := dep.Spec.Template.Spec
	if podSpec.SecurityContext == nil || podSpec.SecurityContext.RunAsNonRoot == nil || !*podSpec.SecurityContext.RunAsNonRoot {
		t.Fatalf("expected pod to run as non-root")
	}
	if podSpec.SecurityContext.RunAsUser == nil || *podSpec.SecurityContext.RunAsUser != 65532 {
		t.Fatalf("expected pod RunAsUser 65532, got %v", podSpec.SecurityContext.RunAsUser)
	}
	if podSpec.SecurityContext.RunAsGroup == nil || *podSpec.SecurityContext.RunAsGroup != 65532 {
		t.Fatalf("expected pod RunAsGroup 65532, got %v", podSpec.SecurityContext.RunAsGroup)
	}
	if podSpec.SecurityContext.FSGroup == nil || *podSpec.SecurityContext.FSGroup != 65532 {
		t.Fatalf("expected pod FSGroup 65532, got %v", podSpec.SecurityContext.FSGroup)
	}
	if podSpec.SecurityContext.SeccompProfile == nil || podSpec.SecurityContext.SeccompProfile.Type != corev1.SeccompProfileTypeRuntimeDefault {
		t.Fatalf("expected runtimeDefault seccomp profile")
	}
	foundURL := false
	foundLogsRoot := false
	for _, env := range dep.Spec.Template.Spec.Containers[0].Env {
		switch env.Name {
		case "KUBEOP_EVENTS_URL":
			if env.Value != cfg.EventsURL {
				t.Fatalf("expected events url %q, got %q", cfg.EventsURL, env.Value)
			}
			foundURL = true
		case "LOGS_ROOT":
			if env.Value != "/var/lib/kubeop-watcher/logs" {
				t.Fatalf("expected logs root /var/lib/kubeop-watcher/logs, got %q", env.Value)
			}
			foundLogsRoot = true
		}
	}
	if !foundURL {
		t.Fatalf("expected events url env var")
	}
	if !foundLogsRoot {
		t.Fatalf("expected logs root env var")
	}
	container := podSpec.Containers[0]
	if container.SecurityContext == nil {
		t.Fatalf("expected container security context")
	}
	if container.SecurityContext.AllowPrivilegeEscalation == nil || *container.SecurityContext.AllowPrivilegeEscalation {
		t.Fatalf("expected privilege escalation disabled")
	}
	if container.SecurityContext.RunAsNonRoot == nil || !*container.SecurityContext.RunAsNonRoot {
		t.Fatalf("expected container runAsNonRoot")
	}
	if container.SecurityContext.Capabilities == nil {
		t.Fatalf("expected container capabilities drop all")
	}
	if container.SecurityContext.RunAsUser == nil || *container.SecurityContext.RunAsUser != 65532 {
		t.Fatalf("expected container RunAsUser 65532, got %v", container.SecurityContext.RunAsUser)
	}
	if container.SecurityContext.RunAsGroup == nil || *container.SecurityContext.RunAsGroup != 65532 {
		t.Fatalf("expected container RunAsGroup 65532, got %v", container.SecurityContext.RunAsGroup)
	}
	dropAll := false
	for _, cap := range container.SecurityContext.Capabilities.Drop {
		if cap == corev1.Capability("ALL") {
			dropAll = true
			break
		}
	}
	if !dropAll {
		t.Fatalf("expected container to drop ALL capabilities")
	}
}

func TestEnsureRespectsRunAsOverrides(t *testing.T) {
	t.Parallel()

	cfg := watcherdeploy.Config{
		Namespace:          "kubeop-system",
		CreateNamespace:    true,
		DeploymentName:     "kubeop-watcher",
		ServiceAccountName: "kubeop-watcher",
		SecretName:         "kubeop-watcher",
		Image:              "ghcr.io/vaheed/kubeop-watcher:latest",
		EventsURL:          "https://kubeop.example.com/v1/events/ingest",
		Token:              "token",
		StorePath:          "/var/lib/kubeop-watcher/state.db",
		RunAsUser:          2000,
		RunAsGroup:         3000,
		FSGroup:            4000,
		WaitForReady:       false,
	}

	clientset := fake.NewSimpleClientset()
	deployer, err := watcherdeploy.New(cfg, func(ctx context.Context, clusterID string, loader watcherdeploy.Loader) (kubernetes.Interface, error) {
		return clientset, nil
	})
	if err != nil {
		t.Fatalf("watcherdeploy.New: %v", err)
	}

	if err := deployer.Ensure(context.Background(), "cluster-override", "", func(context.Context) ([]byte, error) {
		return []byte("kubeconfig"), nil
	}); err != nil {
		t.Fatalf("Ensure: %v", err)
	}

	dep, err := clientset.AppsV1().Deployments(cfg.Namespace).Get(context.Background(), cfg.DeploymentName, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("expected deployment: %v", err)
	}
	podSpec := dep.Spec.Template.Spec
	if podSpec.SecurityContext == nil || podSpec.SecurityContext.RunAsUser == nil || *podSpec.SecurityContext.RunAsUser != cfg.RunAsUser {
		t.Fatalf("expected pod RunAsUser %d, got %v", cfg.RunAsUser, podSpec.SecurityContext.RunAsUser)
	}
	if podSpec.SecurityContext.RunAsGroup == nil || *podSpec.SecurityContext.RunAsGroup != cfg.RunAsGroup {
		t.Fatalf("expected pod RunAsGroup %d, got %v", cfg.RunAsGroup, podSpec.SecurityContext.RunAsGroup)
	}
	if podSpec.SecurityContext.FSGroup == nil || *podSpec.SecurityContext.FSGroup != cfg.FSGroup {
		t.Fatalf("expected pod FSGroup %d, got %v", cfg.FSGroup, podSpec.SecurityContext.FSGroup)
	}
	container := podSpec.Containers[0]
	if container.SecurityContext == nil || container.SecurityContext.RunAsUser == nil || *container.SecurityContext.RunAsUser != cfg.RunAsUser {
		t.Fatalf("expected container RunAsUser %d, got %v", cfg.RunAsUser, container.SecurityContext.RunAsUser)
	}
	if container.SecurityContext.RunAsGroup == nil || *container.SecurityContext.RunAsGroup != cfg.RunAsGroup {
		t.Fatalf("expected container RunAsGroup %d, got %v", cfg.RunAsGroup, container.SecurityContext.RunAsGroup)
	}
}

func TestEnsureUsesTokenProvider(t *testing.T) {
	t.Parallel()

	cfg := watcherdeploy.Config{
		Namespace:          "kubeop-system",
		CreateNamespace:    true,
		DeploymentName:     "kubeop-watcher",
		ServiceAccountName: "kubeop-watcher",
		SecretName:         "kubeop-watcher",
		Image:              "ghcr.io/vaheed/kubeop-watcher:latest",
		EventsURL:          "https://kubeop.example.com/v1/events/ingest",
		StorePath:          "/var/lib/kubeop-watcher/state.db",
		WaitForReady:       false,
	}
	clientset := fake.NewSimpleClientset()
	provider := func(ctx context.Context, clusterID string) (string, error) {
		return "token-for-" + clusterID, nil
	}
	deployer, err := watcherdeploy.New(cfg, func(ctx context.Context, clusterID string, loader watcherdeploy.Loader) (kubernetes.Interface, error) {
		return clientset, nil
	}, watcherdeploy.WithTokenProvider(provider))
	if err != nil {
		t.Fatalf("watcherdeploy.New: %v", err)
	}
	if err := deployer.Ensure(context.Background(), "cluster-xyz", "", func(context.Context) ([]byte, error) {
		return []byte("kubeconfig"), nil
	}); err != nil {
		t.Fatalf("Ensure: %v", err)
	}
	secret, err := clientset.CoreV1().Secrets(cfg.Namespace).Get(context.Background(), cfg.SecretName, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("expected secret: %v", err)
	}
	expectedToken := "token-for-cluster-xyz"
	if string(secret.Data["token"]) != expectedToken {
		t.Fatalf("expected token from provider, got %q", string(secret.Data["token"]))
	}
	expectedHash := fmt.Sprintf("%x", sha256.Sum256([]byte(expectedToken)))
	if secret.Annotations["kubeop.io/token-sha256"] != expectedHash {
		t.Fatalf("expected hash annotation from provider")
	}
}

func TestEnsureWaitForReadyTimesOut(t *testing.T) {
	t.Parallel()

	ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "kubeop-system"}}
	clientset := fake.NewSimpleClientset(ns)
	cfg := watcherdeploy.Config{
		Namespace:          "kubeop-system",
		DeploymentName:     "kubeop-watcher",
		ServiceAccountName: "kubeop-watcher",
		SecretName:         "kubeop-watcher",
		Image:              "ghcr.io/vaheed/kubeop-watcher:latest",
		EventsURL:          "https://kubeop.example.com/v1/events/ingest",
		Token:              "test-token",
		StorePath:          "/var/lib/kubeop-watcher/state.db",
		WaitForReady:       true,
		ReadyTimeout:       time.Second,
	}
	deployer, err := watcherdeploy.New(cfg, func(ctx context.Context, clusterID string, loader watcherdeploy.Loader) (kubernetes.Interface, error) {
		return clientset, nil
	})
	if err != nil {
		t.Fatalf("watcherdeploy.New: %v", err)
	}
	ctx := context.Background()
	if err := deployer.Ensure(ctx, "cluster-2", "secondary", func(context.Context) ([]byte, error) {
		return []byte("kubeconfig"), nil
	}); err == nil {
		t.Fatalf("expected readiness timeout error")
	}
}
