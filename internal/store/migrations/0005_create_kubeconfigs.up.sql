CREATE TABLE IF NOT EXISTS kubeconfigs (
    id UUID PRIMARY KEY,
    cluster_id UUID NOT NULL,
    namespace TEXT NOT NULL,
    user_id UUID NOT NULL,
    project_id UUID,
    service_account TEXT NOT NULL,
    secret_name TEXT NOT NULL,
    kubeconfig_enc BYTEA NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX IF NOT EXISTS kubeconfigs_user_scope
    ON kubeconfigs(cluster_id, namespace, user_id)
    WHERE project_id IS NULL;

CREATE UNIQUE INDEX IF NOT EXISTS kubeconfigs_project_scope
    ON kubeconfigs(cluster_id, namespace, project_id)
    WHERE project_id IS NOT NULL;
