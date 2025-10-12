package testcase

import (
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
