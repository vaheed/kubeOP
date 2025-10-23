# Operations guide

This guide explains how to run kubeOP day-to-day. It covers backups, upgrades, maintenance windows, high availability, and
observability.

## Daily checklist

- Monitor `/healthz`, `/readyz`, and `/metrics` for regression signals.
- Track the cluster health scheduler logs (`cluster health tick complete`) for changes in healthy/unhealthy counts.
- Review the `kubeop-*` labels on Kubernetes workloads to ensure deterministic labelling across clusters.
- Verify maintenance mode is disabled (`GET /v1/admin/maintenance`).

## Backups

| Asset | Why it matters | How to back it up |
| --- | --- | --- |
| PostgreSQL database | Stores clusters, users, projects, releases, events, and maintenance state. | Use native PostgreSQL backups (`pg_dump`, `pg_basebackup`) or cloud snapshots. Run before upgrades. |
| Operator manifests | Ensure the `kubeop-operator` deployment spec matches your desired version. | Store declarative manifests (GitOps, Helm) and keep version history. |
| Environment configuration | Recreate secrets and runtime settings. | Check `.env`, Kubernetes Secrets, and external secret managers. Rotate secrets regularly. |

## Upgrades

1. Enable maintenance mode to block mutating requests:
   ```bash
   curl -sS "${AUTH_HEADER[@]}" \
     -H 'Content-Type: application/json' \
     -d '{"enabled":true,"message":"Upgrading to 0.14.1"}' \
     http://localhost:8080/v1/admin/maintenance
   ```
2. Build or pull the new kubeOP API image/binary.
3. Apply database schema migrations implicitly by restarting the API (migrations run on startup). Watch logs for
   `database connected and migrations applied`.
4. Roll the `kubeop-operator` in each managed cluster to the matching version.
5. Run smoke tests (health checks, list clusters/projects, validate an app).
6. Disable maintenance mode:
   ```bash
   curl -sS "${AUTH_HEADER[@]}" -H 'Content-Type: application/json' \
     -d '{"enabled":false}' http://localhost:8080/v1/admin/maintenance
   ```
7. Update the [CHANGELOG](https://github.com/vaheed/kubeOP/blob/main/CHANGELOG.md) if your organisation tracks internal releases.

## High availability

- **API replicas** – Run multiple kubeOP API replicas behind a load balancer. All nodes connect to the same PostgreSQL instance.
- **PostgreSQL** – Use managed PostgreSQL or set up streaming replication with automated failover.
- **Managed clusters** – Ensure each cluster runs the `kubeop-operator` with leader election (`OPERATOR_LEADER_ELECTION=true`) when
  you deploy more than one operator replica.
- **Kubernetes clients** – kubeOP caches controller-runtime clients per cluster. Rotate kubeconfigs via `/v1/kubeconfigs/rotate`
  when credentials change.

## Maintenance mode

- `PUT /v1/admin/maintenance` toggles maintenance state and records the actor and timestamp.
- While enabled, mutating endpoints (clusters, projects, apps, credentials) return HTTP 503 with your message.
- Read operations (lists, status, logs) continue working for observability.
- Use maintenance mode during upgrades, database maintenance, or operator rollouts.

## Observability

| Signal | Where to read it | Notes |
| --- | --- | --- |
| Metrics | `GET /metrics` | Prometheus format (HTTP latency, scheduler durations, request counters). |
| Logs | `kubectl logs` or Docker logs | Structured JSON with request IDs, actor metadata, and scheduler events. |
| Cluster health | Logs containing `cluster health tick complete` and `/v1/clusters/{id}/health`. | Alert on rising `unhealthy` counts. |
| Project/app logs | `/v1/projects/{id}/logs`, `/v1/projects/{id}/apps/{appId}/logs` | Streams logs directly from managed clusters. |
| Events | `/v1/projects/{id}/events` and `/v1/events/ingest` | Store and ingest tenant events for auditing. |

## Disaster recovery

1. Restore PostgreSQL from the latest backup.
2. Redeploy the kubeOP API pointing at the restored database.
3. Reinstall `kubeop-operator` into each managed cluster if deleted.
4. Rotate `ADMIN_JWT_SECRET` and `KCFG_ENCRYPTION_KEY` if compromise is suspected; update secrets in configuration.
5. Verify `/v1/version` and list clusters/projects to confirm recovery.

## Incident response tips

- Enable verbose logging by setting `LOG_LEVEL=debug` temporarily.
- Use `/v1/projects/{id}/apps/{appId}/delivery` to confirm manifest digests and SBOM metadata when investigating drift.
- Use `/v1/projects/{id}/apps/{appId}/releases` to audit recent rollouts.
- Ingest Kubernetes events via `/v1/events/ingest` to correlate operator warnings with control-plane logs.

## Automation hooks

- `docker-compose.yaml` and Kubernetes manifests expose well-known environment variables—check [ENVIRONMENT](ENVIRONMENT.md) before
  scripting.
- The `docs/examples/curl/register-cluster.sh` script demonstrates automation-friendly logging and error handling; adapt it for CI
  pipelines when registering clusters or rotating credentials.
- For GitOps flows, use `/v1/templates/*` to render manifests and persist the rendered app definition.
