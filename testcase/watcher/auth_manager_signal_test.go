package watcher_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"go.uber.org/zap"

	"kubeop/internal/state"
	authmanager "kubeop/internal/watcher/authmanager"
)

func TestAuthManagerSignalUnauthorizedTriggersRegister(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	store, err := state.Open(filepath.Join(dir, "state.db"))
	if err != nil {
		t.Fatalf("open state store: %v", err)
	}
	t.Cleanup(func() { store.Close() })

	creds := state.Credentials{
		WatcherID:      "watcher-1",
		AccessToken:    "existing-token",
		AccessExpires:  time.Now().Add(30 * time.Minute).UTC(),
		RefreshToken:   "existing-refresh",
		RefreshExpires: time.Now().Add(48 * time.Hour).UTC(),
	}
	if err := store.SaveCredentials(creds); err != nil {
		t.Fatalf("save credentials: %v", err)
	}

	var registerCalls atomic.Int64
	authSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/register":
			registerCalls.Add(1)
			payload := map[string]string{
				"watcherId":        "watcher-1",
				"clusterId":        "cluster-1",
				"accessToken":      "new-token",
				"accessExpiresAt":  time.Now().Add(time.Hour).UTC().Format(time.RFC3339),
				"refreshToken":     "new-refresh",
				"refreshExpiresAt": time.Now().Add(24 * time.Hour).UTC().Format(time.RFC3339),
			}
			w.Header().Set("Content-Type", "application/json")
			if err := json.NewEncoder(w).Encode(payload); err != nil {
				t.Fatalf("encode register response: %v", err)
			}
		case "/refresh":
			http.Error(w, "forbidden", http.StatusUnauthorized)
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
	t.Cleanup(authSrv.Close)

	mgr := authmanager.New(authmanager.Config{
		ClusterID:      "cluster-1",
		RegisterURL:    authSrv.URL + "/register",
		RefreshURL:     authSrv.URL + "/refresh",
		BootstrapToken: "bootstrap",
	}, store, nil, zap.NewNop())

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	if err := mgr.Initialize(ctx); err != nil {
		t.Fatalf("initialise auth manager: %v", err)
	}

	before := registerCalls.Load()
	mgr.SignalUnauthorized()

	deadline := time.Now().Add(5 * time.Second)
	for {
		if registerCalls.Load() > before {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for register call")
		}
		time.Sleep(50 * time.Millisecond)
	}
}
