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

Bind-mount `./logs` for disk-backed project/app logs. Update `.env` with production values before running in shared environments.

### Systemd or bare-metal

1. Build binaries:
   ```bash
   go build -o /usr/local/bin/kubeop-api ./cmd/api
   ```
2. Create a service user with access to configuration and log directories.
3. Provide `/etc/kubeop/kubeop.env` (key=value) and configure a systemd service pointing at `kubeop-api` with `EnvironmentFile=`.

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
Monitoring endpoints:

- `/healthz` (HTTP 200 while the process is running)
- `/readyz` (verifies database connectivity, migration status, and the most recent scheduler tick)
- `/metrics` (Prometheus metrics for request latency, health checks, and background jobs)

## Observability

| Signal | Endpoint / Location | Notes |
| --- | --- | --- |
| Health probes | `GET /healthz`, `GET /readyz` | `readyz` invokes a pluggable health checker (service layer by default) and surfaces dependency errors. |
| Logs | `stdout` + `logs/` | Structured JSON logs for requests/audit and per-project/app logs under `logs/projects/<id>/`. |
| Project events | `GET /v1/projects/{id}/events` | Query PostgreSQL-backed events with filters (`kind`, `severity`, `actor`, `since`, `limit`, `cursor`). |
| Event bridge ingest | `POST /v1/events/ingest` | Enable with `K8S_EVENTS_BRIDGE`; responses include accepted/dropped counts and error indexes per batch. |
| Scheduler status | `GET /v1/clusters/health`, `GET /v1/clusters/{id}/health` | Provides summaries of last cluster check (duration, status, error). |
| Inventory & metadata | `GET /v1/clusters`, `GET /v1/clusters/{id}`, `PATCH /v1/clusters/{id}` | Maintain ownership, environment, region, and tag metadata for auditing and access reviews. |
| Status history | `GET /v1/clusters/{id}/status` | Returns persisted health checks (newest first) with probe stage, API server version, and timestamps. |

## Backups and retention

- **PostgreSQL** – Perform regular logical backups (`pg_dump`) or rely on managed backups. Ensure WAL archiving or PITR is enabled.
- **Logs** – `logs/` can be rsynced or shipped to object storage (e.g. `aws s3 sync logs/ s3://bucket/kubeop`). The directory contains audit logs, per-project logs, and event JSONL files when `EVENTS_DB_ENABLED=false`.

## Disaster recovery

1. Restore PostgreSQL from the latest backup.
2. Redeploy kubeOP with the same `ADMIN_JWT_SECRET` and `KCFG_ENCRYPTION_KEY` to decrypt stored kubeconfigs.

## Maintenance tasks

- **Rotate kubeconfigs** – Use `POST /v1/kubeconfigs/rotate` for user/project bindings. kubeOP re-encrypts secrets and updates stored kubeconfigs.
- **Adjust quotas** – `PATCH /v1/projects/{id}/quota` accepts overrides (`{"overrides":{"limits.cpu":"6"}}`). kubeOP reapplies ResourceQuota and LimitRange objects with annotations to prevent drift.
- **Suspend/resume** – `POST /v1/projects/{id}/suspend` scales deployments to zero and removes Services/Ingresses. `POST /v1/projects/{id}/unsuspend` restores them.
- **Audit webhooks** – Rotate `GIT_WEBHOOK_SECRET` regularly and ensure payload signatures are validated before enqueuing CI jobs.

## Networking requirements

- Expose API via TLS-terminating proxy or ingress. Enforce firewall rules so only trusted networks reach `/v1` endpoints.
