package service

import (
	"context"
	"time"

	"go.uber.org/zap"
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
	logger  *zap.Logger
	// TickTimeout bounds each health probe and defaults to 20 seconds.
	TickTimeout time.Duration
}

// NewClusterHealthScheduler wires the store and service into a scheduler
// helper. A nil logger falls back to zap.NewNop().
func NewClusterHealthScheduler(store clusterLister, checker clusterChecker, logger *zap.Logger) *ClusterHealthScheduler {
	if logger == nil {
		logger = zap.NewNop()
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
	s.logger.Info("cluster health scheduler started", zap.Duration("interval", interval))
	defer func() {
		ticker.Stop()
		s.logger.Info("cluster health scheduler stopped")
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
		s.logger.Warn("scheduler list clusters failed", zap.String("error", err.Error()))
		return
	}
	var healthy, unhealthy int
	for _, cinfo := range clusters {
		health, err := s.checker.CheckCluster(tickCtx, cinfo.ID)
		if err != nil {
			s.logger.Warn("cluster health error", zap.String("cluster", cinfo.Name), zap.String("error", err.Error()))
			unhealthy++
			continue
		}
		fields := []zap.Field{
			zap.String("cluster", health.Name),
			zap.Bool("healthy", health.Healthy),
		}
		if health.Error != "" {
			fields = append(fields, zap.String("error", health.Error))
		}
		if health.Healthy {
			healthy++
			s.logger.Info("cluster health", fields...)
		} else {
			unhealthy++
			s.logger.Warn("cluster health", fields...)
		}
	}
	s.logger.Info("cluster health tick complete",
		zap.Int("clusters_checked", len(clusters)),
		zap.Int("healthy", healthy),
		zap.Int("unhealthy", unhealthy),
	)
}
