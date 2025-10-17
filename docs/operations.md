# Operations runbook

This runbook covers day-2 tasks for operating kubeOP in production: deployments, scaling, observability, and recovery.

## Deploying the control plane

### Docker Compose (reference implementation)

The repository ships `docker-compose.yml` for local or single-host setups.

```bash
docker compose up -d --build
```

Services:

- `api`: runs `go run ./cmd/api`, exposes `:8080`.
- `postgres`: stores application data (`kubeop` database).
- `watcher`: optional local watcher for test clusters.

Bind-mount `./logs` for disk-backed project/app logs. Update `.env` with production values before running in shared environments.

### Systemd or bare-metal

1. Build binaries:
   ```bash
   go build -o /usr/local/bin/kubeop-api ./cmd/api
   go build -o /usr/local/bin/kubeop-watcher ./cmd/kubeop-watcher
   ```
2. Create a service user with access to configuration and log directories.
3. Provide `/etc/kubeop/kubeop.env` (key=value) and configure a systemd service pointing at `kubeop-api` with `EnvironmentFile=`.
4. Point `DATABASE_URL` at a managed PostgreSQL instance and `KUBEOP_BASE_URL` at the external ingress URL (HTTPS required for watcher auto-deploy).

### Kubernetes deployment

- Run the control plane outside managed clusters (e.g. management cluster or bare-metal host).
- Use a Deployment or StatefulSet for `kubeop-api` with `hostNetwork` disabled and ingress terminating TLS.
- Mount persistent storage for `logs/` or redirect logs to object storage/SIEM.
- Expose `/metrics`, `/healthz`, and `/readyz` via a ServiceMonitor or ingress for probes.

## Scaling considerations

- kubeOP is stateless aside from logs; horizontal scaling is achieved by running multiple API replicas behind a load balancer.
- Use PostgreSQL HA (managed service or Patroni) with backups and point-in-time recovery.
- For high churn workloads increase `DATABASE_URL` pool settings using connection string parameters (e.g. `?pool_max_conns=20`).
- Scheduler cadence is controlled by `CLUSTER_HEALTH_INTERVAL_SECONDS`. Keep it ≥30s to avoid overwhelming clusters.

## Watcher fleet management

- Auto-deploy triggers when `KUBEOP_BASE_URL` is set and `WATCHER_AUTO_DEPLOY` is not explicitly false. kubeOP mints per-cluster JWTs via `service.GenerateWatcherToken`, validates `/v1/watchers/handshake`, and now schedules the watcher rollout asynchronously. When `WATCHER_WAIT_FOR_READY=true`, readiness checks still run, but they no longer block the cluster registration API response and failures are confined to logs so registrations succeed consistently.
- Watcher handshakes now accept the cluster identifier from the request body (`{"cluster_id": "<id>"}`) when the token was minted before the claim existed. The body value must match any claim in the JWT, preventing mismatched configurations while letting existing deployments reconnect without rotating secrets first.
- Watcher instances now probe each configured kind on startup and log a warning when a Kubernetes API group/version/resource is missing (for example when cert-manager is not installed). Missing kinds are skipped so readiness remains green for the remaining resources instead of looping in `CrashLoopBackOff`.
- For air-gapped or restricted clusters, disable auto-deploy and run `kubeop-watcher` manually using a kubeconfig with cluster-admin privileges. Persist the SQLite state file (`STORE_PATH`) on durable storage so list/watch resumes without replaying entire cluster histories.
- Watcher pods expose:
  - `/healthz` (HTTP 200 when process alive)
  - `/readyz` (ensures state store open, informers synced, the last handshake succeeded within 60s, and queued batches flushed successfully)
  - `/metrics` (Prometheus metrics for queue depth, drops, retries, heartbeats)

When `K8S_EVENTS_BRIDGE=true`, the `/v1/events/ingest` endpoint persists watcher batches and returns a JSON summary (`accepted`, `dropped`). Leave watchers deployed even when the bridge is disabled—queued events remain on disk and flush automatically after the next successful handshake once ingestion is re-enabled. Expect `{"reason":"delivery"}` responses from `/readyz` while the watcher waits to replay stored batches.

## Observability

| Signal | Endpoint / Location | Notes |
| --- | --- | --- |
| Health probes | `GET /healthz`, `GET /readyz` | `readyz` invokes a pluggable health checker (service layer by default) and surfaces dependency errors. |
| Metrics | `GET /metrics` | Prometheus-format metrics covering HTTP latency, watcher sink stats, scheduler ticks, and quota reconciliations. |
| Logs | `stdout` + `logs/` | Structured JSON logs for requests/audit and per-project/app logs under `logs/projects/<id>/`. |
| Project events | `GET /v1/projects/{id}/events` | Query PostgreSQL-backed events with filters (`kind`, `severity`, `actor`, `since`, `limit`, `cursor`). |
| Scheduler status | `GET /v1/clusters/health`, `GET /v1/clusters/{id}/health` | Provides summaries of last cluster check (duration, status, error). |

## Backups and retention

- **PostgreSQL** – Perform regular logical backups (`pg_dump`) or rely on managed backups. Ensure WAL archiving or PITR is enabled.
- **Logs** – `logs/` can be rsynced or shipped to object storage (e.g. `aws s3 sync logs/ s3://bucket/kubeop`). The directory contains audit logs, per-project logs, and event JSONL files when `EVENTS_DB_ENABLED=false`.
- **Watcher state** – If running the watcher manually, back up the SQLite file at `STORE_PATH` to preserve informer resource versions across restarts.

## Disaster recovery

1. Restore PostgreSQL from the latest backup.
2. Redeploy kubeOP with the same `ADMIN_JWT_SECRET` and `KCFG_ENCRYPTION_KEY` to decrypt stored kubeconfigs.
3. Replay `.env` and secrets, ensuring `KUBEOP_BASE_URL` and watcher settings match previous values.
4. If watchers were manually deployed, reapply Kubernetes manifests and restore their state PVCs or SQLite files.
5. Verify `/readyz`, check `/metrics` for watcher queue depth, and tail project logs to confirm reconciliation.

## Maintenance tasks

- **Rotate kubeconfigs** – Use `POST /v1/kubeconfigs/rotate` for user/project bindings. kubeOP re-encrypts secrets and updates stored kubeconfigs.
- **Adjust quotas** – `PATCH /v1/projects/{id}/quota` accepts overrides (`{"overrides":{"limits.cpu":"6"}}`). kubeOP reapplies ResourceQuota and LimitRange objects with annotations to prevent drift.
- **Suspend/resume** – `POST /v1/projects/{id}/suspend` scales deployments to zero and removes Services/Ingresses. `POST /v1/projects/{id}/unsuspend` restores them.
- **Audit webhooks** – Rotate `GIT_WEBHOOK_SECRET` regularly and ensure payload signatures are validated before enqueuing CI jobs.

## Networking requirements

- Control plane must reach: PostgreSQL (`tcp/5432`), target cluster APIs (typically `tcp/6443`), and watchers over HTTPS (`tcp/443`).
- Watchers require egress to kubeOP’s `KUBEOP_BASE_URL` on HTTPS and Kubernetes API access inside the managed cluster.
- Expose API via TLS-terminating proxy or ingress. Enforce firewall rules so only trusted networks reach `/v1` endpoints.
