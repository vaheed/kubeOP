-- tenants
CREATE TABLE IF NOT EXISTS tenants (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name TEXT NOT NULL UNIQUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- projects
CREATE TABLE IF NOT EXISTS projects (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (tenant_id, name)
);

-- apps
CREATE TABLE IF NOT EXISTS apps (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    image TEXT,
    host TEXT,
    secret_encrypted BYTEA,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (project_id, name)
);

-- usage (hourly)
CREATE TABLE IF NOT EXISTS usage_hourly (
    ts TIMESTAMPTZ NOT NULL,
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    cpu_milli BIGINT NOT NULL DEFAULT 0,
    mem_mib  BIGINT NOT NULL DEFAULT 0,
    PRIMARY KEY (ts, tenant_id)
);

-- tenant rates
CREATE TABLE IF NOT EXISTS tenant_rates (
    tenant_id UUID PRIMARY KEY REFERENCES tenants(id) ON DELETE CASCADE,
    cpu_milli_rate DOUBLE PRECISION NOT NULL DEFAULT 0,
    mem_mib_rate DOUBLE PRECISION NOT NULL DEFAULT 0,
    tier TEXT NOT NULL DEFAULT 'standard',
    effective_from TIMESTAMPTZ NOT NULL DEFAULT now(),
    discount DOUBLE PRECISION NOT NULL DEFAULT 0
);
