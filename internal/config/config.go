package config

import (
    "errors"
    "fmt"
    "os"
    "strconv"
    "strings"

    "gopkg.in/yaml.v3"
)

type Config struct {
    // App
    Env      string `yaml:"env"`
    Port     int    `yaml:"port"`
    LogLevel string `yaml:"logLevel"`

    // Security
    AdminJWTSecret    string `yaml:"adminJWTSecret"`
    DisableAuth       bool   `yaml:"disableAuth"`
    KcfgEncryptionKey string `yaml:"kcfgEncryptionKey"`

    // DB
    DatabaseURL string `yaml:"databaseURL"`

    // Optional config file path (only from env)
    ConfigFile string `yaml:"-"`
}

// Load reads an optional YAML config file and environment variables.
// Precedence: defaults < file < environment variables.
func Load() (*Config, error) {
    cfg := &Config{}

    // 1) Read optional YAML config (path comes from env)
    cfg.ConfigFile = getEnv("CONFIG_FILE", "")
    if cfg.ConfigFile != "" {
        if _, err := os.Stat(cfg.ConfigFile); err == nil {
            by, err := os.ReadFile(cfg.ConfigFile)
            if err != nil {
                return nil, fmt.Errorf("read config file: %w", err)
            }
            if err := yaml.Unmarshal(by, cfg); err != nil {
                return nil, fmt.Errorf("parse config file: %w", err)
            }
        }
    }

    // 2) Apply defaults for any still-zero fields
    if strings.TrimSpace(cfg.Env) == "" { cfg.Env = "development" }
    if cfg.Port == 0 { cfg.Port = 8080 }
    if strings.TrimSpace(cfg.LogLevel) == "" { cfg.LogLevel = "info" }
    if strings.TrimSpace(cfg.AdminJWTSecret) == "" { cfg.AdminJWTSecret = "dev-admin-secret-change-me" }
    if strings.TrimSpace(cfg.KcfgEncryptionKey) == "" { cfg.KcfgEncryptionKey = "dev-not-secure-key" }
    if strings.TrimSpace(cfg.DatabaseURL) == "" { cfg.DatabaseURL = "postgres://postgres:postgres@localhost:5432/kubeop?sslmode=disable" }

    // 3) Override from environment
    cfg.Env = getEnv("APP_ENV", cfg.Env)
    cfg.Port = getEnvInt("PORT", cfg.Port)
    cfg.LogLevel = getEnv("LOG_LEVEL", cfg.LogLevel)
    cfg.AdminJWTSecret = getEnv("ADMIN_JWT_SECRET", cfg.AdminJWTSecret)
    cfg.DisableAuth = getEnvBool("DISABLE_AUTH", cfg.DisableAuth)
    cfg.KcfgEncryptionKey = getEnv("KCFG_ENCRYPTION_KEY", cfg.KcfgEncryptionKey)
    cfg.DatabaseURL = getEnv("DATABASE_URL", cfg.DatabaseURL)

    // 4) Validation
    if strings.TrimSpace(cfg.AdminJWTSecret) == "" && !cfg.DisableAuth {
        return nil, errors.New("ADMIN_JWT_SECRET is required unless DISABLE_AUTH=true")
    }
    if strings.TrimSpace(cfg.KcfgEncryptionKey) == "" {
        return nil, errors.New("KCFG_ENCRYPTION_KEY is required")
    }

    return cfg, nil
}

func getEnv(key, def string) string {
    if v, ok := os.LookupEnv(key); ok {
        return v
    }
    return def
}

func getEnvInt(key string, def int) int {
    if v, ok := os.LookupEnv(key); ok {
        if i, err := strconv.Atoi(v); err == nil {
            return i
        }
    }
    return def
}

func getEnvBool(key string, def bool) bool {
    if v, ok := os.LookupEnv(key); ok {
        switch strings.ToLower(v) {
        case "1", "true", "yes", "y":
            return true
        case "0", "false", "no", "n":
            return false
        }
    }
    return def
}

