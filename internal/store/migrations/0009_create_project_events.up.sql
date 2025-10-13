CREATE TABLE IF NOT EXISTS project_events (
    id UUID PRIMARY KEY,
    project_id UUID NOT NULL,
    app_id UUID,
    actor_user_id TEXT,
    kind TEXT NOT NULL,
    severity TEXT NOT NULL,
    message TEXT NOT NULL,
    meta JSONB,
    at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_project_events_project_at_desc ON project_events(project_id, at DESC);
CREATE INDEX IF NOT EXISTS idx_project_events_actor_at_desc ON project_events(actor_user_id, at DESC);
CREATE INDEX IF NOT EXISTS idx_project_events_meta_gin ON project_events USING GIN (meta);
