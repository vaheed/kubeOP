package logging_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"kubeop/internal/logging"
)

func TestEnsureFileValidatesAbsolutePaths(t *testing.T) {
	dir := t.TempDir()
	goodPath := filepath.Join(dir, "projects", "alpha", "project.log")
	clean, err := logging.ValidateLogPathForTest(goodPath)
	if err != nil {
		t.Fatalf("validate good path: %v", err)
	}
	if clean != goodPath {
		t.Fatalf("expected clean path to equal original: %q vs %q", clean, goodPath)
	}
	touched, err := logging.TouchLogFileForTest(dir, "projects", "alpha", "project.log")
	if err != nil {
		t.Fatalf("touch log file: %v", err)
	}
	if touched != clean {
		t.Fatalf("expected touched path %q to equal clean path %q", touched, clean)
	}
	if _, err := os.Stat(touched); err != nil {
		t.Fatalf("expected file created, got %v", err)
	}

	if _, err := logging.ValidateLogPathForTest("relative.log"); err == nil {
		t.Fatalf("expected relative path to be rejected")
	}

	dirtyPath := goodPath + string(os.PathSeparator) + ".." + string(os.PathSeparator) + "escape.log"
	if _, err := logging.ValidateLogPathForTest(dirtyPath); err == nil {
		t.Fatalf("expected path normalisation to be rejected")
	}
}

func TestTouchLogFileForTestRejectsTraversal(t *testing.T) {
	dir := t.TempDir()
	root := filepath.Join(dir, "logs")
	for _, tc := range []struct {
		name  string
		parts []string
	}{
		{name: "dotdot", parts: []string{"projects", "alpha", "..", "escape.log"}},
		{name: "embedded", parts: []string{"projects", "alpha/../escape.log"}},
		{name: "empty", parts: []string{"projects", "", "escape.log"}},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			if _, err := logging.TouchLogFileForTest(root, tc.parts...); err == nil {
				t.Fatalf("expected traversal attempt %q to be rejected", strings.Join(tc.parts, string(os.PathSeparator)))
			}
		})
	}
}

func TestTouchLogFileForTestStaysWithinRoot(t *testing.T) {
	dir := t.TempDir()
	root := filepath.Join(dir, "logs")
	touched, err := logging.TouchLogFileForTest(root, "projects", "beta", "project.log")
	if err != nil {
		t.Fatalf("touch log file: %v", err)
	}
	if !strings.HasPrefix(touched, root+string(os.PathSeparator)) && touched != root {
		t.Fatalf("expected touched path %q to be within root %q", touched, root)
	}
	if _, err := os.Stat(touched); err != nil {
		t.Fatalf("expected touched file to exist: %v", err)
	}
}
