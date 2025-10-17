DROP INDEX IF EXISTS k8s_objects_last_seen_idx;
DROP INDEX IF EXISTS k8s_objects_uid_idx;
DROP INDEX IF EXISTS k8s_objects_identity_idx;
DROP TABLE IF EXISTS k8s_objects;
DROP INDEX IF EXISTS watchers_cluster_id_key;
DROP TABLE IF EXISTS watchers;
