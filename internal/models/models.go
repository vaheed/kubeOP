package models

import (
    "context"
    "database/sql"
    "errors"
    "time"
)

type Tenant struct {
    ID        string    `json:"id"`
    Name      string    `json:"name"`
    CreatedAt time.Time `json:"created_at"`
}

type Project struct {
    ID        string    `json:"id"`
    TenantID  string    `json:"tenant_id"`
    Name      string    `json:"name"`
    CreatedAt time.Time `json:"created_at"`
}

type App struct {
    ID        string    `json:"id"`
    ProjectID string    `json:"project_id"`
    Name      string    `json:"name"`
    Image     string    `json:"image,omitempty"`
    Host      string    `json:"host,omitempty"`
    CreatedAt time.Time `json:"created_at"`
}

type Store struct { DB *sql.DB }

func NewStore(db *sql.DB) *Store { return &Store{DB: db} }

func (s *Store) CreateTenant(ctx context.Context, name string) (*Tenant, error) {
    var t Tenant
    err := s.DB.QueryRowContext(ctx, `INSERT INTO tenants(name) VALUES($1) RETURNING id,name,created_at`, name).Scan(&t.ID, &t.Name, &t.CreatedAt)
    if err != nil { return nil, err }
    return &t, nil
}
func (s *Store) GetTenant(ctx context.Context, id string) (*Tenant, error) {
    var t Tenant
    err := s.DB.QueryRowContext(ctx, `SELECT id,name,created_at FROM tenants WHERE id=$1`, id).Scan(&t.ID, &t.Name, &t.CreatedAt)
    if errors.Is(err, sql.ErrNoRows) { return nil, nil }
    return &t, err
}

func (s *Store) DeleteTenant(ctx context.Context, id string) error {
    _, err := s.DB.ExecContext(ctx, `DELETE FROM tenants WHERE id=$1`, id)
    return err
}

func (s *Store) CreateProject(ctx context.Context, tenantID, name string) (*Project, error) {
    var p Project
    err := s.DB.QueryRowContext(ctx, `INSERT INTO projects(tenant_id,name) VALUES($1,$2) RETURNING id,tenant_id,name,created_at`, tenantID, name).Scan(&p.ID, &p.TenantID, &p.Name, &p.CreatedAt)
    if err != nil { return nil, err }
    return &p, nil
}

func (s *Store) GetProject(ctx context.Context, id string) (*Project, error) {
    var p Project
    err := s.DB.QueryRowContext(ctx, `SELECT id,tenant_id,name,created_at FROM projects WHERE id=$1`, id).Scan(&p.ID, &p.TenantID, &p.Name, &p.CreatedAt)
    if errors.Is(err, sql.ErrNoRows) { return nil, nil }
    return &p, err
}

func (s *Store) DeleteProject(ctx context.Context, id string) error {
    _, err := s.DB.ExecContext(ctx, `DELETE FROM projects WHERE id=$1`, id)
    return err
}

func (s *Store) CreateApp(ctx context.Context, projectID, name, image, host string) (*App, error) {
    var a App
    err := s.DB.QueryRowContext(ctx, `INSERT INTO apps(project_id,name,image,host) VALUES($1,$2,$3,$4) RETURNING id,project_id,name,image,host,created_at`, projectID, name, image, host).Scan(&a.ID, &a.ProjectID, &a.Name, &a.Image, &a.Host, &a.CreatedAt)
    if err != nil { return nil, err }
    return &a, nil
}

func (s *Store) GetApp(ctx context.Context, id string) (*App, error) {
    var a App
    err := s.DB.QueryRowContext(ctx, `SELECT id,project_id,name,image,host,created_at FROM apps WHERE id=$1`, id).Scan(&a.ID, &a.ProjectID, &a.Name, &a.Image, &a.Host, &a.CreatedAt)
    if errors.Is(err, sql.ErrNoRows) { return nil, nil }
    return &a, err
}

func (s *Store) DeleteApp(ctx context.Context, id string) error {
    _, err := s.DB.ExecContext(ctx, `DELETE FROM apps WHERE id=$1`, id)
    return err
}

type UsageLine struct {
    TS       time.Time `json:"ts"`
    TenantID string    `json:"tenant_id"`
    CPUm    int64     `json:"cpu_milli"`
    MemMiB  int64     `json:"mem_mib"`
}

func (s *Store) AddUsageHour(ctx context.Context, ts time.Time, tenantID string, cpuMilli, memMiB int64) error {
    _, err := s.DB.ExecContext(ctx, `INSERT INTO usage_hourly(ts,tenant_id,cpu_milli,mem_mib) VALUES($1,$2,$3,$4) ON CONFLICT (ts,tenant_id) DO UPDATE SET cpu_milli=usage_hourly.cpu_milli+EXCLUDED.cpu_milli, mem_mib=usage_hourly.mem_mib+EXCLUDED.mem_mib`, ts, tenantID, cpuMilli, memMiB)
    return err
}

func (s *Store) AddUsageRaw(ctx context.Context, ts time.Time, tenantID string, cpuMilli, memMiB int64) error {
    _, err := s.DB.ExecContext(ctx, `INSERT INTO usage_raw(ts,tenant_id,cpu_milli,mem_mib) VALUES($1,$2,$3,$4)`, ts, tenantID, cpuMilli, memMiB)
    return err
}

func (s *Store) Invoice(ctx context.Context, tenantID string, start, end time.Time) ([]UsageLine, error) {
    rows, err := s.DB.QueryContext(ctx, `SELECT ts, tenant_id, cpu_milli, mem_mib FROM usage_hourly WHERE tenant_id=$1 AND ts >= $2 AND ts < $3 ORDER BY ts`, tenantID, start, end)
    if err != nil { return nil, err }
    defer rows.Close()
    var out []UsageLine
    for rows.Next() {
        var u UsageLine
        if err := rows.Scan(&u.TS, &u.TenantID, &u.CPUm, &u.MemMiB); err != nil { return nil, err }
        out = append(out, u)
    }
    return out, rows.Err()
}

type Totals struct {
    Totals map[string]int64 `json:"totals"`
}

func (s *Store) Totals(ctx context.Context) (*Totals, error) {
    var cpu, mem sql.NullInt64
    if err := s.DB.QueryRowContext(ctx, `SELECT COALESCE(sum(cpu_milli),0), COALESCE(sum(mem_mib),0) FROM usage_hourly`).Scan(&cpu, &mem); err != nil {
        return nil, err
    }
    return &Totals{Totals: map[string]int64{"cpu_milli": cpu.Int64, "mem_mib": mem.Int64}}, nil
}

// Optional per-tenant rates
type TenantRate struct {
    TenantID string
    CPUmRate float64
    MemMiBRate float64
    Tier string
}

func (s *Store) GetTenantRate(ctx context.Context, tenantID string) (*TenantRate, error) {
    var tr TenantRate
    err := s.DB.QueryRowContext(ctx, `SELECT tenant_id, cpu_milli_rate, mem_mib_rate, tier FROM tenant_rates WHERE tenant_id=$1 ORDER BY effective_from DESC LIMIT 1`, tenantID).Scan(&tr.TenantID, &tr.CPUmRate, &tr.MemMiBRate, &tr.Tier)
    if errors.Is(err, sql.ErrNoRows) { return nil, nil }
    return &tr, err
}

// CRUD lists and updates
func (s *Store) ListTenants(ctx context.Context) ([]Tenant, error) {
    rows, err := s.DB.QueryContext(ctx, `SELECT id,name,created_at FROM tenants ORDER BY created_at DESC`)
    if err != nil { return nil, err }
    defer rows.Close()
    var out []Tenant
    for rows.Next() {
        var t Tenant
        if err := rows.Scan(&t.ID, &t.Name, &t.CreatedAt); err != nil { return nil, err }
        out = append(out, t)
    }
    return out, rows.Err()
}

func (s *Store) ListProjects(ctx context.Context, tenantID string) ([]Project, error) {
    var rows *sql.Rows
    var err error
    if tenantID == "" {
        rows, err = s.DB.QueryContext(ctx, `SELECT id,tenant_id,name,created_at FROM projects ORDER BY created_at DESC`)
    } else {
        rows, err = s.DB.QueryContext(ctx, `SELECT id,tenant_id,name,created_at FROM projects WHERE tenant_id=$1 ORDER BY created_at DESC`, tenantID)
    }
    if err != nil { return nil, err }
    defer rows.Close()
    var out []Project
    for rows.Next() {
        var p Project
        if err := rows.Scan(&p.ID, &p.TenantID, &p.Name, &p.CreatedAt); err != nil { return nil, err }
        out = append(out, p)
    }
    return out, rows.Err()
}

func (s *Store) ListApps(ctx context.Context, projectID string) ([]App, error) {
    var rows *sql.Rows
    var err error
    if projectID == "" {
        rows, err = s.DB.QueryContext(ctx, `SELECT id,project_id,name,image,host,created_at FROM apps ORDER BY created_at DESC`)
    } else {
        rows, err = s.DB.QueryContext(ctx, `SELECT id,project_id,name,image,host,created_at FROM apps WHERE project_id=$1 ORDER BY created_at DESC`, projectID)
    }
    if err != nil { return nil, err }
    defer rows.Close()
    var out []App
    for rows.Next() {
        var a App
        if err := rows.Scan(&a.ID, &a.ProjectID, &a.Name, &a.Image, &a.Host, &a.CreatedAt); err != nil { return nil, err }
        out = append(out, a)
    }
    return out, rows.Err()
}

func (s *Store) UpdateTenant(ctx context.Context, id, name string) error {
    _, err := s.DB.ExecContext(ctx, `UPDATE tenants SET name=$2 WHERE id=$1`, id, name)
    return err
}
func (s *Store) UpdateProject(ctx context.Context, id, name string) error {
    _, err := s.DB.ExecContext(ctx, `UPDATE projects SET name=$2 WHERE id=$1`, id, name)
    return err
}
func (s *Store) UpdateApp(ctx context.Context, id, name, image, host string) error {
    _, err := s.DB.ExecContext(ctx, `UPDATE apps SET name=$2, image=$3, host=$4 WHERE id=$1`, id, name, image, host)
    return err
}
