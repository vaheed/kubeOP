ALTER TABLE clusters
    DROP CONSTRAINT IF EXISTS clusters_last_status_fk;

DROP TABLE IF EXISTS cluster_status;

ALTER TABLE clusters
    DROP COLUMN IF EXISTS last_status_id,
    DROP COLUMN IF EXISTS last_seen,
    DROP COLUMN IF EXISTS tags,
    DROP COLUMN IF EXISTS description,
    DROP COLUMN IF EXISTS api_server,
    DROP COLUMN IF EXISTS region,
    DROP COLUMN IF EXISTS environment,
    DROP COLUMN IF EXISTS contact,
    DROP COLUMN IF EXISTS owner;
