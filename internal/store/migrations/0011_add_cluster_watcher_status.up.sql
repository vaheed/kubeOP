ALTER TABLE clusters
    ADD COLUMN watcher_status TEXT NOT NULL DEFAULT 'Pending',
    ADD COLUMN watcher_status_message TEXT,
    ADD COLUMN watcher_status_updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    ADD COLUMN watcher_ready_at TIMESTAMPTZ,
    ADD COLUMN watcher_health_deadline TIMESTAMPTZ NOT NULL DEFAULT (now() + INTERVAL '3 minutes');
