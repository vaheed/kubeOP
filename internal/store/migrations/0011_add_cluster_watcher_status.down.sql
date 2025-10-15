ALTER TABLE clusters
    DROP COLUMN IF EXISTS watcher_health_deadline,
    DROP COLUMN IF EXISTS watcher_ready_at,
    DROP COLUMN IF EXISTS watcher_status_updated_at,
    DROP COLUMN IF EXISTS watcher_status_message,
    DROP COLUMN IF EXISTS watcher_status;
