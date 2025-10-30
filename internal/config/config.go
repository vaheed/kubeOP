package config

import (
    "encoding/base64"
    "errors"
    "os"
    "strconv"
)

type Config struct {
    DBURL         string
    RequireAuth   bool
    JWTKey        []byte
    KMSMasterKey  []byte
    HTTPAddr      string
    DevInsecure   bool
    DBMaxOpen     int
    DBMaxIdle     int
    DBConnMaxLife int // seconds
    DBTimeoutMS   int
}

func getenv(key, def string) string {
    if v := os.Getenv(key); v != "" {
        return v
    }
    return def
}

func Parse() (*Config, error) {
    cfg := &Config{}
    cfg.DBURL = getenv("KUBEOP_DB_URL", "")
    cfg.RequireAuth = getenv("KUBEOP_REQUIRE_AUTH", "false") == "true"
    cfg.HTTPAddr = getenv("KUBEOP_HTTP_ADDR", ":8080")
    cfg.DevInsecure = getenv("KUBEOP_DEV_INSECURE", "false") == "true"
    cfg.DBMaxOpen = atoi(getenv("KUBEOP_DB_MAX_OPEN", "10"))
    cfg.DBMaxIdle = atoi(getenv("KUBEOP_DB_MAX_IDLE", "5"))
    cfg.DBConnMaxLife = atoi(getenv("KUBEOP_DB_CONN_MAX_LIFETIME", "1800"))
    cfg.DBTimeoutMS = atoi(getenv("KUBEOP_DB_TIMEOUT_MS", "2000"))

    if v := getenv("KUBEOP_JWT_SIGNING_KEY", ""); v != "" && v != "REPLACE-ME" {
        b, err := base64.StdEncoding.DecodeString(v)
        if err != nil {
            return nil, err
        }
        cfg.JWTKey = b
    }
    if v := getenv("KUBEOP_KMS_MASTER_KEY", ""); v != "" && v != "REPLACE-ME" {
        b, err := base64.StdEncoding.DecodeString(v)
        if err != nil {
            return nil, err
        }
        cfg.KMSMasterKey = b
    }
    if err := cfg.Validate(); err != nil {
        return nil, err
    }
    return cfg, nil
}

func (c *Config) Validate() error {
    if c.DBURL == "" {
        return errors.New("KUBEOP_DB_URL is required")
    }
    if c.RequireAuth && len(c.JWTKey) == 0 && !c.DevInsecure {
        return errors.New("KUBEOP_JWT_SIGNING_KEY is required when auth is enabled")
    }
    if len(c.KMSMasterKey) == 0 && !c.DevInsecure {
        return errors.New("KUBEOP_KMS_MASTER_KEY is required")
    }
    return nil
}

func atoi(s string) int {
    n, _ := strconv.Atoi(s)
    return n
}
