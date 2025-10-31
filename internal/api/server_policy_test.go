package api

import (
    "context"
    "os"
    "testing"
)

// Test isRegistryAllowed with env override to avoid Kubernetes client dependency
func Test_isRegistryAllowed_env(t *testing.T) {
    s := &Server{}
    ctx := context.Background()
    t.Setenv("KUBEOP_IMAGE_ALLOWLIST", "docker.io,ghcr.io")
    if !s.isRegistryAllowed(ctx, "docker.io") {
        t.Fatalf("expected docker.io allowed via env")
    }
    if s.isRegistryAllowed(ctx, "evil.io") {
        t.Fatalf("expected evil.io denied via env")
    }
    // cleanup
    os.Unsetenv("KUBEOP_IMAGE_ALLOWLIST")
}

