package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"go.uber.org/zap"
	"kubeop/internal/api"
	"kubeop/internal/config"
	"kubeop/internal/kube"
	"kubeop/internal/logging"
	"kubeop/internal/service"
	"kubeop/internal/store"
	"kubeop/internal/version"
)

func main() {
	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to load config: %v\n", err)
		os.Exit(1)
	}

	meta := version.Metadata()
	logManager, err := logging.Setup(logging.Metadata{Version: meta.Build.Version, Commit: meta.Build.Commit})
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to initialise logging: %v\n", err)
		os.Exit(1)
	}
	logger := logging.L()
	logger.Info(
		"build metadata",
		zap.String("version", meta.Build.Version),
		zap.String("commit", meta.Build.Commit),
		zap.String("date", meta.Build.Date),
		zap.String("min_client_version", meta.Compatibility.MinClientVersion),
		zap.String("min_api_version", meta.Compatibility.MinAPIVersion),
		zap.String("max_api_version", meta.Compatibility.MaxAPIVersion),
	)
	if meta.Deprecated(time.Now().UTC()) {
		fields := []zap.Field{
			zap.String("version", meta.Build.Version),
		}
		if deadline, ok := meta.DeadlineTime(); ok {
			fields = append(fields, zap.Time("deprecation_deadline", deadline))
		}
		if meta.Deprecation != nil && meta.Deprecation.Note != "" {
			fields = append(fields, zap.String("note", meta.Deprecation.Note))
		}
		logger.Warn("running deprecated kubeOP build", fields...)
	}
	logger.Info("configuration loaded", zap.String("env", cfg.Env), zap.Int("port", cfg.Port))
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Connect store and run migrations
	st, err := store.New(ctx, cfg.DatabaseURL)
	if err != nil {
		logger.Error("failed to connect database", zap.String("error", err.Error()))
		os.Exit(1)
	}
	if err := st.Migrate(); err != nil {
		logger.Error("failed to run migrations", zap.String("error", err.Error()))
		os.Exit(1)
	}
	logger.Info("database connected and migrations applied")

	// Kube multi-cluster manager
	km := kube.NewManager()

	// Service layer
	svc, err := service.New(cfg, st, km)
	if err != nil {
		logger.Error("failed creating service", zap.String("error", err.Error()))
		os.Exit(1)
	}
	if err := svc.EnsureProjectLogs(ctx); err != nil {
		logger.Error("failed to prepare project logs", zap.String("error", err.Error()))
		os.Exit(1)
	}
	logger.Info("service layer initialised")

	// HTTP server
	router := api.NewRouter(cfg, svc)

	srv := &http.Server{
		Addr:              fmt.Sprintf(":%d", cfg.Port),
		Handler:           router,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      60 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	go func() {
		logger.Info("starting api server", zap.Int("port", cfg.Port), zap.String("version", meta.Build.Version))
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("server failed", zap.String("error", err.Error()))
		}
	}()

	hup := make(chan os.Signal, 1)
	signal.Notify(hup, syscall.SIGHUP)
	go func() {
		for range hup {
			if logManager != nil {
				logManager.Reopen()
			}
		}
	}()

	// Cluster health scheduler
	interval := time.Duration(cfg.ClusterHealthIntervalSeconds) * time.Second
	if interval <= 0 {
		interval = 60 * time.Second
	}
	scheduler := service.NewClusterHealthScheduler(st, svc, logger)
	logger.Info("cluster health scheduler starting", zap.Duration("interval", interval))
	go scheduler.Run(ctx, interval)

	// Graceful shutdown
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop

	cancel()

	ctxShutdown, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()
	if err := srv.Shutdown(ctxShutdown); err != nil {
		logger.Error("server shutdown failed", zap.String("error", err.Error()))
	}
	if err := st.Close(); err != nil {
		logger.Warn("store close failed", zap.String("error", err.Error()))
	}
	logger.Info("server stopped")
	if logManager != nil {
		logManager.Sync()
	}
}
