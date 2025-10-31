//go:build integration

package db

import (
	"context"
	"os"
	"testing"
	"time"
)

func TestMigrateAndPing(t *testing.T) {
	dsn := os.Getenv("KUBEOP_DB_URL")
	if dsn == "" {
		t.Skip("KUBEOP_DB_URL not set")
	}
	database, err := Connect(dsn)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(func() { _ = database.Close() })
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := database.Migrate(ctx); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	if err := database.Ping(ctx); err != nil {
		t.Fatalf("ping: %v", err)
	}

	var count int
	if err := database.QueryRowContext(ctx, `SELECT COUNT(*) FROM schema_migrations`).Scan(&count); err != nil {
		t.Fatalf("check migrations: %v", err)
	}
	if count == 0 {
		t.Fatalf("expected migrations to be recorded")
	}
}

func TestConfigurePool(t *testing.T) {
	dsn := os.Getenv("KUBEOP_DB_URL")
	if dsn == "" {
		t.Skip("KUBEOP_DB_URL not set")
	}
	database, err := Connect(dsn)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(func() { _ = database.Close() })
	database.ConfigurePool(5, 3, 10)
	stats := database.Stats()
	if stats.MaxOpenConnections != 5 {
		t.Fatalf("expected max open 5, got %d", stats.MaxOpenConnections)
	}
}
