package testcase

import (
	"context"
	"errors"
	"testing"
	"time"

	"go.uber.org/zap"
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
	failIDs map[string]error
	results map[string]service.ClusterHealth
}

func (f *fakeClusterChecker) CheckCluster(ctx context.Context, id string) (service.ClusterHealth, error) {
	f.calls++
	f.lastCtx = ctx
	if f.failIDs != nil {
		if err, ok := f.failIDs[id]; ok && err != nil {
			return service.ClusterHealth{}, err
		}
	}
	if f.results != nil {
		if res, ok := f.results[id]; ok {
			return res, nil
		}
	}
	return service.ClusterHealth{ID: id, Name: "cluster", Healthy: true, Checked: time.Now().UTC()}, nil
}

func TestClusterHealthSchedulerTickUsesBoundedContext(t *testing.T) {
	storeStub := &fakeClusterStore{clusters: []store.Cluster{{ID: "c1", Name: "c1"}}}
	checkerStub := &fakeClusterChecker{}
	logger := zap.NewNop()
	scheduler := service.NewClusterHealthScheduler(storeStub, checkerStub, logger)
	scheduler.TickTimeout = 50 * time.Millisecond

	start := time.Now()
	summary, err := scheduler.TickWithSummary(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

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
	if summary.Clusters != 1 || summary.Healthy != 1 {
		t.Fatalf("unexpected summary: %+v", summary)
	}
}

func TestClusterHealthSchedulerTickHandlesStoreError(t *testing.T) {
	storeStub := &fakeClusterStore{err: context.DeadlineExceeded}
	checkerStub := &fakeClusterChecker{}
	logger := zap.NewNop()
	scheduler := service.NewClusterHealthScheduler(storeStub, checkerStub, logger)
	scheduler.TickTimeout = 10 * time.Millisecond

	_, err := scheduler.TickWithSummary(context.Background())

	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected store error, got %v", err)
	}
	if checkerStub.calls != 0 {
		t.Fatalf("expected no CheckCluster calls on store error, got %d", checkerStub.calls)
	}
}

func TestClusterHealthSchedulerTickContinuesAfterCheckerError(t *testing.T) {
	storeStub := &fakeClusterStore{clusters: []store.Cluster{
		{ID: "c1", Name: "cluster-1"},
		{ID: "c2", Name: "cluster-2"},
	}}
	checkerStub := &fakeClusterChecker{
		failIDs: map[string]error{
			"c1": context.DeadlineExceeded,
		},
		results: map[string]service.ClusterHealth{
			"c2": {ID: "c2", Name: "cluster-2", Healthy: true, Checked: time.Now().UTC()},
		},
	}
	scheduler := service.NewClusterHealthScheduler(storeStub, checkerStub, zap.NewNop())

	summary, err := scheduler.TickWithSummary(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if checkerStub.calls != 2 {
		t.Fatalf("expected all clusters to be checked despite error, got %d", checkerStub.calls)
	}
	if summary.Healthy != 1 {
		t.Fatalf("expected one healthy cluster, got %+v", summary)
	}
	if summary.Unhealthy != 1 || len(summary.Failures) != 1 {
		t.Fatalf("expected single failure, got %+v", summary)
	}
}

func TestClusterHealthSchedulerTickHandlesMissingDependencies(t *testing.T) {
	scheduler := service.NewClusterHealthScheduler(nil, nil, zap.NewNop())

	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("Tick panicked with missing dependencies: %v", r)
		}
	}()

	scheduler.Tick(context.Background())
	summary, err := scheduler.TickWithSummary(context.Background())
	if !errors.Is(err, service.ErrSchedulerDependenciesMissing) {
		t.Fatalf("expected ErrSchedulerDependenciesMissing, got %v", err)
	}
	if summary.Clusters != 0 || summary.Healthy != 0 || summary.Unhealthy != 0 {
		t.Fatalf("expected zeroed summary for missing deps, got %+v", summary)
	}
}
