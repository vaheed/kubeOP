CREATE TABLE IF NOT EXISTS user_spaces (
    id UUID PRIMARY KEY,
    user_id UUID NOT NULL,
    cluster_id UUID NOT NULL,
    namespace TEXT NOT NULL,
    kubeconfig_enc BYTEA,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (user_id, cluster_id)
);

