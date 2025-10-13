package logging_test

import (
	"os"
	"path/filepath"
	"testing"

	"kubeop/internal/logging"
)

func TestEnsureFileValidatesAbsolutePaths(t *testing.T) {
	dir := t.TempDir()
	goodPath := filepath.Join(dir, "good.log")
	clean, err := logging.ValidateLogPathForTest(goodPath)
	if err != nil {
		t.Fatalf("validate good path: %v", err)
	}
	if clean != goodPath {
		t.Fatalf("expected clean path to equal original: %q vs %q", clean, goodPath)
	}
	if err := logging.TouchLogFileForTest(goodPath); err != nil {
		t.Fatalf("touch log file: %v", err)
	}
	if _, err := os.Stat(goodPath); err != nil {
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
