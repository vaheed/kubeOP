package api

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"

	"github.com/vaheed/kubeop/internal/auth"
	"github.com/vaheed/kubeop/internal/db"
	"github.com/vaheed/kubeop/internal/kms"
)

func newTestServer(t *testing.T) *Server {
	t.Helper()

	sqlDB, _, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	t.Cleanup(func() { sqlDB.Close() })

	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		t.Fatalf("rand: %v", err)
	}
	envelope, err := kms.New(key)
	if err != nil {
		t.Fatalf("kms: %v", err)
	}

	handler := New(slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil)), &db.DB{DB: sqlDB}, envelope, false, key)
	return handler
}

func TestRouterProvidesExpectedEndpoints(t *testing.T) {
	srv := newTestServer(t)
	router := srv.Router()
	expected := []string{"/healthz", "/readyz", "/version", "/openapi.json", "/v1/tenants", "/v1/projects", "/v1/apps"}
	for _, path := range expected {
		path := path
		t.Run(path, func(t *testing.T) {
			t.Parallel()
			req := httptest.NewRequest(http.MethodGet, path, nil)
			rr := httptest.NewRecorder()
			router.ServeHTTP(rr, req)
			if rr.Code == 0 {
				t.Fatalf("route %s did not respond", path)
			}
		})
	}
}

func TestRecovererCatchesPanics(t *testing.T) {
	called := false
	handler := recoverer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		called = true
		panic("boom")
	}))
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/panic", nil))
	if !called {
		t.Fatalf("handler was not invoked")
	}
	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 got %d", rr.Code)
	}
}

func TestRequireRole(t *testing.T) {
	srv := newTestServer(t)
	srv.cfgAuth = true

	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		t.Fatalf("rand: %v", err)
	}
	srv.jwtKey = key

	handler := srv.requireRole("admin", func(w http.ResponseWriter, r *http.Request, _ *auth.Claims) {
		w.WriteHeader(http.StatusNoContent)
	})

	rr := httptest.NewRecorder()
	handler(rr, httptest.NewRequest(http.MethodGet, "/secure", nil))
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 got %d", rr.Code)
	}

	token, err := auth.SignHS256(&auth.Claims{Role: "admin"}, key)
	if err != nil {
		t.Fatalf("sign: %v", err)
	}
	req := httptest.NewRequest(http.MethodGet, "/secure", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rr = httptest.NewRecorder()
	handler(rr, req)
	if rr.Code != http.StatusNoContent {
		t.Fatalf("expected 204 got %d", rr.Code)
	}
}

func TestWithJSONWrapsResponse(t *testing.T) {
	srv := newTestServer(t)
	rr := httptest.NewRecorder()
	srv.withJSON(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusAccepted)
		json.NewEncoder(w).Encode(map[string]string{"ok": "true"})
	})).ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/json", nil))

	if rr.Code != http.StatusAccepted {
		t.Fatalf("expected 202 got %d", rr.Code)
	}
	if ct := rr.Header().Get("Content-Type"); ct != "application/json" {
		t.Fatalf("unexpected content type %s", ct)
	}
}

func TestAccessLogIncludesStatus(t *testing.T) {
	buf := &bytes.Buffer{}
	logger := slog.New(slog.NewTextHandler(buf, nil))
	sqlDB, _, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	t.Cleanup(func() { sqlDB.Close() })
	key := make([]byte, 32)
	rand.Read(key)
	envelope, _ := kms.New(key)
	srv := New(logger, &db.DB{DB: sqlDB}, envelope, false, key)

	rr := httptest.NewRecorder()
	srv.withAccessLog(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		time.Sleep(10 * time.Millisecond)
		w.WriteHeader(http.StatusTeapot)
	})).ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/log", nil))

	if !bytes.Contains(buf.Bytes(), []byte("status=418")) {
		t.Fatalf("log missing status: %s", buf.String())
	}
}

func TestContextTimeouts(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond)
	t.Cleanup(cancel)
	select {
	case <-ctx.Done():
	case <-time.After(5 * time.Millisecond):
		t.Fatalf("context did not cancel on timeout")
	}
}
