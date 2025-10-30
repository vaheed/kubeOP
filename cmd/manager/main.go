package main

import (
    "context"
    crand "crypto/rand"
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
    lg.Info("starting manager")
    cfg, err := config.Parse()
    if err != nil {
        lg.Error("config", slog.String("error", err.Error()))
        os.Exit(2)
    }
    lg.Info("config loaded", slog.String("addr", cfg.HTTPAddr), slog.Bool("auth", cfg.RequireAuth), slog.Bool("dev", cfg.DevInsecure))

    d, err := db.Connect(cfg.DBURL)
    if err != nil { lg.Error("db.connect", slog.String("error", err.Error())); os.Exit(2) }
    if err := d.Ping(context.Background()); err != nil { lg.Error("db.ping", slog.String("error", err.Error())); os.Exit(2) }
    lg.Info("db ready")
    // KMS: allow dev-insecure mode to generate an ephemeral key if none provided
    var enc *kms.Envelope
    if len(cfg.KMSMasterKey) == 0 && cfg.DevInsecure {
        tmp := make([]byte, 32)
        if _, err := crand.Read(tmp); err != nil {
            lg.Error("kms", slog.String("error", err.Error()))
            os.Exit(2)
        }
        e, err := kms.New(tmp)
        if err != nil { lg.Error("kms", slog.String("error", err.Error())); os.Exit(2) }
        enc = e
    } else {
        e, err := kms.New(cfg.KMSMasterKey)
        if err != nil { lg.Error("kms", slog.String("error", err.Error())); os.Exit(2) }
        enc = e
    }
    d.ConfigurePool(cfg.DBMaxOpen, cfg.DBMaxIdle, cfg.DBConnMaxLife)

    s := api.New(lg, d, enc, cfg.RequireAuth, cfg.JWTKey)
    s.MustMigrate(context.Background())
    lg.Info("migrations applied")
    done := make(chan os.Signal, 1)
    signal.Notify(done, syscall.SIGINT, syscall.SIGTERM)
    go func() {
        ctx := context.Background()
        lg.Info("http listening", slog.String("addr", cfg.HTTPAddr))
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
