package testcase

import (
	"os"
	"path/filepath"
	"testing"

	"kubeop/internal/delivery"
)

func TestValidateCheckoutPath(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		{name: "empty", input: "", want: ""},
		{name: "dot", input: ".", want: ""},
		{name: "relative clean", input: "manifests/base", want: "manifests/base"},
		{name: "leading dot slash", input: "./overlays/dev", want: "overlays/dev"},
		{name: "trim whitespace", input: "  nested/configs  ", want: "nested/configs"},
		{name: "reject absolute", input: "/etc/passwd", wantErr: true},
		{name: "reject parent", input: "../secret", wantErr: true},
		{name: "reject parent prefix", input: "../../escape", wantErr: true},
		{name: "reject windows drive", input: "C:/configs", wantErr: true},
		{name: "reject invalid segment", input: "manifests/a:b", wantErr: true},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, err := delivery.ValidateCheckoutPath(tc.input)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error for %q", tc.input)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error for %q: %v", tc.input, err)
			}
			if got != tc.want {
				t.Fatalf("ValidateCheckoutPath(%q) = %q, want %q", tc.input, got, tc.want)
			}
		})
	}
}

func TestLoadManifestsRejectsEscapingSymlink(t *testing.T) {
	t.Parallel()

	repoRoot := t.TempDir()
	baseDir := filepath.Join(repoRoot, "manifests")
	if err := os.MkdirAll(baseDir, 0o755); err != nil {
		t.Fatalf("mkdir manifests: %v", err)
	}

	outside := filepath.Join(repoRoot, "..", "secrets")
	if err := os.MkdirAll(outside, 0o755); err != nil {
		t.Fatalf("mkdir outside: %v", err)
	}

	secretPath := filepath.Join(outside, "secret.yaml")
	if err := os.WriteFile(secretPath, []byte("apiVersion: v1\nkind: Secret\n"), 0o644); err != nil {
		t.Fatalf("write secret: %v", err)
	}

	linkPath := filepath.Join(baseDir, "secret.yaml")
	if err := os.Symlink(secretPath, linkPath); err != nil {
		t.Fatalf("symlink: %v", err)
	}

	info, err := os.Stat(baseDir)
	if err != nil {
		t.Fatalf("stat base: %v", err)
	}

	if _, err := delivery.LoadManifests(repoRoot, baseDir, info); err == nil {
		t.Fatalf("expected LoadManifests to reject symlink escaping root")
	}
}

func TestLoadManifestsReadsFiles(t *testing.T) {
	t.Parallel()

	repoRoot := t.TempDir()
	file := filepath.Join(repoRoot, "deployment.yaml")
	payload := "apiVersion: apps/v1\nkind: Deployment\nmetadata:\n  name: demo\n"
	if err := os.WriteFile(file, []byte(payload), 0o644); err != nil {
		t.Fatalf("write manifest: %v", err)
	}

	info, err := os.Stat(file)
	if err != nil {
		t.Fatalf("stat file: %v", err)
	}

	docs, err := delivery.LoadManifests(repoRoot, file, info)
	if err != nil {
		t.Fatalf("LoadManifests returned error: %v", err)
	}
	if len(docs) != 1 || docs[0] != payload {
		t.Fatalf("unexpected manifests: %#v", docs)
	}
}

func TestLoadManifestsRejectsOutsideBase(t *testing.T) {
	t.Parallel()

	repoRoot := t.TempDir()
	outsideDir := t.TempDir()
	file := filepath.Join(outsideDir, "evil.yaml")
	if err := os.WriteFile(file, []byte("apiVersion: v1\nkind: ConfigMap\n"), 0o644); err != nil {
		t.Fatalf("write outside file: %v", err)
	}
	info, err := os.Stat(file)
	if err != nil {
		t.Fatalf("stat outside file: %v", err)
	}
	if _, err := delivery.LoadManifests(repoRoot, file, info); err == nil {
		t.Fatalf("expected LoadManifests to reject path outside repository root")
	}
}

func TestLoadManifestsRejectsRelativeTraversal(t *testing.T) {
	t.Parallel()

	repoRoot := t.TempDir()
	manifests := filepath.Join(repoRoot, "manifests")
	if err := os.MkdirAll(manifests, 0o755); err != nil {
		t.Fatalf("mkdir manifests: %v", err)
	}

	outsideDir := filepath.Join(repoRoot, "..", "outside")
	if err := os.MkdirAll(outsideDir, 0o755); err != nil {
		t.Fatalf("mkdir outside: %v", err)
	}

	secretPath := filepath.Join(outsideDir, "secret.yaml")
	if err := os.WriteFile(secretPath, []byte("apiVersion: v1\nkind: Secret\n"), 0o644); err != nil {
		t.Fatalf("write secret: %v", err)
	}

	info, err := os.Stat(secretPath)
	if err != nil {
		t.Fatalf("stat secret: %v", err)
	}

	// Craft a path with explicit parent traversals that resolve to the outside file.
	base := repoRoot + string(filepath.Separator) + ".." + string(filepath.Separator) + "outside" + string(filepath.Separator) + "secret.yaml"
	if _, err := delivery.LoadManifests(repoRoot, base, info); err == nil {
		t.Fatalf("expected LoadManifests to reject relative traversal")
	}
}
