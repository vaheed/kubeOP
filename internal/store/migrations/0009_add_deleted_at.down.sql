-- Remove soft-delete columns (destructive)
ALTER TABLE IF EXISTS apps DROP COLUMN IF EXISTS deleted_at;
ALTER TABLE IF EXISTS projects DROP COLUMN IF EXISTS deleted_at;
ALTER TABLE IF EXISTS users DROP COLUMN IF EXISTS deleted_at;

