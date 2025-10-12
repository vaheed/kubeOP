package testcase

import (
	"context"
	"io"
	"log/slog"
	"testing"
	"time"

	"kubeop/internal/service"
	"kubeop/internal/store"
)

type fakeClusterStore struct {
	clusters []store.Cluster
	lastCtx  context.Context
	err      error
}

func (f *fakeClusterStore) ListClusters(ctx context.Context) ([]store.Cluster, error) {
	f.lastCtx = ctx
	return f.clusters, f.err
}

type fakeClusterChecker struct {
	calls   int
	lastCtx context.Context
}

func (f *fakeClusterChecker) CheckCluster(ctx context.Context, id string) (service.ClusterHealth, error) {
	f.calls++
	f.lastCtx = ctx
	return service.ClusterHealth{ID: id, Name: "cluster", Healthy: true, Checked: time.Now().UTC()}, nil
}

func TestClusterHealthSchedulerTickUsesBoundedContext(t *testing.T) {
	storeStub := &fakeClusterStore{clusters: []store.Cluster{{ID: "c1", Name: "c1"}}}
	checkerStub := &fakeClusterChecker{}
	logger := slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelInfo}))
	scheduler := service.NewClusterHealthScheduler(storeStub, checkerStub, logger)
	scheduler.TickTimeout = 50 * time.Millisecond

	start := time.Now()
	scheduler.Tick(context.Background())

	if checkerStub.calls != 1 {
		t.Fatalf("expected 1 CheckCluster call, got %d", checkerStub.calls)
	}
	if storeStub.lastCtx == nil {
		t.Fatalf("store did not receive context")
	}
	if checkerStub.lastCtx == nil {
		t.Fatalf("checker did not receive context")
	}
	if _, ok := storeStub.lastCtx.Deadline(); !ok {
		t.Fatalf("store context is missing deadline")
	}
	deadline, ok := checkerStub.lastCtx.Deadline()
	if !ok {
		t.Fatalf("checker context is missing deadline")
	}
	if deadline.Before(start) {
		t.Fatalf("deadline %v unexpectedly before start %v", deadline, start)
	}
	if time.Until(deadline) > scheduler.TickTimeout+25*time.Millisecond {
		t.Fatalf("deadline not bounded by TickTimeout: remaining=%v", time.Until(deadline))
	}
}

func TestClusterHealthSchedulerTickHandlesStoreError(t *testing.T) {
	storeStub := &fakeClusterStore{err: context.DeadlineExceeded}
	checkerStub := &fakeClusterChecker{}
	logger := slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelInfo}))
	scheduler := service.NewClusterHealthScheduler(storeStub, checkerStub, logger)
	scheduler.TickTimeout = 10 * time.Millisecond

	scheduler.Tick(context.Background())

	if checkerStub.calls != 0 {
		t.Fatalf("expected no CheckCluster calls on store error, got %d", checkerStub.calls)
	}
}
