package usage

import (
    "context"
    "database/sql"
    "log/slog"
    "time"
)

type Aggregator struct {
    Log *slog.Logger
    DB  *sql.DB
}

// RunOnce aggregates raw samples into hourly buckets for the last hour.
func (a *Aggregator) RunOnce(ctx context.Context) error {
    hour := time.Now().UTC().Add(-1 * time.Hour).Truncate(time.Hour)
    _, err := a.DB.ExecContext(ctx, `
        INSERT INTO usage_hourly(ts, tenant_id, cpu_milli, mem_mib)
        SELECT date_trunc('hour', ts) AS ts, tenant_id, SUM(cpu_milli), SUM(mem_mib)
        FROM usage_raw WHERE ts >= $1 AND ts < $2
        GROUP BY date_trunc('hour', ts), tenant_id
        ON CONFLICT (ts, tenant_id)
        DO UPDATE SET cpu_milli = usage_hourly.cpu_milli + EXCLUDED.cpu_milli,
                      mem_mib  = usage_hourly.mem_mib  + EXCLUDED.mem_mib
    `, hour, hour.Add(time.Hour))
    if err != nil { return err }
    // Cleanup processed raw samples for that hour
    _, _ = a.DB.ExecContext(ctx, `DELETE FROM usage_raw WHERE ts >= $1 AND ts < $2`, hour, hour.Add(time.Hour))
    return nil
}

