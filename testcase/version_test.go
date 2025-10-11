package testcase

import (
    "testing"
    "kubeop/internal/version"
)

func TestVersion_Bumped(t *testing.T) {
    if version.Version != "0.1.2" {
        t.Fatalf("expected version 0.1.2, got %q", version.Version)
    }
}

