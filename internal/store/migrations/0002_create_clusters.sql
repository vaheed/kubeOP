-- +goose Up
CREATE TABLE IF NOT EXISTS clusters (
    id UUID PRIMARY KEY,
    name TEXT NOT NULL UNIQUE,
    kubeconfig_enc BYTEA NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- +goose Down
DROP TABLE IF EXISTS clusters;

