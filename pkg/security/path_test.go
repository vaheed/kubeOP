package security_test

import (
	"os"
	"path/filepath"
	"testing"

	"kubeop/pkg/security"
)

func TestNormalizeRepoPath(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		{name: "empty", input: "", want: ""},
		{name: "dot", input: ".", want: ""},
		{name: "simple", input: "manifests/base", want: "manifests/base"},
		{name: "trim", input: "  overlays/dev  ", want: "overlays/dev"},
		{name: "reject absolute", input: "/etc/passwd", wantErr: true},
		{name: "reject parent", input: "../secret", wantErr: true},
		{name: "reject encoded parent", input: "%2e%2e/escape", wantErr: true},
		{name: "reject backslash", input: "..\\escape", wantErr: true},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := security.NormalizeRepoPath(tc.input)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error for %q", tc.input)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.want {
				t.Fatalf("NormalizeRepoPath(%q) = %q, want %q", tc.input, got, tc.want)
			}
		})
	}
}

func TestWithinRepo(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	nested := filepath.Join(root, "charts", "demo")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatalf("mkdir nested: %v", err)
	}
	if _, err := security.WithinRepo(root, nested); err != nil {
		t.Fatalf("expected absolute path inside repo to pass: %v", err)
	}
	if _, err := security.WithinRepo(root, "charts/demo"); err != nil {
		t.Fatalf("expected relative path inside repo to pass: %v", err)
	}
	if _, err := security.WithinRepo(root, filepath.Join(root, "..", "escape")); err == nil {
		t.Fatalf("expected escape to be rejected")
	}
}

func FuzzWithinRepo(f *testing.F) {
	f.Add("manifests/base")
	f.Add("../escape")
	f.Fuzz(func(t *testing.T, candidate string) {
		root := t.TempDir()
		_, _ = security.WithinRepo(root, candidate)
	})
}
