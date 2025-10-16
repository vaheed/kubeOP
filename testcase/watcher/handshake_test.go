package watcher_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	watcherhandshake "kubeop/internal/watcher/handshake"
)

func TestPerformReturnsClusterID(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer token" {
			t.Fatalf("expected bearer token header, got %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"ok","cluster_id":"cluster-123"}`))
	}))
	t.Cleanup(srv.Close)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	t.Cleanup(cancel)

	clusterID, err := watcherhandshake.Perform(ctx, srv.Client(), srv.URL, "token", "")
	if err != nil {
		t.Fatalf("perform handshake: %v", err)
	}
	if clusterID != "cluster-123" {
		t.Fatalf("expected cluster-123, got %q", clusterID)
	}
}

func TestPerformUsesExpectedClusterWhenMissing(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	}))
	t.Cleanup(srv.Close)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	t.Cleanup(cancel)

	clusterID, err := watcherhandshake.Perform(ctx, srv.Client(), srv.URL, "token", "expected-cluster")
	if err != nil {
		t.Fatalf("perform handshake: %v", err)
	}
	if clusterID != "expected-cluster" {
		t.Fatalf("expected fallback cluster id, got %q", clusterID)
	}
}

func TestPerformErrorsOnMismatch(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"status":"ok","cluster_id":"other"}`))
	}))
	t.Cleanup(srv.Close)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	t.Cleanup(cancel)

	if _, err := watcherhandshake.Perform(ctx, srv.Client(), srv.URL, "token", "expected"); err == nil {
		t.Fatal("expected mismatch error, got nil")
	}
}

func TestPerformErrorsWhenClusterMissingAndNoExpectation(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	}))
	t.Cleanup(srv.Close)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	t.Cleanup(cancel)

	if _, err := watcherhandshake.Perform(ctx, srv.Client(), srv.URL, "token", ""); err == nil {
		t.Fatal("expected error for missing cluster id, got nil")
	}
}

func TestPerformPropagatesHTTPFailures(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "boom", http.StatusInternalServerError)
	}))
	t.Cleanup(srv.Close)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	t.Cleanup(cancel)

	if _, err := watcherhandshake.Perform(ctx, srv.Client(), srv.URL, "token", "cluster"); err == nil {
		t.Fatal("expected HTTP error, got nil")
	}
}
