ALTER TABLE apps
    ADD COLUMN IF NOT EXISTS external_ref TEXT;

CREATE UNIQUE INDEX IF NOT EXISTS idx_apps_external_ref
    ON apps (external_ref)
    WHERE external_ref IS NOT NULL AND deleted_at IS NULL;
