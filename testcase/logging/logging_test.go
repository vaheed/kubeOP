package logging_test

import (
	"bufio"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"go.uber.org/zap"
	httpmw "kubeop/internal/http/middleware"
	"kubeop/internal/logging"
)

func TestReadConfigParsesEnvironment(t *testing.T) {
	t.Setenv("LOG_LEVEL", "debug")
	t.Setenv("LOG_DIR", "/tmp/kubeop")
	t.Setenv("LOGS_ROOT", "/tmp/root")
	t.Setenv("LOG_MAX_SIZE_MB", "99")
	t.Setenv("LOG_MAX_BACKUPS", "9")
	t.Setenv("LOG_MAX_AGE_DAYS", "21")
	t.Setenv("LOG_COMPRESS", "false")
	t.Setenv("AUDIT_ENABLED", "false")
	t.Setenv("CLUSTER_ID", "cluster-123")

	cfg := logging.ReadConfig()
	if cfg.Level != "debug" || cfg.Dir != "/tmp/kubeop" {
		t.Fatalf("unexpected config: %+v", cfg)
	}
	if cfg.MaxSizeMB != 99 || cfg.MaxBackups != 9 || cfg.MaxAgeDays != 21 {
		t.Fatalf("rotation config mismatch: %+v", cfg)
	}
	if cfg.Compress {
		t.Fatalf("expected compression disabled")
	}
	if cfg.AuditEnable {
		t.Fatalf("expected audit disabled")
	}
	if cfg.ClusterID != "cluster-123" {
		t.Fatalf("expected cluster id propagated, got %q", cfg.ClusterID)
	}
	if cfg.Root != "/tmp/root" {
		t.Fatalf("expected logs root propagated, got %q", cfg.Root)
	}
}

func TestReadConfigUsesLogsRootWhenDirMissing(t *testing.T) {
	t.Setenv("LOGS_ROOT", "/var/lib/kubeop")
	t.Setenv("LOG_LEVEL", "info")
	cfg := logging.ReadConfig()
	if cfg.Dir != "/var/lib/kubeop" || cfg.Root != "/var/lib/kubeop" {
		t.Fatalf("expected dir/root to follow logs root, got %#v", cfg)
	}
}

func TestSetupWritesJSONLogs(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("LOG_DIR", dir)
	t.Setenv("LOGS_ROOT", dir)
	t.Setenv("LOG_COMPRESS", "false")
	t.Setenv("AUDIT_ENABLED", "true")

	mgr, err := logging.Setup(logging.Metadata{Version: "test", Commit: "abc123"})
	if err != nil {
		t.Fatalf("setup logging: %v", err)
	}
	logging.L().Info("unit-test", zap.String("component", "logging"))
	logging.Audit().Info("audit-test", zap.String("component", "logging"))
	mgr.Sync()

	appLog := filepath.Join(dir, "app.log")
	if _, err := os.Stat(appLog); err != nil {
		t.Fatalf("expected app log: %v", err)
	}
	auditLog := filepath.Join(dir, "audit.log")
	if _, err := os.Stat(auditLog); err != nil {
		t.Fatalf("expected audit log: %v", err)
	}

	entry := readLastJSONLine(t, appLog)
	if entry["service"] != "kubeop" {
		t.Fatalf("expected service field, got %v", entry["service"])
	}
	if _, ok := entry["ts"]; !ok {
		t.Fatalf("expected timestamp field: %+v", entry)
	}
	if entry["level"] == "" {
		t.Fatalf("expected level field")
	}
}

func TestAccessLogWritesRequestEntry(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("LOG_DIR", dir)
	t.Setenv("LOGS_ROOT", dir)
	t.Setenv("LOG_COMPRESS", "false")
	t.Setenv("AUDIT_ENABLED", "false")

	mgr, err := logging.Setup(logging.Metadata{Version: "test", Commit: "abcdef"})
	if err != nil {
		t.Fatalf("setup logging: %v", err)
	}

	handler := httpmw.AccessLog(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte("ok"))
	}))

	req := httptest.NewRequest(http.MethodPost, "/api/test", strings.NewReader("payload"))
	req.Header.Set("User-Agent", "kubeop-tests")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	if got := rr.Result().Header.Get("X-Request-Id"); got == "" {
		t.Fatalf("expected request id header")
	}

	mgr.Sync()
	entry := readLastJSONLine(t, filepath.Join(dir, "app.log"))
	if entry["msg"] != "http_request" {
		t.Fatalf("expected http_request log, got %v", entry["msg"])
	}
	if entry["request_id"] == "" {
		t.Fatalf("missing request_id field: %+v", entry)
	}
	if entry["status"].(float64) != float64(http.StatusCreated) {
		t.Fatalf("unexpected status: %+v", entry["status"])
	}
}

func TestAuditLogRedactsSecrets(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("LOG_DIR", dir)
	t.Setenv("LOGS_ROOT", dir)
	t.Setenv("LOG_COMPRESS", "false")
	t.Setenv("AUDIT_ENABLED", "true")

	mgr, err := logging.Setup(logging.Metadata{Version: "test", Commit: "fedcba"})
	if err != nil {
		t.Fatalf("setup logging: %v", err)
	}

	handler := httpmw.AccessLog(httpmw.AuditLog(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})))

	req := httptest.NewRequest(http.MethodPost, "/v1/projects/demo/secrets/reset", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	mgr.Sync()
	entry := readLastJSONLine(t, filepath.Join(dir, "audit.log"))
	if entry["resource_id"] != "demo/redacted/reset" {
		t.Fatalf("expected redacted resource_id, got %v", entry["resource_id"])
	}
	if entry["verb"] != "POST" {
		t.Fatalf("expected verb POST, got %v", entry["verb"])
	}
}

func TestSetupRedactsTokens(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("LOG_DIR", dir)
	t.Setenv("LOGS_ROOT", dir)
	t.Setenv("LOG_COMPRESS", "false")
	t.Setenv("AUDIT_ENABLED", "false")

	mgr, err := logging.Setup(logging.Metadata{Version: "test", Commit: "redact"})
	if err != nil {
		t.Fatalf("setup logging: %v", err)
	}
	logging.L().Info("token=my-secret authorization=Bearer-12345")
	mgr.Sync()

	by, err := os.ReadFile(filepath.Join(dir, "app.log"))
	if err != nil {
		t.Fatalf("read app log: %v", err)
	}
	content := string(by)
	if strings.Contains(content, "my-secret") {
		t.Fatalf("expected token to be redacted: %s", content)
	}
	if !strings.Contains(content, "token=REDACTED") {
		t.Fatalf("expected token redaction, got: %s", content)
	}
	if !strings.Contains(content, "authorization=REDACTED") {
		t.Fatalf("expected authorization redaction, got: %s", content)
	}
}

func TestFileManagerProjectLogs(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("LOG_DIR", dir)
	t.Setenv("LOGS_ROOT", dir)
	t.Setenv("LOG_COMPRESS", "false")
	t.Setenv("AUDIT_ENABLED", "false")

	mgr, err := logging.Setup(logging.Metadata{Version: "test", Commit: "files"})
	if err != nil {
		t.Fatalf("setup logging: %v", err)
	}
	fm := logging.Files()
	if fm == nil {
		t.Fatalf("expected file manager available")
	}
	if err := fm.EnsureProject("proj1", []string{"appA"}); err != nil {
		t.Fatalf("ensure project: %v", err)
	}
	if err := fm.EnsureApp("proj1", "appB"); err != nil {
		t.Fatalf("ensure app: %v", err)
	}

	logging.ProjectLogger("proj1").Info("project_event", zap.String("detail", "init"))
	logging.ProjectEventsLogger("proj1").Info("project_event", zap.String("detail", "event"))
	logging.AppLogger("proj1", "appA").Info("app_event", zap.String("status", "ok"))
	logging.AppErrorLogger("proj1", "appA").Error("app_failed", zap.Error(errors.New("boom")))
	mgr.Sync()

	base := filepath.Join(dir, "projects", "proj1")
	paths := []string{
		filepath.Join(base, "project.log"),
		filepath.Join(base, "events.jsonl"),
		filepath.Join(base, "apps", "appA", "app.log"),
		filepath.Join(base, "apps", "appA", "app.err.log"),
		filepath.Join(base, "apps", "appB", "app.log"),
		filepath.Join(base, "apps", "appB", "app.err.log"),
	}
	for _, p := range paths {
		if _, err := os.Stat(p); err != nil {
			t.Fatalf("expected log file %s: %v", p, err)
		}
	}
	entry := readLastJSONLine(t, filepath.Join(base, "project.log"))
	if entry["project_id"] != "proj1" {
		t.Fatalf("project log missing project_id: %+v", entry)
	}
	if entry["msg"] != "project_event" {
		t.Fatalf("unexpected project log msg: %+v", entry)
	}
	appEntry := readLastJSONLine(t, filepath.Join(base, "apps", "appA", "app.log"))
	if appEntry["app_id"] != "appA" {
		t.Fatalf("app log missing app_id: %+v", appEntry)
	}
}

func TestFileManagerRejectsUnsafeSegments(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("LOG_DIR", dir)
	t.Setenv("LOGS_ROOT", dir)
	t.Setenv("LOG_COMPRESS", "false")
	t.Setenv("AUDIT_ENABLED", "false")

	mgr, err := logging.Setup(logging.Metadata{Version: "test", Commit: "sanitize"})
	if err != nil {
		t.Fatalf("setup logging: %v", err)
	}
	t.Cleanup(func() {
		mgr.Sync()
	})

	fm := logging.Files()
	if fm == nil {
		t.Fatalf("expected file manager available")
	}
	if fmRoot := fm.Root(); fmRoot == "" || !filepath.IsAbs(fmRoot) {
		t.Fatalf("expected absolute logs root, got %q", fmRoot)
	}
	if err := fm.EnsureProject("../escape", nil); err == nil {
		t.Fatalf("expected traversal project id to be rejected")
	}
	if err := fm.EnsureProject("proj!id", nil); err == nil {
		t.Fatalf("expected punctuation-heavy project id to be rejected")
	}
	if err := fm.EnsureProject("  safe  ", []string{"  app  "}); err != nil {
		t.Fatalf("ensure project with whitespace ids: %v", err)
	}
	if err := fm.EnsureApp("safe", "../bad"); err == nil {
		t.Fatalf("expected traversal app id to be rejected")
	}
	if err := fm.EnsureApp("safe", "bad!id"); err == nil {
		t.Fatalf("expected punctuation-heavy app id to be rejected")
	}

	base := filepath.Join(dir, "projects", "safe")
	if _, err := os.Stat(base); err != nil {
		t.Fatalf("expected sanitized project directory: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "projects", "  safe  ")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("unexpected unsanitized project directory created: %v", err)
	}
	if _, err := os.Stat(filepath.Join(base, "apps", "app", "app.log")); err != nil {
		t.Fatalf("expected sanitized app log: %v", err)
	}

	logging.ProjectLogger("../escape").Info("should not log")
	logging.ProjectLogger("proj!id").Info("should not log")
	logging.AppLogger("safe", "app").Info("app_ok")
	logging.AppLogger("safe", "../bad").Info("app_bad")
	logging.AppLogger("safe", "bad!id").Info("app_bad")
	mgr.Sync()

	projEntries, err := os.ReadDir(filepath.Join(dir, "projects"))
	if err != nil {
		t.Fatalf("read projects dir: %v", err)
	}
	for _, entry := range projEntries {
		if entry.Name() == "escape" || entry.Name() == "proj!id" {
			t.Fatalf("unexpected project directory created: %s", entry.Name())
		}
	}
	appEntries, err := os.ReadDir(filepath.Join(base, "apps"))
	if err != nil {
		t.Fatalf("read apps dir: %v", err)
	}
	for _, entry := range appEntries {
		if entry.Name() == "bad" || entry.Name() == "bad!id" {
			t.Fatalf("unexpected app directory created: %s", entry.Name())
		}
	}

	appEntry := readLastJSONLine(t, filepath.Join(base, "apps", "app", "app.log"))
	if appEntry["app_id"] != "app" {
		t.Fatalf("expected sanitized app_id in logs, got %+v", appEntry)
	}
}

func TestFileManagerNormalisesRoot(t *testing.T) {
	base := t.TempDir()
	dirtyRoot := filepath.Join(base, "nested") + string(os.PathSeparator) + ".." + string(os.PathSeparator) + "logs-root" + string(os.PathSeparator) + "."
	t.Setenv("LOG_DIR", dirtyRoot)
	t.Setenv("LOGS_ROOT", dirtyRoot)
	t.Setenv("LOG_COMPRESS", "false")
	t.Setenv("AUDIT_ENABLED", "false")

	mgr, err := logging.Setup(logging.Metadata{Version: "test", Commit: "normalize"})
	if err != nil {
		t.Fatalf("setup logging: %v", err)
	}
	t.Cleanup(func() {
		mgr.Sync()
	})

	fm := logging.Files()
	if fm == nil {
		t.Fatalf("expected file manager available")
	}
	expectedRoot := filepath.Clean(dirtyRoot)
	expectedAbs, err := filepath.Abs(expectedRoot)
	if err != nil {
		t.Fatalf("resolve abs root: %v", err)
	}
	if fm.Root() != expectedAbs {
		t.Fatalf("expected cleaned logs root %q, got %q", expectedAbs, fm.Root())
	}
	if err := fm.EnsureProject("demo", nil); err != nil {
		t.Fatalf("ensure project: %v", err)
	}
	projectLog := filepath.Join(expectedAbs, "projects", "demo", "project.log")
	if _, err := os.Stat(projectLog); err != nil {
		t.Fatalf("expected project log, got %v", err)
	}
	prefix := expectedAbs + string(os.PathSeparator)
	if !strings.HasPrefix(projectLog, prefix) {
		t.Fatalf("expected project log to stay under %q, got %q", expectedAbs, projectLog)
	}
}

func readLastJSONLine(t *testing.T, path string) map[string]any {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("open log: %v", err)
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	var line string
	for scanner.Scan() {
		if strings.TrimSpace(scanner.Text()) == "" {
			continue
		}
		line = scanner.Text()
	}
	if err := scanner.Err(); err != nil {
		t.Fatalf("scan log: %v", err)
	}
	if line == "" {
		t.Fatalf("no log lines in %s", path)
	}
	var out map[string]any
	if err := json.Unmarshal([]byte(line), &out); err != nil {
		t.Fatalf("unmarshal log line: %v (line=%s)", err, line)
	}
	return out
}
