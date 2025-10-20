ALTER TABLE clusters
    ADD COLUMN owner TEXT,
    ADD COLUMN contact TEXT,
    ADD COLUMN environment TEXT,
    ADD COLUMN region TEXT,
    ADD COLUMN api_server TEXT,
    ADD COLUMN description TEXT,
    ADD COLUMN tags JSONB NOT NULL DEFAULT '[]'::jsonb,
    ADD COLUMN last_seen TIMESTAMPTZ,
    ADD COLUMN last_status_id UUID;

CREATE TABLE cluster_status (
    id UUID PRIMARY KEY,
    cluster_id UUID NOT NULL REFERENCES clusters(id) ON DELETE CASCADE,
    healthy BOOLEAN NOT NULL,
    message TEXT,
    apiserver_version TEXT,
    node_count INTEGER,
    checked_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    details JSONB NOT NULL DEFAULT '{}'::jsonb
);

CREATE INDEX cluster_status_cluster_checked_at_idx
    ON cluster_status (cluster_id, checked_at DESC);

ALTER TABLE clusters
    ADD CONSTRAINT clusters_last_status_fk FOREIGN KEY (last_status_id) REFERENCES cluster_status(id) ON DELETE SET NULL;
