package testcase

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestDocumentsFolderPresent(t *testing.T) {
	// Resolve repo root based on this test file location (../)
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot resolve caller path")
	}
	root := filepath.Dir(filepath.Dir(file))
	docs := filepath.Join(root, "docs")
	fi, err := os.Stat(docs)
	if err != nil || !fi.IsDir() {
		t.Fatalf("docs/ folder missing at %s: %v", docs, err)
	}
    required := []string{
            "README.md",
            "ARCHITECTURE.md",
            "DEPLOY.md",
            "API.md",
            "TENANTS.md",
            "LOGS_EVENTS.md",
            "SECURITY.md",
            "ROADMAP.md",
            "SUMMARY.md",
            "CHANGELOG.md",
            "index.md",
    }
    for _, file := range required {
            if _, err := os.Stat(filepath.Join(docs, file)); err != nil {
                    t.Fatalf("%s missing: %v", file, err)
            }
    }
}
