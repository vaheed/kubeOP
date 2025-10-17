DROP INDEX IF EXISTS idx_apps_external_ref;
ALTER TABLE apps DROP COLUMN IF EXISTS external_ref;
