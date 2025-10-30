# Troubleshooting

## Operator deployment not ready

- Check the controller logs via `kubectl -n kubeop-system logs deploy/kubeop-operator`. The controllers set `Ready=False` with reason `Progressing` when Deployments have no available replicas (`AppReconciler` in [`controllers.go`](https://github.com/vaheed/kubeOP/blob/main/internal/operator/controllers/controllers.go#L188-L205)).
- Ensure the namespace exists; the project reconciler creates `kubeop-<tenant>-<project>` namespaces (see [`ProjectReconciler`](https://github.com/vaheed/kubeOP/blob/main/internal/operator/controllers/controllers.go#L78-L128)). Missing namespaces lead to repeated NotFound errors.

## Tenant/Project/App CRDs stuck without status

- Verify the manager API created the backing records. The operator relies on the API-driven workflow but does not enforce referential integrity. Use `/v1/projects` or `/v1/apps` to inspect objects (see [API](./api.md)).
- Inspect RBAC: the ClusterRole in [`deploy/k8s/operator/rbac.yaml`](https://github.com/vaheed/kubeOP/blob/main/deploy/k8s/operator/rbac.yaml#L9-L25) must allow namespace and status updates.

## Manager `/readyz` returns 503

- The handler pings PostgreSQL with a timeout derived from `KUBEOP_DB_TIMEOUT_MS` and requires a non-nil KMS envelope (`handleReady` in [`server.go`](https://github.com/vaheed/kubeOP/blob/main/internal/api/server.go#L202-L211)). Confirm the DSN and base64 keys used in [`internal/config`](https://github.com/vaheed/kubeOP/blob/main/internal/config/config.go#L18-L52).
- When running locally, set `KUBEOP_DEV_INSECURE=true` to auto-generate a KMS key if `KUBEOP_KMS_MASTER_KEY` is unset.

If Docker Compose marks the manager container unhealthy, inspect the `/hc` helper built from [`cmd/healthcheck`](https://github.com/vaheed/kubeOP/blob/main/cmd/healthcheck/main.go#L9-L24); it exits non-zero when `HEALTH_URL` fails.

## `{"error":"db"}` responses from the API

- Every store operation wraps database failures with this error body (e.g., [`tenantsCreate`](https://github.com/vaheed/kubeOP/blob/main/internal/api/server.go#L232-L243)). Inspect PostgreSQL logs or increase log verbosity with `LOG_LEVEL=debug` to capture SQL errors.

## Webhook timeouts or failures

- The webhook client retries three times with exponential backoff and increments `kubeop_webhook_failures_total` on error (`webhook.Client.Send`). Confirm `KUBEOP_HOOK_URL` is reachable and returns 2xx.
- Disable webhooks by leaving `KUBEOP_HOOK_URL` empty.

## Invoice totals look wrong

- The invoice handler multiplies usage lines by `KUBEOP_RATE_CPU_MILLI` and `KUBEOP_RATE_MEM_MIB`, unless tenant overrides exist in the database (`invoice` in [`server.go`](https://github.com/vaheed/kubeOP/blob/main/internal/api/server.go#L402-L417)). Check environment variables and DB overrides to ensure rates are set as expected.
