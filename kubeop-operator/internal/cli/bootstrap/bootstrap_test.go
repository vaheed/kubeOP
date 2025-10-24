package bootstrap

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	appv1alpha1 "github.com/vaheed/kubeOP/kubeop-operator/apis/paas/v1alpha1"
	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestBuildTenantDefaultsAndValidation(t *testing.T) {
	_, err := BuildTenant(TenantInput{Name: "", BillingAccountRef: "acct"})
	if err == nil {
		t.Fatalf("expected error for empty name")
	}
	tenant, err := BuildTenant(TenantInput{Name: "acme", DisplayName: "", BillingAccountRef: "acct"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tenant.Spec.DisplayName != "acme" {
		t.Fatalf("expected display name default, got %s", tenant.Spec.DisplayName)
	}
	if tenant.Labels["paas.kubeop.io/tenant"] != "acme" {
		t.Fatalf("expected tenant label to be set")
	}
}

func TestBuildProjectDefaultsEnvironment(t *testing.T) {
	project, err := BuildProject(ProjectInput{Name: "app", Namespace: "app-ns", TenantRef: "acme", Purpose: "demo"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if project.Spec.Environment != appv1alpha1.ProjectEnvironmentDev {
		t.Fatalf("expected dev environment by default, got %s", project.Spec.Environment)
	}
	if project.Labels["paas.kubeop.io/project"] != "app" {
		t.Fatalf("expected project label to match name")
	}
}

func TestApplyObjectCreatesResource(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := clientgoscheme.AddToScheme(scheme); err != nil {
		t.Fatalf("add core scheme: %v", err)
	}
	if err := appv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatalf("add paas scheme: %v", err)
	}
	kubeClient := fake.NewClientBuilder().WithScheme(scheme).Build()
	tenant, err := BuildTenant(TenantInput{Name: "acme", BillingAccountRef: "acct"})
	if err != nil {
		t.Fatalf("build tenant: %v", err)
	}
	if err := ApplyObject(context.Background(), kubeClient, scheme, tenant, "test-owner"); err != nil {
		t.Fatalf("apply object: %v", err)
	}
	var fetched appv1alpha1.Tenant
	if err := kubeClient.Get(context.Background(), client.ObjectKey{Name: "acme"}, &fetched); err != nil {
		t.Fatalf("get tenant: %v", err)
	}
}

type recordingSink struct {
	events []AuditEvent
}

func (r *recordingSink) Emit(_ context.Context, event AuditEvent) error {
	r.events = append(r.events, event)
	return nil
}

func TestRunnerApplyObjectWritesManifests(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := clientgoscheme.AddToScheme(scheme); err != nil {
		t.Fatalf("add core scheme: %v", err)
	}
	if err := appv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatalf("add paas scheme: %v", err)
	}
	kubeClient := fake.NewClientBuilder().WithScheme(scheme).Build()
	sink := &recordingSink{}
	logger := zap.NewNop().Sugar()
	outDir := t.TempDir()
	runner, err := NewRunner(kubeClient, scheme, logger, sink, outDir)
	if err != nil {
		t.Fatalf("new runner: %v", err)
	}
	tenant, err := BuildTenant(TenantInput{Name: "acme", BillingAccountRef: "acct"})
	if err != nil {
		t.Fatalf("build tenant: %v", err)
	}
	summary, err := runner.ApplyObject(context.Background(), tenant)
	if err != nil {
		t.Fatalf("apply tenant: %v", err)
	}
	if summary.Name != "acme" || summary.Kind != "Tenant" {
		t.Fatalf("unexpected summary: %+v", summary)
	}
	if len(sink.events) != 1 {
		t.Fatalf("expected one event, got %d", len(sink.events))
	}
	manifest := filepath.Join(outDir, "tenant_acme.yaml")
	data, err := os.ReadFile(manifest)
	if err != nil {
		t.Fatalf("read manifest: %v", err)
	}
	if !strings.Contains(string(data), "kind: Tenant") {
		t.Fatalf("manifest does not contain kind: %s", data)
	}
}

func TestRenderOutputTable(t *testing.T) {
	var buf bytes.Buffer
	rows := [][]string{{"A", "B"}, {"1", "2"}}
	if err := RenderOutput(&buf, "table", func(runtime.Object) ([]byte, error) {
		return nil, errors.New("should not be called")
	}, rows, nil); err != nil {
		t.Fatalf("render table: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if len(lines) != 2 {
		t.Fatalf("expected two table rows, got %d", len(lines))
	}
	if fields := strings.Fields(lines[0]); len(fields) != 2 || fields[0] != "A" || fields[1] != "B" {
		t.Fatalf("unexpected header row: %q", lines[0])
	}
	if fields := strings.Fields(lines[1]); len(fields) != 2 || fields[0] != "1" || fields[1] != "2" {
		t.Fatalf("unexpected data row: %q", lines[1])
	}
}

func TestRenderOutputYAML(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := clientgoscheme.AddToScheme(scheme); err != nil {
		t.Fatalf("add core scheme: %v", err)
	}
	if err := appv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatalf("add paas scheme: %v", err)
	}
	kubeClient := fake.NewClientBuilder().WithScheme(scheme).Build()
	runner, err := NewRunner(kubeClient, scheme, zap.NewNop().Sugar(), &recordingSink{}, t.TempDir())
	if err != nil {
		t.Fatalf("new runner: %v", err)
	}
	tenant, err := BuildTenant(TenantInput{Name: "acme", BillingAccountRef: "acct"})
	if err != nil {
		t.Fatalf("build tenant: %v", err)
	}
	var buf bytes.Buffer
	if err := RenderOutput(&buf, "yaml", runner.EncodeYAML, nil, []client.Object{tenant}); err != nil {
		t.Fatalf("render yaml: %v", err)
	}
	if !strings.Contains(buf.String(), "kind: Tenant") {
		t.Fatalf("expected yaml output, got %s", buf.String())
	}
}

func TestParseRegistryType(t *testing.T) {
	got, err := parseRegistryType("ECR")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != appv1alpha1.RegistryCredentialECR {
		t.Fatalf("expected ecr, got %s", got)
	}
	if _, err := parseRegistryType("unknown"); err == nil {
		t.Fatalf("expected error for unsupported type")
	}
}
