# Operations guide

Use this guide to run kubeOP in production, covering backups, upgrades, high availability, and observability.

## Backups

### PostgreSQL

- Schedule logical dumps with `pg_dump` or physical backups via WAL archiving.
- Retain at least 7 days of backups and test restores regularly.
- Example cronjob:

```bash
pg_dump --format=custom --file=/backups/kubeop-$(date +%F).dump "$DATABASE_URL"
```

### Project logs

- Rotate `${LOGS_ROOT}` using logrotate or object storage sync.
- Ensure backups capture `logs/projects/<project-id>` for audit purposes.

## Upgrades

1. Review the [CHANGELOG](https://github.com/vaheed/kubeOP/blob/main/CHANGELOG.md) for breaking changes.
2. Enable maintenance mode to pause mutating operations:
   ```bash
   curl -s ${KUBEOP_AUTH_HEADER} -H 'Content-Type: application/json' \
     -d '{"enabled":true,"reason":"upgrade"}' \
     http://localhost:8080/v1/admin/maintenance
   ```
3. Back up PostgreSQL and project logs.
4. Deploy the new API image (update Docker Compose or Kubernetes Deployment).
5. Wait for `/readyz` to return HTTP 200 and disable maintenance mode.
6. Confirm `/v1/version` reports the expected build metadata.

## High availability

- Run multiple API replicas behind a load balancer; the application is stateless beyond PostgreSQL and the filesystem logs.
- Use a managed PostgreSQL offering or configure Patroni/HA setups.
- Store logs on shared storage (NFS, object store) to allow any replica to serve downloads.
- The `kubeop-operator` runs within each cluster; use `--leader-elect` via `OPERATOR_LEADER_ELECTION=true` for HA deployments.

## Observability

- **Metrics** – scrape `/metrics` with Prometheus. Key series include `kubeop_http_requests_total` and scheduler gauges.
- **Logs** – send container logs to your aggregation stack. JSON structure includes request IDs and tenant metadata.
- **Tracing** – integrate with OpenTelemetry by wrapping HTTP handlers (contributions welcome).
- **Health checks** – `/healthz` and `/readyz` include dependency status (database connectivity, log directory checks).

## Disaster recovery

- Restore PostgreSQL from the latest valid backup and redeploy the API with the same `ADMIN_JWT_SECRET` and `KCFG_ENCRYPTION_KEY`.
- Restore project logs from object storage if end users require audit trails.
- Re-register clusters if kubeconfigs rotated; `kubeop-operator` will reconcile automatically after API recovery.

## Migrations

- Database migrations run automatically on startup (`store.Migrate`). Monitor logs for `database connected and migrations applied`.
- For large datasets, enable maintenance mode, restart the API to run migrations, and monitor logs before re-enabling writes.

## Troubleshooting

- Review [TROUBLESHOOTING.md](TROUBLESHOOTING.md) for symptom-based diagnostics.
- Check `/v1/projects/{id}/events` for namespace-level issues recorded by kubeOP.
- Use `kubectl logs -n kubeop-system deployment/kubeop-operator` to inspect the controller when reconciles fail.
