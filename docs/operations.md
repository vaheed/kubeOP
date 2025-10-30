# Operations

## Logs

- Manager logs use `log/slog` JSON via [`internal/logging.New`](https://github.com/vaheed/kubeOP/blob/main/internal/logging/log.go#L8-L19). Set `LOG_LEVEL=debug` or `LOG_LEVEL=warn` to adjust verbosity; otherwise INFO logs are emitted with a `component` field.
- Operator logs are produced by controller-runtime with the zap logger configured in [`cmd/operator/main.go`](https://github.com/vaheed/kubeOP/blob/main/cmd/operator/main.go#L27-L32). Use the `--leader-elect` flag for HA scenarios so only the elected leader produces reconciliation logs.

## Metrics

Both binaries expose Prometheus metrics:

- Manager: `/metrics` served by [`PromHandler`](https://github.com/vaheed/kubeOP/blob/main/internal/api/metrics.go#L34-L57) includes request counters/histograms (`kubeop_api_requests_total`, `kubeop_api_request_duration_seconds`).
- Operator: `/metrics` served by controller-runtime on `--metrics-bind-address` adds controller metrics and the project/operator-specific metrics recorded via [`internal/metrics`](https://github.com/vaheed/kubeOP/blob/main/internal/metrics/metrics.go#L7-L40):
  - `kubeop_business_created_total{kind="tenant|project|app"}` increments on API-driven creations.
  - `kubeop_webhook_events_total{event, outcome}` and `kubeop_webhook_failures_total` reflect webhook delivery status from [`webhook.Client.Send`](https://github.com/vaheed/kubeOP/blob/main/internal/webhook/webhook.go#L18-L52).
  - `kubeop_db_latency_seconds{op}` tracks database call latency.

Auxiliary services built from [`cmd/admission`](https://github.com/vaheed/kubeOP/blob/main/cmd/admission/main.go#L13-L31), [`cmd/delivery`](https://github.com/vaheed/kubeOP/blob/main/cmd/delivery/main.go#L13-L31), and [`cmd/meter`](https://github.com/vaheed/kubeOP/blob/main/cmd/meter/main.go#L13-L31) reuse `PromHandler`, so their `/metrics` endpoints expose the same counters when deployed.

Consider alerting on sustained increases in `kubeop_webhook_failures_total` or zero available replicas reported by the operator deployment.

## Health probes

- Manager: `/healthz` returns 200 immediately; `/readyz` depends on a successful DB ping and initialized KMS (`handleReady` in [`internal/api/server.go`](https://github.com/vaheed/kubeOP/blob/main/internal/api/server.go#L202-L211)).
- Operator: readiness and liveness are managed by controller-runtime via the probe server bound to `--health-probe-bind-address` (`cmd/operator/main.go`). Default probes respond with `healthz`/`readyz` returning 200.

## Backups and restores

The manager persists state in PostgreSQL via [`internal/db`](https://github.com/vaheed/kubeOP/blob/main/internal/db/db.go#L18-L73). Use PostgreSQL-native tooling (e.g., `pg_dump`, `pg_restore`) against the DSN supplied in `KUBEOP_DB_URL`. Schema migrations are embedded SQL files in [`internal/db/migrations`](https://github.com/vaheed/kubeOP/tree/main/internal/db/migrations); ensure they run (`Server.MustMigrate`) before restoring application traffic.

## Usage aggregation

When `KUBEOP_AGGREGATOR=true`, the manager starts [`usage.Aggregator.RunOnce`](https://github.com/vaheed/kubeOP/blob/main/internal/usage/aggregator.go#L9-L33) every hour, rolling raw usage into `usage_hourly` and cleaning processed rows. Monitor logs for `"db"` errors originating from the aggregator loop.

## Common alerts

- **Operator stalled**: watch the deployment status via `kubectl -n kubeop-system get deploy kubeop-operator`; reconciliation failures will surface as Ready=False conditions in the CRDs documented in [CRDs](./crds.md).
- **Webhook failures**: alert on non-zero `kubeop_webhook_failures_total` or repeated `outcome="failure"` in `kubeop_webhook_events_total`.
- **Database latency**: alert on high percentiles of `kubeop_db_latency_seconds` for operations like `create_tenant`, `invoice_lines`, or `usage_ingest` (see [`metrics.ObserveDB` usage in server handlers](https://github.com/vaheed/kubeOP/blob/main/internal/api/server.go#L232-L563)).
