package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestShardPackagesDeterministic(t *testing.T) {
	pkgs := []string{"internal/foo", "cmd/kubeop", "pkg/api", "api/types", "docs/gen"}
	shard := shardPackages(pkgs, 2, 1)
	expected := []string{"cmd/kubeop", "internal/foo"}
	if len(shard) != len(expected) {
		t.Fatalf("expected %d packages, got %d", len(expected), len(shard))
	}
	for i, pkg := range expected {
		if shard[i] != pkg {
			t.Fatalf("expected shard[%d] = %q, got %q", i, pkg, shard[i])
		}
	}
}

func TestCollectPackagesFromList(t *testing.T) {
	pkgs := collectPackages("cmd/kubeop, internal/foo\n pkg/api ", "")
	expected := []string{"cmd/kubeop", "internal/foo", "pkg/api"}
	if len(pkgs) != len(expected) {
		t.Fatalf("expected %d packages, got %d", len(expected), len(pkgs))
	}
	for i, pkg := range expected {
		if pkgs[i] != pkg {
			t.Fatalf("expected pkgs[%d] = %q, got %q", i, pkg, pkgs[i])
		}
	}
}

func TestCollectPackagesFromFile(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "pkgs.txt")
	content := "cmd/kubeop\n\n internal/foo \n pkg/api\n"
	if err := os.WriteFile(file, []byte(content), 0o600); err != nil {
		t.Fatalf("write file: %v", err)
	}
	pkgs := collectPackages("", file)
	expected := []string{"cmd/kubeop", "internal/foo", "pkg/api"}
	if len(pkgs) != len(expected) {
		t.Fatalf("expected %d packages, got %d", len(expected), len(pkgs))
	}
	for i, pkg := range expected {
		if pkgs[i] != pkg {
			t.Fatalf("expected pkgs[%d] = %q, got %q", i, pkg, pkgs[i])
		}
	}
}
