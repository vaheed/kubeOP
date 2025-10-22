CREATE TABLE IF NOT EXISTS k8s_crds (
    id UUID PRIMARY KEY,
    cluster_id UUID NOT NULL REFERENCES clusters(id) ON DELETE CASCADE,
    project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    kind TEXT NOT NULL,
    namespace TEXT NOT NULL,
    name TEXT NOT NULL,
    uid TEXT NOT NULL,
    resource_version TEXT NOT NULL,
    spec_hash TEXT NOT NULL,
    spec JSONB NOT NULL,
    status JSONB NOT NULL,
    deleted_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX IF NOT EXISTS k8s_crds_identity_idx ON k8s_crds(cluster_id, kind, namespace, name);
CREATE INDEX IF NOT EXISTS k8s_crds_project_idx ON k8s_crds(project_id);
