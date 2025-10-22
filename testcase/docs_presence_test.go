package testcase

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestDocumentsFolderPresent(t *testing.T) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot resolve caller path")
	}
	root := filepath.Dir(filepath.Dir(file))
	docsDir := filepath.Join(root, "docs")
	fi, err := os.Stat(docsDir)
	if err != nil || !fi.IsDir() {
		t.Fatalf("docs/ folder missing at %s: %v", docsDir, err)
	}

	requiredDocs := []string{
		"QUICKSTART.md",
		"INSTALL.md",
		"ENVIRONMENT.md",
		"ARCHITECTURE.md",
		"STYLEGUIDE.md",
	}

	for _, rel := range requiredDocs {
		path := filepath.Join(docsDir, rel)
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("required doc missing (%s): %v", rel, err)
		}
	}
}
