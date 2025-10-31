//go:build e2e

package e2e

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func TestEndToEndSuite(t *testing.T) {
	t.Parallel()

	artifacts := os.Getenv("ARTIFACTS_DIR")
	if artifacts == "" {
		artifacts = filepath.Join(os.TempDir(), "test-artifacts", "e2e")
	}
	if err := os.MkdirAll(artifacts, 0o755); err != nil {
		t.Fatalf("create artifacts dir: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 75*time.Minute)
	defer cancel()

	cmd := exec.CommandContext(ctx, "bash", "hack/e2e/run.sh")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = append(os.Environ(), "ARTIFACTS_DIR="+artifacts)
	if err := cmd.Run(); err != nil {
		t.Fatalf("e2e suite failed: %v", err)
	}
}
