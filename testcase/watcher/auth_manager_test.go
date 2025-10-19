package watcher_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"kubeop/internal/state"
	authmanager "kubeop/internal/watcher/authmanager"

	"go.uber.org/zap"
)

func TestAuthManagerForceRefreshPrefersRegister(t *testing.T) {
	t.Parallel()

	var registerCalls atomic.Int64
	var refreshCalls atomic.Int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/register":
			count := registerCalls.Add(1)
			if got := r.Header.Get("Authorization"); got != "Bearer bootstrap-token" {
				t.Fatalf("expected bootstrap token, got %q", got)
			}
			respondWithCredentials(t, w, fmt.Sprintf("access-register-%d", count), fmt.Sprintf("refresh-register-%d", count))
		case "/refresh":
			count := refreshCalls.Add(1)
			respondWithCredentials(t, w, fmt.Sprintf("access-refresh-%d", count), fmt.Sprintf("refresh-refresh-%d", count))
		default:
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
	}))
	t.Cleanup(srv.Close)

	dir := t.TempDir()
	store, err := state.Open(filepath.Join(dir, "state.db"))
	if err != nil {
		t.Fatalf("open state store: %v", err)
	}
	t.Cleanup(func() { store.Close() })

	cfg := authmanager.Config{
		ClusterID:      "cluster-123",
		RegisterURL:    srv.URL + "/register",
		RefreshURL:     srv.URL + "/refresh",
		BootstrapToken: "bootstrap-token",
	}

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	mgr := authmanager.New(cfg, store, nil, zap.NewNop())
	if err := mgr.Initialize(ctx); err != nil {
		t.Fatalf("initialise auth manager: %v", err)
	}

	beforeToken := mgr.AccessToken()
	beforeRegister := registerCalls.Load()
	beforeRefresh := refreshCalls.Load()

	if err := mgr.ForceRefresh(ctx); err != nil {
		t.Fatalf("force refresh: %v", err)
	}

	if got := registerCalls.Load(); got != beforeRegister+1 {
		t.Fatalf("expected register to run once during force refresh, before=%d after=%d", beforeRegister, got)
	}
	if got := refreshCalls.Load(); got != beforeRefresh {
		t.Fatalf("expected refresh not to run, before=%d after=%d", beforeRefresh, got)
	}

	afterToken := mgr.AccessToken()
	if afterToken == beforeToken {
		t.Fatalf("expected access token to change after forced refresh")
	}
	if !strings.HasPrefix(afterToken, "access-register-") {
		t.Fatalf("expected access token from register, got %q", afterToken)
	}

	creds, ok, err := store.LoadCredentials()
	if err != nil {
		t.Fatalf("load credentials: %v", err)
	}
	if !ok {
		t.Fatalf("expected credentials persisted")
	}
	if !strings.HasPrefix(creds.AccessToken, "access-register-") {
		t.Fatalf("expected persisted access token from register, got %q", creds.AccessToken)
	}
	if !strings.HasPrefix(creds.RefreshToken, "refresh-register-") {
		t.Fatalf("expected persisted refresh token from register, got %q", creds.RefreshToken)
	}
}

func TestAuthManagerForceRefreshFallsBackToRefresh(t *testing.T) {
	t.Parallel()

	var registerCalls atomic.Int64
	var refreshCalls atomic.Int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/register":
			count := registerCalls.Add(1)
			if count == 1 {
				respondWithCredentials(t, w, "initial-access", "initial-refresh")
				return
			}
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte("boom"))
		case "/refresh":
			count := refreshCalls.Add(1)
			respondWithCredentials(t, w, fmt.Sprintf("access-refresh-%d", count), fmt.Sprintf("refresh-refresh-%d", count))
		default:
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
	}))
	t.Cleanup(srv.Close)

	dir := t.TempDir()
	store, err := state.Open(filepath.Join(dir, "state.db"))
	if err != nil {
		t.Fatalf("open state store: %v", err)
	}
	t.Cleanup(func() { store.Close() })

	cfg := authmanager.Config{
		ClusterID:      "cluster-123",
		RegisterURL:    srv.URL + "/register",
		RefreshURL:     srv.URL + "/refresh",
		BootstrapToken: "bootstrap-token",
	}

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	mgr := authmanager.New(cfg, store, nil, zap.NewNop())
	if err := mgr.Initialize(ctx); err != nil {
		t.Fatalf("initialise auth manager: %v", err)
	}

	beforeRegister := registerCalls.Load()
	beforeRefresh := refreshCalls.Load()

	if err := mgr.ForceRefresh(ctx); err != nil {
		t.Fatalf("force refresh: %v", err)
	}

	if got := registerCalls.Load(); got != beforeRegister+1 {
		t.Fatalf("expected one register attempt during forced refresh, before=%d after=%d", beforeRegister, got)
	}
	if got := refreshCalls.Load(); got != beforeRefresh+1 {
		t.Fatalf("expected fallback refresh to run once, before=%d after=%d", beforeRefresh, got)
	}

	creds, ok, err := store.LoadCredentials()
	if err != nil {
		t.Fatalf("load credentials: %v", err)
	}
	if !ok {
		t.Fatalf("expected credentials persisted")
	}
	if !strings.HasPrefix(creds.AccessToken, "access-refresh-") {
		t.Fatalf("expected persisted access token from refresh, got %q", creds.AccessToken)
	}
	if !strings.HasPrefix(creds.RefreshToken, "refresh-refresh-") {
		t.Fatalf("expected persisted refresh token from refresh, got %q", creds.RefreshToken)
	}
}

func TestAuthManagerUnauthorizedThrottle(t *testing.T) {
	t.Parallel()

	var registerCalls atomic.Int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/register":
			registerCalls.Add(1)
			respondWithCredentials(t, w, "access-token", "refresh-token")
		case "/refresh":
			t.Fatalf("refresh should not be called during forced throttle test")
		default:
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
	}))
	t.Cleanup(srv.Close)

	dir := t.TempDir()
	store, err := state.Open(filepath.Join(dir, "state.db"))
	if err != nil {
		t.Fatalf("open state store: %v", err)
	}
	t.Cleanup(func() { store.Close() })

	cfg := authmanager.Config{
		ClusterID:      "cluster-123",
		RegisterURL:    srv.URL + "/register",
		RefreshURL:     srv.URL + "/refresh",
		BootstrapToken: "bootstrap-token",
	}

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	mgr := authmanager.New(cfg, store, nil, zap.NewNop())
	mgr.SetUnauthorizedCooldown(50 * time.Millisecond)
	if err := mgr.Initialize(ctx); err != nil {
		t.Fatalf("initialise auth manager: %v", err)
	}

	baseline := registerCalls.Load()

	if err := mgr.ForceRefreshAfterUnauthorized(ctx); err != nil {
		t.Fatalf("first forced refresh: %v", err)
	}
	if got := registerCalls.Load(); got != baseline+1 {
		t.Fatalf("expected forced refresh to register once, baseline=%d got=%d", baseline, got)
	}

	start := time.Now()
	// Immediate retry should be skipped because of the throttle while waiting for the cooldown.
	if err := mgr.ForceRefreshAfterUnauthorized(ctx); err != nil {
		t.Fatalf("second forced refresh within cooldown: %v", err)
	}
	if elapsed := time.Since(start); elapsed < 40*time.Millisecond {
		t.Fatalf("expected cooldown wait of at least ~40ms, got %s", elapsed)
	}
	if got := registerCalls.Load(); got != baseline+1 {
		t.Fatalf("expected throttle to skip register during cooldown, baseline=%d got=%d", baseline, got)
	}

	time.Sleep(75 * time.Millisecond)

	if err := mgr.ForceRefreshAfterUnauthorized(ctx); err != nil {
		t.Fatalf("forced refresh after cooldown: %v", err)
	}
	if got := registerCalls.Load(); got != baseline+2 {
		t.Fatalf("expected second register after cooldown, baseline=%d got=%d", baseline, got)
	}
}

func TestAuthManagerUnauthorizedThrottleResetsOnFailure(t *testing.T) {
	t.Parallel()

	var registerCalls atomic.Int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/register":
			count := registerCalls.Add(1)
			if count == 1 {
				http.Error(w, "boom", http.StatusInternalServerError)
				return
			}
			respondWithCredentials(t, w, "access-token", "refresh-token")
		case "/refresh":
			t.Fatalf("refresh should not be called during throttle failure test")
		default:
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
	}))
	t.Cleanup(srv.Close)

	dir := t.TempDir()
	store, err := state.Open(filepath.Join(dir, "state.db"))
	if err != nil {
		t.Fatalf("open state store: %v", err)
	}
	t.Cleanup(func() { store.Close() })

	cfg := authmanager.Config{
		ClusterID:      "cluster-123",
		RegisterURL:    srv.URL + "/register",
		RefreshURL:     srv.URL + "/refresh",
		BootstrapToken: "bootstrap-token",
	}

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	existing := state.Credentials{
		WatcherID:      "watcher-existing",
		AccessToken:    "access-existing",
		AccessExpires:  time.Now().Add(30 * time.Minute).UTC(),
		RefreshToken:   "refresh-existing",
		RefreshExpires: time.Now().Add(24 * time.Hour).UTC(),
	}
	if err := store.SaveCredentials(existing); err != nil {
		t.Fatalf("seed credentials: %v", err)
	}

	mgr := authmanager.New(cfg, store, nil, zap.NewNop())
	mgr.SetUnauthorizedCooldown(200 * time.Millisecond)
	if err := mgr.Initialize(ctx); err != nil {
		t.Fatalf("initialise auth manager: %v", err)
	}

	baseline := registerCalls.Load()

	if err := mgr.ForceRefreshAfterUnauthorized(ctx); err == nil {
		t.Fatalf("expected error from failed register")
	}
	if got := registerCalls.Load(); got != baseline+1 {
		t.Fatalf("expected first register attempt after failure, baseline=%d got=%d", baseline, got)
	}

	if err := mgr.ForceRefreshAfterUnauthorized(ctx); err != nil {
		t.Fatalf("expected retry after failure: %v", err)
	}
	if got := registerCalls.Load(); got != baseline+2 {
		t.Fatalf("expected retry to register again, baseline=%d got=%d", baseline, got)
	}
}

func TestAuthManagerUnauthorizedThrottleRespectsContext(t *testing.T) {
	t.Parallel()

	var registerCalls atomic.Int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/register":
			registerCalls.Add(1)
			respondWithCredentials(t, w, "access-token", "refresh-token")
		case "/refresh":
			t.Fatalf("refresh should not be called in context test")
		default:
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
	}))
	t.Cleanup(srv.Close)

	dir := t.TempDir()
	store, err := state.Open(filepath.Join(dir, "state.db"))
	if err != nil {
		t.Fatalf("open state store: %v", err)
	}
	t.Cleanup(func() { store.Close() })

	cfg := authmanager.Config{
		ClusterID:      "cluster-ctx",
		RegisterURL:    srv.URL + "/register",
		RefreshURL:     srv.URL + "/refresh",
		BootstrapToken: "bootstrap-token",
	}

	baseCtx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	mgr := authmanager.New(cfg, store, nil, zap.NewNop())
	mgr.SetUnauthorizedCooldown(200 * time.Millisecond)
	if err := mgr.Initialize(baseCtx); err != nil {
		t.Fatalf("initialise auth manager: %v", err)
	}

	baseline := registerCalls.Load()

	if err := mgr.ForceRefreshAfterUnauthorized(baseCtx); err != nil {
		t.Fatalf("first forced refresh: %v", err)
	}
	if got := registerCalls.Load(); got != baseline+1 {
		t.Fatalf("expected forced refresh to register once, baseline=%d got=%d", baseline, got)
	}

	waitCtx, cancelWait := context.WithTimeout(baseCtx, 25*time.Millisecond)
	defer cancelWait()

	start := time.Now()
	err = mgr.ForceRefreshAfterUnauthorized(waitCtx)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected deadline exceeded waiting for cooldown, got %v", err)
	}
	if elapsed := time.Since(start); elapsed < 20*time.Millisecond {
		t.Fatalf("expected wait before deadline exceeded, got %s", elapsed)
	}
	if got := registerCalls.Load(); got != baseline+1 {
		t.Fatalf("expected no additional register attempts during cooldown wait, baseline=%d got=%d", baseline, got)
	}
}

func TestAuthManagerWaitUnauthorizedCooldownWaitsRemainingWindow(t *testing.T) {
	t.Parallel()

	var registerCalls atomic.Int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/register":
			registerCalls.Add(1)
			respondWithCredentials(t, w, "access-token", "refresh-token")
		case "/refresh":
			t.Fatalf("refresh should not be invoked when testing cooldown wait")
		default:
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
	}))
	t.Cleanup(srv.Close)

	dir := t.TempDir()
	store, err := state.Open(filepath.Join(dir, "state.db"))
	if err != nil {
		t.Fatalf("open state store: %v", err)
	}
	t.Cleanup(func() { store.Close() })

	cfg := authmanager.Config{
		ClusterID:      "cluster-123",
		RegisterURL:    srv.URL + "/register",
		RefreshURL:     srv.URL + "/refresh",
		BootstrapToken: "bootstrap-token",
	}

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	mgr := authmanager.New(cfg, store, nil, zap.NewNop())
	mgr.SetUnauthorizedCooldown(120 * time.Millisecond)
	if err := mgr.Initialize(ctx); err != nil {
		t.Fatalf("initialise auth manager: %v", err)
	}

	if err := mgr.ForceRefreshAfterUnauthorized(ctx); err != nil {
		t.Fatalf("seed forced refresh: %v", err)
	}
	before := registerCalls.Load()

	start := time.Now()
	if err := mgr.WaitUnauthorizedCooldown(ctx); err != nil {
		t.Fatalf("wait cooldown: %v", err)
	}
	if elapsed := time.Since(start); elapsed < 90*time.Millisecond {
		t.Fatalf("expected wait close to cooldown window, got %s", elapsed)
	}
	if got := registerCalls.Load(); got != before {
		t.Fatalf("expected no additional register calls during cooldown wait, before=%d after=%d", before, got)
	}

	// Subsequent waits after the cooldown should return immediately.
	if err := mgr.WaitUnauthorizedCooldown(ctx); err != nil {
		t.Fatalf("second wait cooldown: %v", err)
	}
}

func TestAuthManagerWaitUnauthorizedCooldownRespectsDeadline(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/register":
			respondWithCredentials(t, w, "access-token", "refresh-token")
		case "/refresh":
			t.Fatalf("refresh should not be invoked in deadline test")
		default:
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
	}))
	t.Cleanup(srv.Close)

	dir := t.TempDir()
	store, err := state.Open(filepath.Join(dir, "state.db"))
	if err != nil {
		t.Fatalf("open state store: %v", err)
	}
	t.Cleanup(func() { store.Close() })

	cfg := authmanager.Config{
		ClusterID:      "cluster-123",
		RegisterURL:    srv.URL + "/register",
		RefreshURL:     srv.URL + "/refresh",
		BootstrapToken: "bootstrap-token",
	}

	baseCtx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	mgr := authmanager.New(cfg, store, nil, zap.NewNop())
	mgr.SetUnauthorizedCooldown(200 * time.Millisecond)
	if err := mgr.Initialize(baseCtx); err != nil {
		t.Fatalf("initialise auth manager: %v", err)
	}

	if err := mgr.ForceRefreshAfterUnauthorized(baseCtx); err != nil {
		t.Fatalf("seed forced refresh: %v", err)
	}

	waitCtx, cancelWait := context.WithTimeout(baseCtx, 50*time.Millisecond)
	defer cancelWait()

	start := time.Now()
	err = mgr.WaitUnauthorizedCooldown(waitCtx)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected deadline exceeded, got %v", err)
	}
	if elapsed := time.Since(start); elapsed < 45*time.Millisecond {
		t.Fatalf("expected wait until deadline, got %s", elapsed)
	}
}

func respondWithCredentials(t *testing.T, w http.ResponseWriter, accessToken, refreshToken string) {
	t.Helper()
	payload := map[string]string{
		"watcherId":        "watcher-test",
		"clusterId":        "cluster-123",
		"accessToken":      accessToken,
		"accessExpiresAt":  time.Now().Add(15 * time.Minute).UTC().Format(time.RFC3339),
		"refreshToken":     refreshToken,
		"refreshExpiresAt": time.Now().Add(24 * time.Hour).UTC().Format(time.RFC3339),
	}
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(payload); err != nil {
		t.Fatalf("encode credentials: %v", err)
	}
}
