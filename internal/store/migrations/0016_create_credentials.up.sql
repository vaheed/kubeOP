CREATE TABLE IF NOT EXISTS git_credentials (
    id UUID PRIMARY KEY,
    name TEXT NOT NULL,
    user_id UUID REFERENCES users(id) ON DELETE CASCADE,
    project_id UUID REFERENCES projects(id) ON DELETE CASCADE,
    auth_type TEXT NOT NULL CHECK (auth_type IN ('TOKEN', 'BASIC', 'SSH')),
    username TEXT,
    secret_enc BYTEA NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CHECK (
        (user_id IS NOT NULL AND project_id IS NULL)
        OR (user_id IS NULL AND project_id IS NOT NULL)
    )
);

CREATE UNIQUE INDEX IF NOT EXISTS git_credentials_user_name_idx
    ON git_credentials (user_id, name)
    WHERE user_id IS NOT NULL;

CREATE UNIQUE INDEX IF NOT EXISTS git_credentials_project_name_idx
    ON git_credentials (project_id, name)
    WHERE project_id IS NOT NULL;

CREATE TABLE IF NOT EXISTS registry_credentials (
    id UUID PRIMARY KEY,
    name TEXT NOT NULL,
    registry TEXT NOT NULL,
    user_id UUID REFERENCES users(id) ON DELETE CASCADE,
    project_id UUID REFERENCES projects(id) ON DELETE CASCADE,
    auth_type TEXT NOT NULL CHECK (auth_type IN ('TOKEN', 'BASIC')),
    username TEXT,
    secret_enc BYTEA NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CHECK (
        (user_id IS NOT NULL AND project_id IS NULL)
        OR (user_id IS NULL AND project_id IS NOT NULL)
    )
);

CREATE UNIQUE INDEX IF NOT EXISTS registry_credentials_user_name_idx
    ON registry_credentials (user_id, name)
    WHERE user_id IS NOT NULL;

CREATE UNIQUE INDEX IF NOT EXISTS registry_credentials_project_name_idx
    ON registry_credentials (project_id, name)
    WHERE project_id IS NOT NULL;
