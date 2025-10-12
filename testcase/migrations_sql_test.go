package testcase

import (
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestMigrationsDoNotUseInvalidAlterTableSyntax(t *testing.T) {
	root := repoRoot(t)
	dir := filepath.Join(root, "internal", "store", "migrations")
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read migrations dir: %v", err)
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".up.sql") {
			continue
		}
		path := filepath.Join(dir, entry.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", entry.Name(), err)
		}
		if strings.Contains(string(data), "ALTER TABLE IF NOT EXISTS") {
			t.Fatalf("migration %s contains unsupported ALTER TABLE IF NOT EXISTS syntax", entry.Name())
		}
	}
}

func repoRoot(t testing.TB) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot resolve caller path")
	}
	// testcase/ -> repo root is one level up from testcase directory
	dir := filepath.Dir(file)
	root, err := filepath.Abs(filepath.Join(dir, ".."))
	if err != nil {
		t.Fatalf("resolve repo root: %v", err)
	}
	// Ensure directory exists
	if _, err := os.Stat(root); err != nil {
		t.Fatalf("stat repo root: %v", err)
	}
	return root
}

func TestMigrationsDirIsRegular(t *testing.T) {
	root := repoRoot(t)
	dir := filepath.Join(root, "internal", "store", "migrations")
	info, err := os.Stat(dir)
	if err != nil {
		t.Fatalf("stat migrations dir: %v", err)
	}
	if info.Mode()&fs.ModeSymlink != 0 {
		t.Fatalf("migrations dir should not be symlink: %s", dir)
	}
}
