CREATE TABLE IF NOT EXISTS releases (
    id UUID PRIMARY KEY,
    project_id UUID NOT NULL REFERENCES projects(id),
    app_id UUID NOT NULL REFERENCES apps(id),
    source TEXT NOT NULL,
    spec_digest TEXT NOT NULL,
    render_digest TEXT NOT NULL,
    spec JSONB NOT NULL,
    rendered_objects JSONB NOT NULL,
    load_balancers JSONB NOT NULL,
    warnings JSONB NOT NULL DEFAULT '[]'::jsonb,
    helm_chart TEXT,
    helm_values JSONB NOT NULL DEFAULT '{}'::jsonb,
    helm_render_sha TEXT,
    manifests_sha TEXT,
    repo TEXT,
    status TEXT NOT NULL DEFAULT 'succeeded',
    message TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_releases_app_created_at ON releases(app_id, created_at DESC, id DESC);
CREATE INDEX IF NOT EXISTS idx_releases_project_app ON releases(project_id, app_id);
