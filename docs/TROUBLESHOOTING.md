# Troubleshooting

Use this guide to diagnose common kubeOP issues. Each entry lists a symptom, likely cause, and fix with commands.

| Symptom | Likely cause | Fix |
| --- | --- | --- |
| `curl /readyz` returns 500 with `database connection failed` | PostgreSQL unavailable or credentials invalid. | Check logs: `docker compose logs postgres` or `kubectl logs deploy/kubeop-api`. Verify `DATABASE_URL`. |
| API responds with 503 `maintenance mode enabled` | Maintenance mode toggled during upgrades. | Disable maintenance: `curl -sS "${AUTH_HEADER[@]}" -H 'Content-Type: application/json' -d '{"enabled":false}' http://localhost:8080/v1/admin/maintenance`. |
| 401 `missing bearer token` | `Authorization` header omitted or token expired. | Export JWT via `export KUBEOP_TOKEN=...` and retry using `AUTH_HEADER`. |
| 403 `forbidden` | JWT does not include `{"role":"admin"}`. | Issue a new token signed with `ADMIN_JWT_SECRET` containing the correct role claim. |
| `/v1/projects/{id}/apps` returns validation warnings about quotas | Requested resources exceed namespace quotas. | Adjust payload (`resources`, `ports`) or increase quotas via `PATCH /v1/projects/{id}/quota`. |
| App stuck in `Pending` | Operator missing permissions or cluster lacks resources. | Inspect operator logs (`kubectl logs deploy/kubeop-operator -n kubeop-system`). Check Kubernetes events (`kubectl describe app <name>`). |
| Operator logs report `no matches for kind "App" in version "kubeop.io/v1alpha1"` | App CRD missing from the managed cluster. | The manager bootstraps the CRD during startup; check for RBAC denials in the operator logs. If controllers cannot create CRDs, apply `kubeop-operator/config/crd/bases/kubeop.io_apps.yaml` manually before restarting the deployment. |
| Operator logs `CustomResourceDefinition.apiextensions.k8s.io "alertpolicies.paas.kubeop.io" is invalid: spec.additionalPrinterColumns[0].JSONPath: Invalid value: "size(.spec.routes)"` | Bundled or pre-existing CRDs include unsupported JSONPath functions. | Upgrade to kubeOP v0.19.4 or later and redeploy; the operator now removes invalid printer columns before applying CRDs. If the message persists, re-run `make -C kubeop-operator crds` and reapply the manifests manually to overwrite stale definitions. |
| `/v1/projects/{id}/apps/{appId}/logs` empty | Logging sidecars or Fluent Bit not configured in the cluster. | Ensure cluster log aggregation is enabled and kubeOP labels (`kubeop.app.id`) are indexed. |
| `/v1/events/ingest` returns 400 `decode json` | Payload not an array of events or exceeds 1 MiB. | Send a JSON array (see [API](API.md#events-and-webhooks)). Keep payloads under 1 MiB. |
| Scheduler logs `dependencies missing` | Service or store dependency nil (misconfiguration during testing). | Ensure `service.New` receives a store and kube manager. Occurs if mocks not initialised in custom builds. |
| `/v1/kubeconfigs/rotate` fails with 404 | Kubeconfig ID not found. | List existing kubeconfigs (`/v1/users/{id}` or `/v1/projects/{id}`) and reuse valid IDs. |

## Diagnostics commands

```bash
# Tail API logs (Docker Compose)
docker compose --file docs/examples/docker-compose.yaml logs -f api

# Tail API logs (Kubernetes)
kubectl -n kubeop-system logs deploy/kubeop-api -f

# Check cluster health snapshots
curl -sS "${AUTH_HEADER[@]}" http://localhost:8080/v1/clusters/health | jq

# Inspect project quota usage
curl -sS "${AUTH_HEADER[@]}" http://localhost:8080/v1/projects/<project-id>/quota | jq
```

## When to escalate

- You discover a security issue → email [security@kubeOP.io](mailto:security@kubeOP.io).
- Data loss or corruption → open an incident with detailed PostgreSQL logs and restore steps attempted.
- Operator reconciliation loops → capture operator logs, relevant Kubernetes events, and app payloads.
