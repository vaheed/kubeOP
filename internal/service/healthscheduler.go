package service

import (
	"context"
	"log/slog"
	"time"

	"kubeop/internal/store"
)

// clusterLister exposes the subset of store.Store used by the scheduler. The
// concrete *store.Store satisfies this interface and keeps the scheduler
// testable.
type clusterLister interface {
	ListClusters(ctx context.Context) ([]store.Cluster, error)
}

// clusterChecker exposes the health check capability implemented by Service.
type clusterChecker interface {
	CheckCluster(ctx context.Context, id string) (ClusterHealth, error)
}

// ClusterHealthScheduler periodically checks registered clusters and emits
// structured logs. The scheduler is safe for reuse in tests via its minimal
// dependencies.
type ClusterHealthScheduler struct {
	store   clusterLister
	checker clusterChecker
	logger  *slog.Logger
	// TickTimeout bounds each health probe and defaults to 20 seconds.
	TickTimeout time.Duration
}

// NewClusterHealthScheduler wires the store and service into a scheduler
// helper. A nil logger falls back to slog.Default().
func NewClusterHealthScheduler(store clusterLister, checker clusterChecker, logger *slog.Logger) *ClusterHealthScheduler {
	if logger == nil {
		logger = slog.Default()
	}
	return &ClusterHealthScheduler{
		store:       store,
		checker:     checker,
		logger:      logger,
		TickTimeout: 20 * time.Second,
	}
}

// Run executes ticks on the provided interval until ctx is cancelled.
func (s *ClusterHealthScheduler) Run(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	s.logger.InfoContext(ctx, "cluster health scheduler started", slog.Duration("interval", interval))
	defer func() {
		ticker.Stop()
		s.logger.InfoContext(ctx, "cluster health scheduler stopped")
	}()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.Tick(ctx)
		}
	}
}

// Tick performs a single bounded health probe across all clusters. It logs the
// aggregate success and emits warnings for failures. The function tolerates
// partial failures so operators receive visibility without breaking the loop.
func (s *ClusterHealthScheduler) Tick(ctx context.Context) {
	if s == nil {
		return
	}
	tickCtx, cancel := context.WithTimeout(ctx, s.TickTimeout)
	defer cancel()
	clusters, err := s.store.ListClusters(tickCtx)
	if err != nil {
		s.logger.WarnContext(tickCtx, "scheduler list clusters failed", slog.String("error", err.Error()))
		return
	}
	var healthy, unhealthy int
	for _, cinfo := range clusters {
		health, err := s.checker.CheckCluster(tickCtx, cinfo.ID)
		if err != nil {
			s.logger.WarnContext(tickCtx, "cluster health error", slog.String("cluster", cinfo.Name), slog.String("error", err.Error()))
			unhealthy++
			continue
		}
		lvl := slog.LevelInfo
		if !health.Healthy {
			lvl = slog.LevelWarn
			unhealthy++
		} else {
			healthy++
		}
		s.logger.Log(tickCtx, lvl, "cluster health",
			slog.String("cluster", health.Name),
			slog.Bool("healthy", health.Healthy),
			slog.String("error", health.Error),
		)
	}
	s.logger.InfoContext(tickCtx, "cluster health tick complete",
		slog.Int("clusters_checked", len(clusters)),
		slog.Int("healthy", healthy),
		slog.Int("unhealthy", unhealthy),
	)
}
