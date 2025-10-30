package logging

import (
    "log/slog"
    "os"
)

func New(component string) *slog.Logger {
    h := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo})
    if lvl := os.Getenv("LOG_LEVEL"); lvl != "" {
        switch lvl {
        case "debug":
            h = slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug})
        case "warn":
            h = slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelWarn})
        }
    }
    return slog.New(h).With("component", component)
}

