-- add optional cluster reference to tenants
ALTER TABLE tenants ADD COLUMN IF NOT EXISTS cluster_id UUID NULL REFERENCES clusters(id);

