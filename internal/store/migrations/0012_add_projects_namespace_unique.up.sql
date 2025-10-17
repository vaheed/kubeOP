CREATE UNIQUE INDEX IF NOT EXISTS idx_projects_cluster_namespace
    ON projects (cluster_id, namespace)
    WHERE deleted_at IS NULL;
