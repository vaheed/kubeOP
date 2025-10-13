package testcase

import (
	"strings"
	"testing"

	"kubeop/internal/service"
)

func TestExtractYAMLScalarHelpers(t *testing.T) {
	t.Parallel()

	kc := []byte(`clusters:
- cluster:
    certificate-authority-data: ZmFrZS1jYQ==
    server: https://api.example.com:6443
  name: demo
`)

	if got := service.TestExtractYAMLScalar(kc, "server:"); got != "https://api.example.com:6443" {
		t.Fatalf("expected server to parse, got %q", got)
	}
	if got := service.TestExtractYAMLScalar(kc, "certificate-authority-data:"); got != "ZmFrZS1jYQ==" {
		t.Fatalf("expected CA data to parse, got %q", got)
	}
	if got := service.TestExtractYAMLScalar(kc, "missing:"); got != "" {
		t.Fatalf("expected missing key to be empty, got %q", got)
	}
}

func TestBuildNamespaceScopedKubeconfig(t *testing.T) {
	t.Parallel()

	kc, err := service.TestBuildNamespaceScopedKubeconfig("https://api.example.com:6443", "Q0E=", "tenant", "user", "cluster", "token123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(kc, "current-context: cluster") {
		t.Fatalf("expected current context in kubeconfig: %s", kc)
	}
	if !strings.Contains(kc, "certificate-authority-data: Q0E=") {
		t.Fatalf("expected CA data to be embedded: %s", kc)
	}
	if !strings.Contains(kc, "token: token123") {
		t.Fatalf("expected token to be embedded: %s", kc)
	}
}
