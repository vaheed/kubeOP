package main

import (
	"testing"
)

func TestLoadConfigDerivesBaseFromEvents(t *testing.T) {
	t.Setenv("WATCHER_EVENTS_URL", "https://api.example.com:8443/v1/events/ingest")
	t.Setenv("CLUSTER_ID", "cluster-123")
	cfg, err := loadConfig()
	if err != nil {
		t.Fatalf("loadConfig error: %v", err)
	}
	if cfg.BaseURL != "https://api.example.com:8443" {
		t.Fatalf("expected base url with host+port, got %q", cfg.BaseURL)
	}
	if cfg.EventsURL != "https://api.example.com:8443/v1/events/ingest" {
		t.Fatalf("unexpected events url %q", cfg.EventsURL)
	}
	if cfg.HandshakeURL != "https://api.example.com:8443/v1/watchers/handshake" {
		t.Fatalf("unexpected handshake url %q", cfg.HandshakeURL)
	}
}

func TestLoadConfigRejectsMismatchedHosts(t *testing.T) {
	t.Setenv("KUBEOP_BASE_URL", "https://internal.example.com")
	t.Setenv("WATCHER_EVENTS_URL", "https://public.example.com/v1/events/ingest")
	t.Setenv("CLUSTER_ID", "cluster-123")
	if _, err := loadConfig(); err == nil {
		t.Fatalf("expected error when hosts differ")
	}
}

func TestLoadConfigRejectsHTTPWithoutOverride(t *testing.T) {
	t.Setenv("KUBEOP_BASE_URL", "http://api.example.com:8080")
	t.Setenv("CLUSTER_ID", "cluster-123")
	if _, err := loadConfig(); err == nil {
		t.Fatalf("expected error when http used without override")
	}
}

func TestLoadConfigAllowsHTTPWithOverride(t *testing.T) {
	t.Setenv("KUBEOP_BASE_URL", "http://api.example.com:8080")
	t.Setenv("ALLOW_INSECURE_HTTP", "true")
	t.Setenv("CLUSTER_ID", "cluster-123")
	cfg, err := loadConfig()
	if err != nil {
		t.Fatalf("loadConfig error: %v", err)
	}
	if cfg.BaseURL != "http://api.example.com:8080" {
		t.Fatalf("expected http base url, got %q", cfg.BaseURL)
	}
	if cfg.EventsURL != "http://api.example.com:8080/v1/events/ingest" {
		t.Fatalf("expected http events url, got %q", cfg.EventsURL)
	}
}

func TestLoadConfigRequiresClusterID(t *testing.T) {
	t.Setenv("KUBEOP_BASE_URL", "https://api.example.com")
	if _, err := loadConfig(); err == nil {
		t.Fatalf("expected error when CLUSTER_ID missing")
	}
}

func TestLoadConfigPrefersBaseWhenEventsMissing(t *testing.T) {
	t.Setenv("KUBEOP_BASE_URL", "https://api.example.com")
	t.Setenv("CLUSTER_ID", "cluster-123")
	cfg, err := loadConfig()
	if err != nil {
		t.Fatalf("loadConfig error: %v", err)
	}
	if cfg.EventsURL != "https://api.example.com/v1/events/ingest" {
		t.Fatalf("unexpected events url %q", cfg.EventsURL)
	}
}

func TestLoadConfigRequiresBaseOrEvents(t *testing.T) {
	t.Setenv("CLUSTER_ID", "cluster-123")
	if _, err := loadConfig(); err == nil {
		t.Fatalf("expected error when URLs missing")
	}
}

func TestLoadConfigRejectsEventsWithWrongPath(t *testing.T) {
	t.Setenv("WATCHER_EVENTS_URL", "https://api.example.com/v1/other")
	t.Setenv("CLUSTER_ID", "cluster-123")
	if _, err := loadConfig(); err == nil {
		t.Fatalf("expected error for events path mismatch")
	}
}

func TestLoadConfigRejectsBaseWithPath(t *testing.T) {
	t.Setenv("KUBEOP_BASE_URL", "https://api.example.com/api")
	t.Setenv("CLUSTER_ID", "cluster-123")
	if _, err := loadConfig(); err == nil {
		t.Fatalf("expected error for base url path")
	}
}

func TestLoadConfigUsesLegacyEventsEnv(t *testing.T) {
	t.Setenv("KUBEOP_EVENTS_URL", "https://api.example.com/v1/events/ingest")
	t.Setenv("CLUSTER_ID", "cluster-123")
	cfg, err := loadConfig()
	if err != nil {
		t.Fatalf("loadConfig error: %v", err)
	}
	if cfg.BaseURL != "https://api.example.com" {
		t.Fatalf("expected base derived from legacy env, got %q", cfg.BaseURL)
	}
}
