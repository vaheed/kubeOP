package logging_test

import (
	"bufio"
	"encoding/json"
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
}

func TestSetupWritesJSONLogs(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("LOG_DIR", dir)
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
