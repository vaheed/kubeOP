package testcase

import (
    "testing"
    "kubeop/internal/version"
)

func TestVersion_Bumped(t *testing.T) {
    if version.Version != "0.2.0" {
        t.Fatalf("expected version 0.2.0, got %q", version.Version)
    }
}

