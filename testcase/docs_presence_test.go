package testcase

import (
    "path/filepath"
    "runtime"
    "os"
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
    // Also ensure KUBECONFIG.md exists to document kubeconfig behavior
    if _, err := os.Stat(filepath.Join(docs, "KUBECONFIG.md")); err != nil {
        t.Fatalf("KUBECONFIG.md missing: %v", err)
    }
}
