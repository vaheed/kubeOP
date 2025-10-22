package testcase

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func runTenantProjectScript(t *testing.T, script string) string {
	t.Helper()
	root, err := filepath.Abs("..")
	if err != nil {
		t.Fatalf("abs root: %v", err)
	}
	cmd := exec.Command(filepath.Join(root, "samples", "01-tenant-project", script))
	cmd.Dir = root
	env := os.Environ()
	env = append(env,
		"AUTH_TOKEN=test-token",
		"TENANT_EMAIL=tenant@example.com",
		"TENANT_NAME=Tenant Example",
		"PROJECT_NAME=tenant-demo",
		"CLUSTER_ID=clu-12345",
		"DRY_RUN=1",
	)
	cmd.Env = env
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("%s failed: %v (output: %s)", script, err, string(out))
	}
	return string(out)
}

func TestTenantProjectSampleDryRun(t *testing.T) {
	out := runTenantProjectScript(t, "curl.sh")
	if !strings.Contains(out, "DRY_RUN=1 skipping POST /v1/users/bootstrap") {
		t.Fatalf("expected dry-run warning for user bootstrap, got output: %s", out)
	}
	if !strings.Contains(out, "DRY_RUN=1 skipping POST /v1/projects") {
		t.Fatalf("expected dry-run warning for project creation, got output: %s", out)
	}
}

func TestTenantProjectVerifyDryRun(t *testing.T) {
	out := runTenantProjectScript(t, "verify.sh")
	if !strings.Contains(out, "DRY_RUN=1 skipping GET /v1/projects?limit=10") {
		t.Fatalf("expected dry-run warning for project lookup, got output: %s", out)
	}
	if !strings.Contains(out, "DRY_RUN=1 skipping POST /v1/projects/<project-id>/kubeconfig/renew") {
		t.Fatalf("expected dry-run warning for kubeconfig renewal, got output: %s", out)
	}
}
