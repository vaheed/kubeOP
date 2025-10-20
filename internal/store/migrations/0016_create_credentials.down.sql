DROP INDEX IF EXISTS registry_credentials_project_name_idx;
DROP INDEX IF EXISTS registry_credentials_user_name_idx;
DROP TABLE IF EXISTS registry_credentials;

DROP INDEX IF EXISTS git_credentials_project_name_idx;
DROP INDEX IF EXISTS git_credentials_user_name_idx;
DROP TABLE IF EXISTS git_credentials;
