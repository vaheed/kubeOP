package db

import (
    "context"
    "database/sql"
    "embed"
    "fmt"
    "strings"
    "time"

    _ "github.com/jackc/pgx/v5/stdlib"
)

//go:embed migrations/*.sql
var migrations embed.FS

type DB struct {
    *sql.DB
}

func Connect(url string) (*DB, error) {
    db, err := sql.Open("pgx", url)
    if err != nil {
        return nil, err
    }
    // defaults; allow caller to tune via exported setters
    db.SetMaxOpenConns(10)
    db.SetMaxIdleConns(5)
    db.SetConnMaxLifetime(30 * time.Minute)
    return &DB{DB: db}, nil
}

func (d *DB) Ping(ctx context.Context) error { return d.DB.PingContext(ctx) }

func (d *DB) Migrate(ctx context.Context) error {
    if _, err := d.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS schema_migrations (version text primary key)`); err != nil {
        return err
    }
    entries, err := migrations.ReadDir("migrations")
    if err != nil {
        return err
    }
    for _, e := range entries {
        if e.IsDir() || !strings.HasSuffix(e.Name(), ".sql") {
            continue
        }
        version := e.Name()
        var exists bool
        if err := d.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM schema_migrations WHERE version=$1)`, version).Scan(&exists); err != nil {
            return err
        }
        if exists {
            continue
        }
        b, err := migrations.ReadFile("migrations/" + version)
        if err != nil {
            return err
        }
        if _, err := d.ExecContext(ctx, string(b)); err != nil {
            return fmt.Errorf("migration %s failed: %w", version, err)
        }
        if _, err := d.ExecContext(ctx, `INSERT INTO schema_migrations(version) VALUES($1)`, version); err != nil {
            return err
        }
    }
    return nil
}

func (d *DB) ConfigurePool(maxOpen, maxIdle, maxLifeSeconds int) {
    if maxOpen > 0 { d.DB.SetMaxOpenConns(maxOpen) }
    if maxIdle >= 0 { d.DB.SetMaxIdleConns(maxIdle) }
    if maxLifeSeconds > 0 { d.DB.SetConnMaxLifetime(time.Duration(maxLifeSeconds) * time.Second) }
}
