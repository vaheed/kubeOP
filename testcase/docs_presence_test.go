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
    // Ensure the kubeconfig guide exists after the VitePress migration
    guidePath := filepath.Join(docs, "guides", "kubeconfig-and-rbac.md")
    if _, err := os.Stat(guidePath); err != nil {
            t.Fatalf("kubeconfig guide missing: %v", err)
    }
}
