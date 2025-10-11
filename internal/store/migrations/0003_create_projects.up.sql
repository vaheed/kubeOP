CREATE TABLE IF NOT EXISTS projects (
    id UUID PRIMARY KEY,
    user_id UUID NOT NULL,
    cluster_id UUID NOT NULL,
    name TEXT NOT NULL,
    namespace TEXT NOT NULL,
    suspended BOOLEAN NOT NULL DEFAULT FALSE,
    quota_overrides JSONB,
    kubeconfig_enc BYTEA,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

