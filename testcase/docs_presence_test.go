package testcase

import (
    "os"
    "testing"
)

func TestDocumentsFolderPresent(t *testing.T) {
    if _, err := os.Stat("documents"); err != nil {
        t.Fatalf("documents/ folder missing: %v", err)
    }
}

