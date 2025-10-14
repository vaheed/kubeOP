package testcase

import (
	"os"
	"path/filepath"
	"testing"

	"gopkg.in/yaml.v3"

	"kubeop/internal/version"
)

func TestOpenAPIVersionMatchesBinary(t *testing.T) {
	specPath := filepath.Clean(filepath.Join("..", "docs", "openapi.yaml"))
	raw, err := os.ReadFile(specPath)
	if err != nil {
		t.Fatalf("failed reading openapi spec: %v", err)
	}

	type openAPIInfo struct {
		Info struct {
			Version string `yaml:"version"`
		} `yaml:"info"`
	}

	var spec openAPIInfo
	if err := yaml.Unmarshal(raw, &spec); err != nil {
		t.Fatalf("failed parsing openapi spec: %v", err)
	}

	if spec.Info.Version != version.Version {
		t.Fatalf("openapi info.version %q does not match binary version %q", spec.Info.Version, version.Version)
	}
}
