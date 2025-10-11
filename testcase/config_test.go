package testcase

import (
    "os"
    "path/filepath"
    "testing"

    "kubeop/internal/config"
)

func TestConfigLoad_FromEnv(t *testing.T) {
    t.Setenv("APP_ENV", "test")
    t.Setenv("PORT", "9090")
    t.Setenv("LOG_LEVEL", "debug")
    t.Setenv("DISABLE_AUTH", "false")
    t.Setenv("DATABASE_URL", "postgres://u:p@localhost:5432/db?sslmode=disable")
    t.Setenv("ADMIN_JWT_SECRET", "secret")
    t.Setenv("KCFG_ENCRYPTION_KEY", "key")

    cfg, err := config.Load()
    if err != nil {
        t.Fatalf("Load() error: %v", err)
    }
    if cfg.Env != "test" || cfg.Port != 9090 || cfg.LogLevel != "debug" {
        t.Fatalf("unexpected basic values: %+v", cfg)
    }
    if cfg.AdminJWTSecret != "secret" || cfg.DatabaseURL == "" || cfg.KcfgEncryptionKey == "" {
        t.Fatalf("secrets/DSN not set correctly: %+v", cfg)
    }
}

func TestConfigLoad_FileMergeAndOverride(t *testing.T) {
    dir := t.TempDir()
    file := filepath.Join(dir, "config.yaml")
    yaml := []byte("" +
        "env: production\n" +
        "port: 9090\n" +
        "logLevel: debug\n" +
        "adminJWTSecret: fromfile\n" +
        "disableAuth: true\n" +
        "kcfgEncryptionKey: filekey\n" +
        "databaseURL: postgres://file:pass@localhost:5432/db?sslmode=disable\n",
    )
    if err := os.WriteFile(file, yaml, 0o600); err != nil {
        t.Fatalf("write file: %v", err)
    }
    t.Setenv("CONFIG_FILE", file)
    // Override PORT via env to ensure env wins over file
    t.Setenv("PORT", "8081")

    cfg, err := config.Load()
    if err != nil {
        t.Fatalf("Load() error: %v", err)
    }
    if cfg.Env != "production" {
        t.Fatalf("expected env from file, got %q", cfg.Env)
    }
    if cfg.Port != 8081 { // env override wins
        t.Fatalf("expected port 8081 from env override, got %d", cfg.Port)
    }
    if !cfg.DisableAuth {
        t.Fatalf("expected DisableAuth true from file merge")
    }
    if cfg.KcfgEncryptionKey == "" || cfg.DatabaseURL == "" {
        t.Fatalf("expected keys from file merge: %+v", cfg)
    }
}

func TestConfigLoad_RequiresEncryptionKey(t *testing.T) {
    t.Setenv("KCFG_ENCRYPTION_KEY", "") // explicit empty should error
    t.Setenv("ADMIN_JWT_SECRET", "secret") // avoid auth error
    _, err := config.Load()
    if err == nil {
        t.Fatalf("expected error when KCFG_ENCRYPTION_KEY is empty")
    }
}

