CREATE TABLE IF NOT EXISTS maintenance_state (
    id TEXT PRIMARY KEY,
    enabled BOOLEAN NOT NULL DEFAULT FALSE,
    message TEXT NOT NULL DEFAULT '',
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_by TEXT NOT NULL DEFAULT ''
);

INSERT INTO maintenance_state (id, enabled, message, updated_by)
VALUES ('global', FALSE, '', '')
ON CONFLICT (id) DO UPDATE SET
    enabled = EXCLUDED.enabled,
    message = EXCLUDED.message,
    updated_at = NOW(),
    updated_by = EXCLUDED.updated_by;
