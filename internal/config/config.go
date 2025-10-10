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
    Env     string `yaml:"env"`
    Port    int    `yaml:"port"`
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

// Load reads environment variables and optional YAML config file. Env overrides file.
func Load() (*Config, error) {
    cfg := &Config{
        Env:              getEnv("APP_ENV", "development"),
        Port:             getEnvInt("PORT", 8080),
        LogLevel:         getEnv("LOG_LEVEL", "info"),
        AdminJWTSecret:   getEnv("ADMIN_JWT_SECRET", "dev-admin-secret-change-me"),
        DisableAuth:      getEnvBool("DISABLE_AUTH", false),
        KcfgEncryptionKey: getEnv("KCFG_ENCRYPTION_KEY", "dev-not-secure-key"),
        DatabaseURL:      getEnv("DATABASE_URL", "postgres://postgres:postgres@localhost:5432/kubeop?sslmode=disable"),
        ConfigFile:       getEnv("CONFIG_FILE", ""),
    }

    // If YAML config file provided, load and merge (env wins)
    if cfg.ConfigFile != "" {
        if _, err := os.Stat(cfg.ConfigFile); err == nil {
            by, err := os.ReadFile(cfg.ConfigFile)
            if err != nil {
                return nil, fmt.Errorf("read config file: %w", err)
            }
            fileCfg := Config{}
            if err := yaml.Unmarshal(by, &fileCfg); err != nil {
                return nil, fmt.Errorf("parse config file: %w", err)
            }
            mergeConfig(cfg, &fileCfg)
        }
    }

    if strings.TrimSpace(cfg.AdminJWTSecret) == "" && !cfg.DisableAuth {
        return nil, errors.New("ADMIN_JWT_SECRET is required unless DISABLE_AUTH=true")
    }
    if strings.TrimSpace(cfg.KcfgEncryptionKey) == "" {
        return nil, errors.New("KCFG_ENCRYPTION_KEY is required")
    }

    return cfg, nil
}

func mergeConfig(dst, src *Config) {
    // Only set from src if dst has default/not set and env didn't override
    if dst.Env == "" && src.Env != "" { dst.Env = src.Env }
    if dst.Port == 0 && src.Port != 0 { dst.Port = src.Port }
    if dst.LogLevel == "" && src.LogLevel != "" { dst.LogLevel = src.LogLevel }
    if dst.AdminJWTSecret == "" && src.AdminJWTSecret != "" { dst.AdminJWTSecret = src.AdminJWTSecret }
    if !dst.DisableAuth && src.DisableAuth { dst.DisableAuth = true }
    if dst.KcfgEncryptionKey == "" && src.KcfgEncryptionKey != "" { dst.KcfgEncryptionKey = src.KcfgEncryptionKey }
    if dst.DatabaseURL == "" && src.DatabaseURL != "" { dst.DatabaseURL = src.DatabaseURL }
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

