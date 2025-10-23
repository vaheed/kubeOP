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
	// Tenancy / Projects
	t.Setenv("PROJECTS_IN_USER_NAMESPACE", "true")
	t.Setenv("PROJECT_LR_REQUEST_CPU", "25m")
	t.Setenv("PROJECT_LR_REQUEST_MEMORY", "64Mi")
	t.Setenv("PROJECT_LR_LIMIT_CPU", "500m")
	t.Setenv("PROJECT_LR_LIMIT_MEMORY", "512Mi")
	t.Setenv("CLUSTER_HEALTH_INTERVAL_SECONDS", "45")
	t.Setenv("CLUSTER_DEFAULT_ENVIRONMENT", "staging")
	t.Setenv("CLUSTER_DEFAULT_REGION", "eu-west")

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
	// project-level and scheduler envs applied
	if !cfg.ProjectsInUserNamespace {
		t.Fatalf("ProjectsInUserNamespace expected true")
	}
	if cfg.ProjectLRRequestCPU != "25m" || cfg.ProjectLRLimitMemory != "512Mi" {
		t.Fatalf("project LR envs not applied: %+v", cfg)
	}
	if cfg.ClusterHealthIntervalSeconds != 45 {
		t.Fatalf("scheduler interval not applied: %d", cfg.ClusterHealthIntervalSeconds)
	}
	if cfg.ClusterDefaultEnvironment != "staging" || cfg.ClusterDefaultRegion != "eu-west" {
		t.Fatalf("cluster defaults not applied: env=%q region=%q", cfg.ClusterDefaultEnvironment, cfg.ClusterDefaultRegion)
	}
}

func TestDefaultProjectsInUserNamespaceDefaultTrue(t *testing.T) {
	// Set required secrets to avoid validation errors
	t.Setenv("ADMIN_JWT_SECRET", "secret")
	t.Setenv("KCFG_ENCRYPTION_KEY", "key")
	// Don't set PROJECTS_IN_USER_NAMESPACE to use default
	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if !cfg.ProjectsInUserNamespace {
		t.Fatalf("expected default ProjectsInUserNamespace=true")
	}
}

func TestProjectsInUserNamespaceEnvOverrideFalse(t *testing.T) {
	t.Setenv("ADMIN_JWT_SECRET", "secret")
	t.Setenv("KCFG_ENCRYPTION_KEY", "key")
	t.Setenv("PROJECTS_IN_USER_NAMESPACE", "false")
	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if cfg.ProjectsInUserNamespace {
		t.Fatalf("expected ProjectsInUserNamespace=false when env override is set")
	}
}

func TestProjectLRDefaults(t *testing.T) {
	t.Setenv("ADMIN_JWT_SECRET", "secret")
	t.Setenv("KCFG_ENCRYPTION_KEY", "key")
	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if cfg.ProjectLRRequestCPU != "100m" {
		t.Fatalf("expected project default request cpu 100m, got %q", cfg.ProjectLRRequestCPU)
	}
	if cfg.ProjectLRRequestMemory != "128Mi" {
		t.Fatalf("expected project default request memory 128Mi, got %q", cfg.ProjectLRRequestMemory)
	}
	if cfg.ProjectLRLimitCPU != "1" {
		t.Fatalf("expected project default limit cpu 1, got %q", cfg.ProjectLRLimitCPU)
	}
	if cfg.ProjectLRLimitMemory != "1Gi" {
		t.Fatalf("expected project default limit memory 1Gi, got %q", cfg.ProjectLRLimitMemory)
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
		"databaseURL: postgres://file:pass@localhost:5432/db?sslmode=disable\n" +
		"projectsInUserNamespace: true\n",
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
	if !cfg.ProjectsInUserNamespace {
		t.Fatalf("expected ProjectsInUserNamespace true from file merge")
	}
	if cfg.KcfgEncryptionKey == "" || cfg.DatabaseURL == "" {
		t.Fatalf("expected keys from file merge: %+v", cfg)
	}
}

func TestConfigLoad_EventsBridgeDisabledByDefault(t *testing.T) {
	t.Setenv("ADMIN_JWT_SECRET", "secret")
	t.Setenv("KCFG_ENCRYPTION_KEY", "key")

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if cfg.EventsBridgeEnabled {
		t.Fatalf("expected EventsBridgeEnabled=false by default")
	}
}

func TestConfigLoad_EventsBridgeEnabledToggle(t *testing.T) {
	t.Setenv("ADMIN_JWT_SECRET", "secret")
	t.Setenv("KCFG_ENCRYPTION_KEY", "key")
	t.Setenv("EVENT_BRIDGE_ENABLED", "true")

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if !cfg.EventsBridgeEnabled {
		t.Fatalf("expected EventsBridgeEnabled=true when EVENT_BRIDGE_ENABLED=true")
	}
}

func TestConfigLoad_RequiresEncryptionKey(t *testing.T) {
	t.Setenv("KCFG_ENCRYPTION_KEY", "")    // explicit empty should error
	t.Setenv("ADMIN_JWT_SECRET", "secret") // avoid auth error
	_, err := config.Load()
	if err == nil {
		t.Fatalf("expected error when KCFG_ENCRYPTION_KEY is empty")
	}
}
