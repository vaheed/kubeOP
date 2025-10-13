package testcase

import (
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"kubeop/internal/api"
	"kubeop/internal/config"
	"kubeop/internal/logging"
	"kubeop/internal/service"
	"kubeop/internal/store"
)

func TestProjectLogsEndpoint_ReadsFileAndTail(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("LOGS_ROOT", tmp)
	mgr, err := logging.Setup(logging.Metadata{})
	if err != nil {
		t.Fatalf("logging setup: %v", err)
	}
	t.Cleanup(func() {
		mgr.Sync()
		if fm := logging.Files(); fm != nil {
			_ = fm.Close()
		}
	})

	if fm := logging.Files(); fm != nil {
		if err := fm.EnsureProject("proj-logs", nil); err != nil {
			t.Fatalf("ensure project logs: %v", err)
		}
	}
	path, err := logging.ProjectLogPath("proj-logs")
	if err != nil {
		t.Fatalf("project log path: %v", err)
	}
	content := "line1\nline2\nline3\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write log file: %v", err)
	}

	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	cfg := &config.Config{DisableAuth: true, KcfgEncryptionKey: "unit-test", EventsDBEnabled: true}
	svc, err := service.New(cfg, store.NewWithDB(db), nil)
	if err != nil {
		t.Fatalf("service.New: %v", err)
	}

	router := api.NewRouter(cfg, svc)

	t.Run("full", func(t *testing.T) {
		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/v1/projects/proj-logs/logs", nil)
		router.ServeHTTP(rr, req)
		if rr.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", rr.Code)
		}
		if rr.Body.String() != content {
			t.Fatalf("expected body %q, got %q", content, rr.Body.String())
		}
	})

	t.Run("tail", func(t *testing.T) {
		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/v1/projects/proj-logs/logs?tail=2", nil)
		router.ServeHTTP(rr, req)
		if rr.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", rr.Code)
		}
		want := "line2\nline3\n"
		if rr.Body.String() != want {
			t.Fatalf("expected tail body %q, got %q", want, rr.Body.String())
		}
	})

	t.Run("invalid tail", func(t *testing.T) {
		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/v1/projects/proj-logs/logs?tail=oops", nil)
		router.ServeHTTP(rr, req)
		if rr.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d", rr.Code)
		}
	})

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("sql expectations: %v", err)
	}
}
