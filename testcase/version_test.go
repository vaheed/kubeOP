package testcase

import (
    "testing"
    "kubeop/internal/version"
)

func TestVersion_Bumped(t *testing.T) {
    if version.Version != "0.1.3" {
        t.Fatalf("expected version 0.1.3, got %q", version.Version)
    }
}

