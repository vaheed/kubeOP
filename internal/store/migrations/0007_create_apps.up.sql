CREATE TABLE IF NOT EXISTS apps (
    id UUID PRIMARY KEY,
    project_id UUID NOT NULL,
    name TEXT NOT NULL,
    status TEXT NOT NULL,
    source JSONB,
    repo TEXT,
    webhook_secret TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_apps_project_id ON apps(project_id);
CREATE INDEX IF NOT EXISTS idx_apps_repo ON apps(repo);
