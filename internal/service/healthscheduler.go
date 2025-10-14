package service

import (
	"context"
	"errors"
	"time"

	"go.uber.org/zap"
	"kubeop/internal/store"
)

// ErrSchedulerDependenciesMissing indicates the scheduler cannot run because
// either the store or checker dependency is missing.
var ErrSchedulerDependenciesMissing = errors.New("cluster health scheduler dependencies missing")

// ClusterHealthFailure captures information about a failed cluster check so
// callers can act on repeated outages without parsing logs.
type ClusterHealthFailure struct {
	ClusterID   string
	ClusterName string
	Error       string
}

// ClusterHealthSummary reports aggregated statistics for a single scheduler
// tick, including duration and failure details for downstream observability.
type ClusterHealthSummary struct {
	StartedAt time.Time
	Duration  time.Duration
	Clusters  int
	Healthy   int
	Unhealthy int
	Failures  []ClusterHealthFailure
}

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
	summary, err := s.TickWithSummary(ctx)
	logger := s.logger
	if logger == nil {
		logger = zap.NewNop()
	}
	if errors.Is(err, ErrSchedulerDependenciesMissing) {
		return
	}
	if err != nil {
		logger.Warn("cluster health tick failed", zap.Error(err))
		return
	}
	logger.Info("cluster health tick complete",
		zap.Int("clusters_checked", summary.Clusters),
		zap.Int("healthy", summary.Healthy),
		zap.Int("unhealthy", summary.Unhealthy),
		zap.Int("failures", len(summary.Failures)),
		zap.Time("started_at", summary.StartedAt),
		zap.Duration("duration", summary.Duration),
		zap.Duration("tick_timeout", s.TickTimeout),
	)
}

// TickWithSummary mirrors Tick but surfaces aggregated statistics and errors so
// callers can introspect the run without scraping logs.
func (s *ClusterHealthScheduler) TickWithSummary(ctx context.Context) (summary ClusterHealthSummary, err error) {
	summary.StartedAt = time.Now()
	defer func() {
		if summary.StartedAt.IsZero() {
			summary.StartedAt = time.Now()
		}
		summary.Duration = time.Since(summary.StartedAt)
	}()
	if s == nil {
		return summary, nil
	}
	logger := s.logger
	if logger == nil {
		logger = zap.NewNop()
	}
	if s.store == nil || s.checker == nil {
		logger.Warn("cluster health scheduler dependencies missing",
			zap.Bool("store_nil", s.store == nil),
			zap.Bool("checker_nil", s.checker == nil),
		)
		return summary, ErrSchedulerDependenciesMissing
	}
	tickCtx, cancel := context.WithTimeout(ctx, s.TickTimeout)
	defer cancel()
	clusters, err := s.store.ListClusters(tickCtx)
	if err != nil {
		logger.Warn("scheduler list clusters failed", zap.Error(err))
		return summary, err
	}
	summary.Clusters = len(clusters)
	for _, cinfo := range clusters {
		health, checkErr := s.checker.CheckCluster(tickCtx, cinfo.ID)
		if checkErr != nil {
			logger.Warn("cluster health error",
				zap.String("cluster_id", cinfo.ID),
				zap.String("cluster_name", cinfo.Name),
				zap.Error(checkErr),
			)
			summary.Unhealthy++
			summary.Failures = append(summary.Failures, ClusterHealthFailure{
				ClusterID:   cinfo.ID,
				ClusterName: cinfo.Name,
				Error:       checkErr.Error(),
			})
			continue
		}
		if health.ID == "" {
			health.ID = cinfo.ID
		}
		if health.Name == "" {
			health.Name = cinfo.Name
		}
		fields := []zap.Field{
			zap.String("cluster_id", health.ID),
			zap.String("cluster_name", health.Name),
			zap.Bool("healthy", health.Healthy),
		}
		if health.Error != "" {
			fields = append(fields, zap.String("error", health.Error))
		}
		if health.Healthy {
			summary.Healthy++
			logger.Info("cluster health", fields...)
		} else {
			summary.Unhealthy++
			logger.Warn("cluster health", fields...)
		}
	}
	return summary, nil
}
