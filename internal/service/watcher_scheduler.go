package service

import (
	"context"
	"time"

	"go.uber.org/zap"
	"kubeop/internal/watcherdeploy"
)

// WatcherScheduler coordinates asynchronous watcher deployment ensures.
type WatcherScheduler interface {
	// Schedule queues a watcher ensure task. Implementations must never block the caller
	// for the duration of the ensure; they should return immediately once the work has been
	// queued. Implementations are expected to add their own logging and error handling.
	Schedule(context.Context, string, string, watcherdeploy.Loader)
}

const defaultWatcherConcurrency = 4

type asyncWatcherScheduler struct {
	provisioner watcherdeploy.Provisioner
	logger      *zap.Logger
	timeout     time.Duration
	sem         chan struct{}
}

// newAsyncWatcherScheduler builds the default asynchronous scheduler used by the service.
func newAsyncWatcherScheduler(provisioner watcherdeploy.Provisioner, logger *zap.Logger, timeout time.Duration) WatcherScheduler {
	if provisioner == nil {
		return nil
	}
	if logger == nil {
		logger = zap.NewNop()
	}
	if timeout <= 0 {
		timeout = 2 * time.Minute
	}
	return &asyncWatcherScheduler{
		provisioner: provisioner,
		logger:      logger,
		timeout:     timeout,
		sem:         make(chan struct{}, defaultWatcherConcurrency),
	}
}

func (s *asyncWatcherScheduler) Schedule(_ context.Context, clusterID, clusterName string, loader watcherdeploy.Loader) {
	if loader == nil {
		s.logger.Error("skipping watcher ensure", zap.String("cluster_id", clusterID), zap.String("cluster_name", clusterName), zap.String("reason", "loader nil"))
		return
	}
	logger := s.logger.With(zap.String("cluster_id", clusterID))
	if clusterName != "" {
		logger = logger.With(zap.String("cluster_name", clusterName))
	}
	logger.Info("scheduled watcher ensure")
	go s.execute(logger, clusterID, clusterName, loader)
}

func (s *asyncWatcherScheduler) execute(logger *zap.Logger, clusterID, clusterName string, loader watcherdeploy.Loader) {
	s.sem <- struct{}{}
	defer func() { <-s.sem }()

	ctx := context.Background()
	if s.timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, s.timeout)
		defer cancel()
	}

	logger.Info("starting watcher ensure")
	if err := s.provisioner.Ensure(ctx, clusterID, clusterName, loader); err != nil {
		logger.Error("watcher ensure failed", zap.Error(err))
		return
	}
	logger.Info("watcher ensure complete")
}
