package testcase

import (
	"kubeop/internal/version"
	"testing"
)

func TestVersion_Bumped(t *testing.T) {
        if version.Version != "0.3.2" {
                t.Fatalf("expected version 0.3.2, got %q", version.Version)
        }
}
