package main

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

	"kubeop/internal/sink"
	"kubeop/internal/state"
	authmanager "kubeop/internal/watcher/authmanager"
	"kubeop/internal/watcher/readiness"
)

func TestStartHandshakeLoopSignalsUnauthorized(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	store, err := state.Open(filepath.Join(dir, "state.db"))
	if err != nil {
		t.Fatalf("open state store: %v", err)
	}
	t.Cleanup(func() { store.Close() })

	creds := state.Credentials{
		WatcherID:      "watcher-1",
		AccessToken:    "initial-token",
		AccessExpires:  time.Now().Add(30 * time.Minute).UTC(),
		RefreshToken:   "refresh-token",
		RefreshExpires: time.Now().Add(24 * time.Hour).UTC(),
	}
	if err := store.SaveCredentials(creds); err != nil {
		t.Fatalf("save credentials: %v", err)
	}

	var registerCalls atomic.Int64
	authSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/register":
			if r.Header.Get("Authorization") != "Bearer bootstrap-token" {
				t.Fatalf("unexpected authorization header: %q", r.Header.Get("Authorization"))
			}
			registerCalls.Add(1)
			payload := map[string]string{
				"watcherId":        "watcher-1",
				"clusterId":        "cluster-1",
				"accessToken":      "new-token",
				"accessExpiresAt":  time.Now().Add(time.Hour).UTC().Format(time.RFC3339),
				"refreshToken":     "new-refresh",
				"refreshExpiresAt": time.Now().Add(48 * time.Hour).UTC().Format(time.RFC3339),
			}
			w.Header().Set("Content-Type", "application/json")
			if err := json.NewEncoder(w).Encode(payload); err != nil {
				t.Fatalf("encode register response: %v", err)
			}
		case "/refresh":
			http.Error(w, "forbidden", http.StatusUnauthorized)
		default:
			t.Fatalf("unexpected auth path: %s", r.URL.Path)
		}
	}))
	t.Cleanup(authSrv.Close)

	ingestSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "nope", http.StatusUnauthorized)
	}))
	t.Cleanup(ingestSrv.Close)

	sinkClient := ingestSrv.Client()
	sinkClient.Timeout = time.Second
	eventSink, err := sink.New(sink.Config{
		URL:           ingestSrv.URL,
		Token:         "initial-token",
		BatchMax:      1,
		HTTPTimeout:   time.Second,
		HTTPClient:    sinkClient,
		AllowInsecure: true,
	}, zap.NewNop())
	if err != nil {
		t.Fatalf("new sink: %v", err)
	}

	mgr := authmanager.New(authmanager.Config{
		ClusterID:      "cluster-1",
		RegisterURL:    authSrv.URL + "/register",
		RefreshURL:     authSrv.URL + "/refresh",
		BootstrapToken: "bootstrap-token",
	}, store, nil, zap.NewNop())
	mgr.AttachSink(eventSink)

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	if err := mgr.Initialize(ctx); err != nil {
		t.Fatalf("initialise auth manager: %v", err)
	}

	queue := NewEventQueue(store, zap.NewNop())
	evt := sink.Event{ClusterID: "cluster-1", EventType: "Added", Kind: "Pod", Name: "pod", Summary: "created", DedupKey: "pod#1"}
	if err := queue.Store([]sink.Event{evt}); err != nil {
		t.Fatalf("store queued event: %v", err)
	}

	originalPerform := PerformHandshake
	PerformHandshake = func(context.Context, *http.Client, string, string, string) (string, error) {
		return "cluster-1", nil
	}
	defer func() { PerformHandshake = originalPerform }()

	status := readiness.New()
	StartHandshakeLoop(ctx, WatcherConfig{
		ClusterID:    "cluster-1",
		HandshakeURL: "https://example.com/handshake",
		BatchMax:     1,
	}, status, queue, eventSink, mgr, zap.NewNop())

	deadline := time.Now().Add(15 * time.Second)
	for {
		if registerCalls.Load() > 0 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for register call")
		}
		time.Sleep(50 * time.Millisecond)
	}
}
