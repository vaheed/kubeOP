# Environment variables

This reference highlights the environment settings touched most often when configuring kubeOP and the watcher bridge.

## Core control plane

| Variable | Purpose | Notes |
| --- | --- | --- |
| `KUBEOP_BASE_URL` | External HTTPS URL used by the API, watcher handshake, and event ingest. | Required for watcher auto-deploy. Should be HTTPS unless `ALLOW_INSECURE_HTTP=true`. |
| `ALLOW_INSECURE_HTTP` | Permits `http://` base URLs. | Development only. Production deployments must keep this disabled. |
| `DATABASE_URL` | PostgreSQL connection string. | Overrides individual `PG*` vars. |
| `ADMIN_JWT_SECRET` | HMAC key for admin tokens and watcher JWTs. | Keep secret; rotate with watcher tokens if regenerated. |
| `WATCHER_AUTO_DEPLOY` | Controls automatic watcher rollout. | Defaults to `true` when `KUBEOP_BASE_URL` is set. |
| `WATCHER_RUN_AS_USER` / `WATCHER_RUN_AS_GROUP` / `WATCHER_FS_GROUP` | Override the default watcher UID/GID/FSGroup (`65532`). | Leave unset to keep the hardened defaults from the watcher image. |

## Watcher runtime overrides

| Variable | Purpose |
| --- | --- |
| `CLUSTER_ID` | Identifier of the managed cluster. Used when generating watcher events. |
| `KUBEOP_BASE_URL` | Same as the control plane value. The watcher derives `/v1/watchers/handshake` and `/v1/events/ingest` from this base. |
| `KUBEOP_TOKEN` | Bearer token (JWT) used by the watcher. Auto-generated when auto-deploy is enabled. |
| `STORE_PATH` | BoltDB file storing informer resource versions and queued events. |
| `BATCH_MAX` / `BATCH_WINDOW_MS` | Tune watcher batching behaviour. |
| `ALLOW_INSECURE_HTTP` | Optional override to permit HTTP during development. Mirrors the control plane variable. |

## Behavioural notes

- The watcher keeps a durable queue of undelivered batches under `STORE_PATH`. When the API becomes reachable again the queue is flushed automatically after a successful handshake.
- `/readyz` now reports readiness only after the state store opens **and** a `/v1/watchers/handshake` succeeds within the last 60 seconds.
- Handshake failures or stale connections return JSON details so probes and dashboards can surface actionable reasons.
