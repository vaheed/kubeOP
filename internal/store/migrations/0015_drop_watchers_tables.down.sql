CREATE TABLE IF NOT EXISTS watchers (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    cluster_id UUID NOT NULL REFERENCES clusters(id) ON DELETE CASCADE,
    refresh_token_hash TEXT NOT NULL,
    refresh_token_expires_at TIMESTAMPTZ NOT NULL,
    access_token_expires_at TIMESTAMPTZ NOT NULL,
    last_seen_at TIMESTAMPTZ,
    last_refresh_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    disabled BOOLEAN NOT NULL DEFAULT FALSE
);

CREATE UNIQUE INDEX IF NOT EXISTS watchers_cluster_id_key ON watchers(cluster_id);

CREATE TABLE IF NOT EXISTS k8s_objects (
    cluster_id UUID NOT NULL REFERENCES clusters(id) ON DELETE CASCADE,
    namespace TEXT,
    kind TEXT NOT NULL,
    name TEXT NOT NULL,
    uid TEXT NOT NULL,
    resource_version TEXT NOT NULL,
    labels JSONB DEFAULT '{}'::jsonb,
    annotations JSONB DEFAULT '{}'::jsonb,
    spec_hash TEXT,
    status_hash TEXT,
    desired_state JSONB,
    observed_state JSONB,
    last_seen_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ,
    PRIMARY KEY (cluster_id, uid, resource_version)
);

CREATE UNIQUE INDEX IF NOT EXISTS k8s_objects_identity_idx ON k8s_objects(cluster_id, namespace, kind, name);
CREATE INDEX IF NOT EXISTS k8s_objects_uid_idx ON k8s_objects(uid);
CREATE INDEX IF NOT EXISTS k8s_objects_last_seen_idx ON k8s_objects(last_seen_at);
