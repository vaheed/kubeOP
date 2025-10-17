package watcher_test

import (
	"testing"
	"time"

	"kubeop/internal/watcher/authutil"
)

func TestNextAccessRefreshHalfLife(t *testing.T) {
	now := time.Now()
	expires := now.Add(5 * time.Minute)

	refreshAt := authutil.NextAccessRefresh(now, expires)
	expected := now.Add(150 * time.Second)
	if refreshAt.Before(expected.Add(-5*time.Second)) || refreshAt.After(expected.Add(5*time.Second)) {
		t.Fatalf("expected refresh near %s, got %s", expected, refreshAt)
	}
}

func TestNextAccessRefreshHandlesShortLifetime(t *testing.T) {
	now := time.Now()
	expires := now.Add(45 * time.Second)

	refreshAt := authutil.NextAccessRefresh(now, expires)
	minDelay := now.Add(15 * time.Second)
	if refreshAt.Before(minDelay) {
		t.Fatalf("expected refreshAt >= %s, got %s", minDelay, refreshAt)
	}
	maxDelay := expires.Add(-30 * time.Second)
	if refreshAt.After(maxDelay) {
		t.Fatalf("expected refreshAt <= %s, got %s", maxDelay, refreshAt)
	}
}

func TestNextAccessRefreshExpiredToken(t *testing.T) {
	now := time.Now()
	expires := now.Add(-time.Minute)

	refreshAt := authutil.NextAccessRefresh(now, expires)
	if !refreshAt.Equal(now) {
		t.Fatalf("expected immediate refresh at %s, got %s", now, refreshAt)
	}
}
