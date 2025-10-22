package testcase

import (
	"testing"

	"kubeop/internal/service"
	"kubeop/internal/store"
)

func TestCanonicalAppLabelsIncludesCanonicalKeys(t *testing.T) {
	t.Parallel()

	project := store.Project{
		ID:        "proj-123",
		Name:      "Payments Portal",
		Namespace: "tenant-payments",
		ClusterID: "cluster-eu-1",
		UserID:    "tenant-42",
	}
	labels := service.CanonicalAppLabels(project, "Orders API", "orders-api-1", "app-789")

	required := map[string]string{
		"kubeop.app-id":       "app-789",
		"kubeop.app.id":       "app-789",
		"kubeop.app.name":     "orders-api-1",
		"kubeop.project.id":   "proj-123",
		"kubeop.project.name": "payments-portal",
		"kubeop.cluster.id":   "cluster-eu-1",
		"kubeop.tenant.id":    "tenant-42",
	}
	for key, want := range required {
		got, ok := labels[key]
		if !ok {
			t.Fatalf("expected label %q to be set", key)
		}
		if got != want {
			t.Fatalf("expected label %q to equal %q, got %q", key, want, got)
		}
	}
}

func TestCanonicalAppLabelsFallsBackToNamespaceAndSlug(t *testing.T) {
	t.Parallel()

	project := store.Project{ID: "proj-1", Namespace: "Tenant-Primary"}
	labels := service.CanonicalAppLabels(project, "Hello World", "", "app-1")

	if got := labels["kubeop.app.name"]; got != "hello-world" {
		t.Fatalf("expected kubeop.app.name to slugify app name, got %q", got)
	}
	if got := labels["kubeop.project.name"]; got != "tenant-primary" {
		t.Fatalf("expected kubeop.project.name to slugify namespace fallback, got %q", got)
	}
}
