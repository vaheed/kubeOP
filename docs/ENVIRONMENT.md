# Environment variables

This reference highlights the environment settings touched most often when configuring kubeOP and the watcher bridge.

## Core control plane

| Variable | Purpose | Notes |
| --- | --- | --- |
| `KUBEOP_BASE_URL` | External HTTPS URL used by the API, watcher handshake, and event ingest. | Required for watcher auto-deploy. Should be HTTPS unless `ALLOW_INSECURE_HTTP=true`. |
| `ALLOW_INSECURE_HTTP` | Permits `http://` base URLs and allows the watcher handshake plus event sink to connect over HTTP. | Development only. Production deployments must keep this disabled. |
| `DATABASE_URL` | PostgreSQL connection string. | Overrides individual `PG*` vars. |
| `ADMIN_JWT_SECRET` | HMAC key for admin tokens and watcher JWTs. | Keep secret; rotate with watcher tokens if regenerated. |
| `WATCHER_AUTO_DEPLOY` | Controls automatic watcher rollout. | Defaults to `true` when `KUBEOP_BASE_URL` is set. |
| `WATCHER_RUN_AS_USER` / `WATCHER_RUN_AS_GROUP` / `WATCHER_FS_GROUP` | Override the default watcher UID/GID/FSGroup (`65532`). | Leave unset to keep the hardened defaults from the watcher image. |
| `POD_SECURITY_LEVEL` | Pod Security Admission profile applied to namespaces created by kubeOP. | Defaults to `baseline`; set to `restricted` to enforce non-root workloads. |
| `POD_SECURITY_WARN_LEVEL` | Pod Security profile that surfaces warnings. | Match `POD_SECURITY_LEVEL` to suppress warnings while keeping enforcement. |
| `POD_SECURITY_AUDIT_LEVEL` | Pod Security profile recorded by audit backends. | Mirrors the enforcement level by default. |

## Watcher runtime overrides

| Variable | Purpose |
| --- | --- |
| `CLUSTER_ID` | Identifier of the managed cluster. Used when generating watcher events. |
| `KUBEOP_BASE_URL` | Same as the control plane value. The watcher derives `/v1/watchers/handshake` and `/v1/events/ingest` from this base. |
| `KUBEOP_TOKEN` | Bearer token (JWT) used by the watcher. Auto-generated when auto-deploy is enabled. |
| `STORE_PATH` | BoltDB file storing informer resource versions and queued events. |
| `LOGS_ROOT` | Directory for watcher log output. Defaults to `/var/lib/kubeop-watcher/logs`; must be writable by the watcher UID. |
| `BATCH_MAX` / `BATCH_WINDOW_MS` | Tune watcher batching behaviour. |
| `WATCH_NAMESPACE_PREFIXES` | Namespace prefixes that should emit events (comma-separated). Defaults to `user-` so only tenant namespaces are observed. |
| `ALLOW_INSECURE_HTTP` | Optional override to permit HTTP during development. Mirrors the control plane variable so the watcher handshake and sink both accept HTTP targets. |

## Behavioural notes

- The watcher keeps a durable queue of undelivered batches under `STORE_PATH`. When the API becomes reachable again the queue is flushed automatically after a successful handshake.
- `/readyz` now reports readiness only after the state store opens **and** a `/v1/watchers/handshake` succeeds within the last 60 seconds.
- Handshake failures or stale connections return JSON details so probes and dashboards can surface actionable reasons.
