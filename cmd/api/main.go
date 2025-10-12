package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

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

	// Setup logger
	logger := logging.NewLogger(cfg.LogLevel)
	slog.SetDefault(logger)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Connect store and run migrations
	st, err := store.New(ctx, cfg.DatabaseURL)
	if err != nil {
		slog.Error("failed to connect database", slog.String("error", err.Error()))
		os.Exit(1)
	}
	if err := st.Migrate(); err != nil {
		slog.Error("failed to run migrations", slog.String("error", err.Error()))
		os.Exit(1)
	}

	// Kube multi-cluster manager
	km := kube.NewManager()

	// Service layer
	svc, err := service.New(cfg, st, km)
	if err != nil {
		slog.Error("failed creating service", slog.String("error", err.Error()))
		os.Exit(1)
	}

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
		slog.Info("starting api server", slog.Int("port", cfg.Port), slog.String("version", version.Version))
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("server failed", slog.String("error", err.Error()))
		}
	}()

	// Cluster health scheduler
	interval := time.Duration(cfg.ClusterHealthIntervalSeconds) * time.Second
	if interval <= 0 {
		interval = 60 * time.Second
	}
	scheduler := service.NewClusterHealthScheduler(st, svc, slog.Default())
	go scheduler.Run(ctx, interval)

	// Graceful shutdown
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop

	cancel()

	ctxShutdown, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()
	if err := srv.Shutdown(ctxShutdown); err != nil {
		slog.Error("server shutdown failed", slog.String("error", err.Error()))
	}
	if err := st.Close(); err != nil {
		slog.Warn("store close failed", slog.String("error", err.Error()))
	}
	slog.Info("server stopped")
}
