//go:build integration

package models

import (
	"context"
	"os"
	"testing"
	"time"

	dbpkg "github.com/vaheed/kubeop/internal/db"
)

func TestTotalsSkipIfNoDB(t *testing.T) {
	dsn := os.Getenv("KUBEOP_DB_URL")
	if dsn == "" {
		t.Skip("no db url")
	}
	db, err := dbpkg.Connect(dsn)
	if err != nil {
		t.Skipf("db connect: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := db.Migrate(ctx); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	s := NewStore(db.DB)
	tot, err := s.Totals(ctx)
	if err != nil {
		t.Fatalf("totals: %v", err)
	}
	if tot.Totals["cpu_milli"] < 0 || tot.Totals["mem_mib"] < 0 {
		t.Fatalf("invalid totals: %+v", tot)
	}
}
