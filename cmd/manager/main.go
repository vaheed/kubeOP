package main

import (
    "context"
    "log/slog"
    "os"
    "os/signal"
    "syscall"
    "time"

    "github.com/vaheed/kubeop/internal/api"
    "github.com/vaheed/kubeop/internal/config"
    "github.com/vaheed/kubeop/internal/db"
    "github.com/vaheed/kubeop/internal/kms"
    "github.com/vaheed/kubeop/internal/logging"
    "github.com/vaheed/kubeop/internal/usage"
)

func main() {
    lg := logging.New("manager")
    cfg, err := config.Parse()
    if err != nil {
        lg.Error("config", slog.String("error", err.Error()))
        os.Exit(2)
    }

    d, err := db.Connect(cfg.DBURL)
    if err != nil { lg.Error("db", slog.String("error", err.Error())); os.Exit(2) }
    if err := d.Ping(context.Background()); err != nil { lg.Error("db", slog.String("error", err.Error())); os.Exit(2) }
    enc, err := kms.New(cfg.KMSMasterKey)
    if err != nil { lg.Error("kms", slog.String("error", err.Error())); os.Exit(2) }
    d.ConfigurePool(cfg.DBMaxOpen, cfg.DBMaxIdle, cfg.DBConnMaxLife)

    s := api.New(lg, d, enc, cfg.RequireAuth, cfg.JWTKey)
    s.MustMigrate(context.Background())
    done := make(chan os.Signal, 1)
    signal.Notify(done, syscall.SIGINT, syscall.SIGTERM)
    go func() {
        ctx := context.Background()
        if err := s.Start(ctx, cfg.HTTPAddr); err != nil {
            lg.Error("http", slog.String("error", err.Error()))
            os.Exit(1)
        }
    }()
    // Optional background aggregator
    if os.Getenv("KUBEOP_AGGREGATOR") == "true" {
        ag := &usage.Aggregator{Log: lg, DB: d.DB}
        go func() {
            ticker := time.NewTicker(1 * time.Hour)
            defer ticker.Stop()
            // Run once on startup for last hour
            _ = ag.RunOnce(context.Background())
            for {
                select {
                case <-ticker.C:
                    _ = ag.RunOnce(context.Background())
                case <-done:
                    return
                }
            }
        }()
    }
    <-done
    lg.Info("shutting down")
}
