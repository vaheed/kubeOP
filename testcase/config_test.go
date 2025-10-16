package testcase

import (
	"os"
	"path/filepath"
	"strings"
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

func TestWatcherDefaultsDeriveFromBaseURL(t *testing.T) {
	t.Setenv("ADMIN_JWT_SECRET", "secret")
	t.Setenv("KCFG_ENCRYPTION_KEY", "key")
	t.Setenv("KUBEOP_BASE_URL", "https://kubeop.example.com")

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("Load(): %v", err)
	}
	if !cfg.WatcherAutoDeploy {
		t.Fatalf("expected watcher auto deploy enabled by default")
	}
	expectedURL := "https://kubeop.example.com/v1/events/ingest"
	if cfg.WatcherEventsURL != expectedURL {
		t.Fatalf("expected watcher events url %q, got %q", expectedURL, cfg.WatcherEventsURL)
	}
	if cfg.WatcherNamespace != "kubeop-system" {
		t.Fatalf("expected watcher namespace default kubeop-system, got %q", cfg.WatcherNamespace)
	}
	if cfg.WatcherDeploymentName != "kubeop-watcher" {
		t.Fatalf("expected watcher deployment default kubeop-watcher, got %q", cfg.WatcherDeploymentName)
	}
	if !cfg.WatcherNamespaceCreate {
		t.Fatalf("expected watcher namespace creation default true")
	}
	if cfg.WatcherBatchMax != 200 || cfg.WatcherBatchWindowMillis != 1000 {
		t.Fatalf("expected watcher batching defaults, got max=%d window=%d", cfg.WatcherBatchMax, cfg.WatcherBatchWindowMillis)
	}
	if cfg.WatcherAutoDeploySource != "base-url" {
		t.Fatalf("expected watcher auto deploy source base-url, got %q", cfg.WatcherAutoDeploySource)
	}
	if !strings.Contains(cfg.WatcherAutoDeployExplanation(), "KUBEOP_BASE_URL") {
		t.Fatalf("expected watcher explanation to reference KUBEOP_BASE_URL, got %q", cfg.WatcherAutoDeployExplanation())
	}
}

func TestWatcherAutoDeployConfigFileDefaults(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "config.yaml")
	yaml := []byte("" +
		"baseURL: https://kubeop.example.com\n" +
		"adminJWTSecret: secret\n" +
		"kcfgEncryptionKey: key\n",
	)
	if err := os.WriteFile(file, yaml, 0o600); err != nil {
		t.Fatalf("write file: %v", err)
	}
	t.Setenv("CONFIG_FILE", file)

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("Load(): %v", err)
	}
	if !cfg.WatcherAutoDeploy {
		t.Fatalf("expected watcher auto deploy enabled when publicURL configured in file")
	}
	if cfg.WatcherAutoDeploySource != "base-url" {
		t.Fatalf("expected watcher auto deploy source base-url, got %q", cfg.WatcherAutoDeploySource)
	}
}

func TestWatcherAutoDeployRespectsExplicitConfigFileFalse(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "config.yaml")
	yaml := []byte("" +
		"baseURL: https://kubeop.example.com\n" +
		"adminJWTSecret: secret\n" +
		"kcfgEncryptionKey: key\n" +
		"watcherAutoDeploy: false\n",
	)
	if err := os.WriteFile(file, yaml, 0o600); err != nil {
		t.Fatalf("write file: %v", err)
	}
	t.Setenv("CONFIG_FILE", file)

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("Load(): %v", err)
	}
	if cfg.WatcherAutoDeploy {
		t.Fatalf("expected watcher auto deploy to remain disabled when explicitly set false in file")
	}
	if cfg.WatcherAutoDeploySource != "config" {
		t.Fatalf("expected watcher auto deploy source config, got %q", cfg.WatcherAutoDeploySource)
	}
	if !strings.Contains(cfg.WatcherAutoDeployExplanation(), "disabled by watcherAutoDeploy") {
		t.Fatalf("expected watcher explanation to reference config disable, got %q", cfg.WatcherAutoDeployExplanation())
	}
}

func TestWatcherAutoDeployDisabledWithoutBaseURL(t *testing.T) {
	t.Setenv("ADMIN_JWT_SECRET", "secret")
	t.Setenv("KCFG_ENCRYPTION_KEY", "key")
	// Ensure no KUBEOP_BASE_URL or watcher overrides are set
	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("Load(): %v", err)
	}
	if cfg.WatcherAutoDeploy {
		t.Fatalf("expected watcher auto deploy disabled when KUBEOP_BASE_URL is empty")
	}
	if cfg.WatcherEventsURL != "" {
		t.Fatalf("expected watcher events URL empty without KUBEOP_BASE_URL, got %q", cfg.WatcherEventsURL)
	}
	if cfg.WatcherAutoDeploySource != "default" {
		t.Fatalf("expected watcher auto deploy source default, got %q", cfg.WatcherAutoDeploySource)
	}
	if !strings.Contains(cfg.WatcherAutoDeployExplanation(), "KUBEOP_BASE_URL") {
		t.Fatalf("expected watcher explanation to mention KUBEOP_BASE_URL requirement, got %q", cfg.WatcherAutoDeployExplanation())
	}
}

func TestWatcherAutoDeployEnvOverrideDisables(t *testing.T) {
	t.Setenv("ADMIN_JWT_SECRET", "secret")
	t.Setenv("KCFG_ENCRYPTION_KEY", "key")
	t.Setenv("KUBEOP_BASE_URL", "https://kubeop.example.com")
	t.Setenv("WATCHER_AUTO_DEPLOY", "false")

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("Load(): %v", err)
	}
	if cfg.WatcherAutoDeploy {
		t.Fatalf("expected watcher auto deploy disabled due to env override")
	}
	if cfg.WatcherAutoDeploySource != "env" {
		t.Fatalf("expected watcher auto deploy source env, got %q", cfg.WatcherAutoDeploySource)
	}
	if !strings.Contains(cfg.WatcherAutoDeployExplanation(), "WATCHER_AUTO_DEPLOY") {
		t.Fatalf("expected watcher explanation to mention env override, got %q", cfg.WatcherAutoDeployExplanation())
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

func TestConfigLoad_RequiresEncryptionKey(t *testing.T) {
	t.Setenv("KCFG_ENCRYPTION_KEY", "")    // explicit empty should error
	t.Setenv("ADMIN_JWT_SECRET", "secret") // avoid auth error
	_, err := config.Load()
	if err == nil {
		t.Fatalf("expected error when KCFG_ENCRYPTION_KEY is empty")
	}
}
