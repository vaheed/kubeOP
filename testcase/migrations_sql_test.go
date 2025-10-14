package testcase

import (
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
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

func TestMigrationVersionsAreSequential(t *testing.T) {
	root := repoRoot(t)
	dir := filepath.Join(root, "internal", "store", "migrations")
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read migrations dir: %v", err)
	}

	versions := make(map[int]string)
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".up.sql") {
			continue
		}
		name := entry.Name()
		parts := strings.SplitN(name, "_", 2)
		if len(parts) != 2 {
			t.Fatalf("migration %s must start with zero-padded version and underscore", name)
		}
		version, err := strconv.Atoi(parts[0])
		if err != nil {
			t.Fatalf("parse migration version from %s: %v", name, err)
		}
		if version <= 0 {
			t.Fatalf("migration %s has non-positive version %d", name, version)
		}
		if prev, dup := versions[version]; dup {
			t.Fatalf("duplicate migration version %04d: %s and %s", version, prev, name)
		}
		downName := strings.TrimSuffix(name, ".up.sql") + ".down.sql"
		if _, err := os.Stat(filepath.Join(dir, downName)); err != nil {
			t.Fatalf("missing down migration for %s: %v", name, err)
		}
		versions[version] = name
	}

	if len(versions) == 0 {
		t.Fatal("no migrations found")
	}

	nums := make([]int, 0, len(versions))
	for v := range versions {
		nums = append(nums, v)
	}
	sort.Ints(nums)
	if nums[0] != 1 {
		t.Fatalf("first migration must start at 0001, found %04d", nums[0])
	}
	for i := 1; i < len(nums); i++ {
		expected := nums[i-1] + 1
		if nums[i] != expected {
			t.Fatalf("migration sequence gap: expected %04d after %04d (saw %04d)", expected, nums[i-1], nums[i])
		}
	}

	// Log a helpful summary when running tests with -v to ease debugging.
	if testing.Verbose() {
		for _, v := range nums {
			t.Logf("migration %04d: %s", v, versions[v])
		}
	}
}
