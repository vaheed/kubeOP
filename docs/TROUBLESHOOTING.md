# Troubleshooting

Use this guide to diagnose common kubeOP issues. Each entry includes symptoms, likely causes, and remediation steps.

| Symptom | Likely cause | Resolution |
| --- | --- | --- |
| `curl /healthz` returns 500 | Database unavailable | Ensure PostgreSQL is reachable and credentials match `DATABASE_URL`. Check logs for `failed to connect database`. |
| Mutating APIs return 503 with `maintenance` message | Maintenance mode enabled | Disable maintenance via `POST /v1/admin/maintenance {"enabled":false}` once upgrades complete. |
| `POST /v1/clusters` returns `invalid kubeconfig` | Base64 input malformed or missing | Re-encode the kubeconfig (`base64 -w0`) and confirm the file includes certificate data. |
| `POST /v1/projects/{id}/apps` fails with quota errors | ResourceQuota exceeded | Increase quota variables (`KUBEOP_DEFAULT_*`) or adjust namespace/project quotas in Kubernetes. |
| `GET /v1/projects/{id}/apps/{appId}` shows stale status | Operator unreachable | Check `kubectl get deployment -n kubeop-system kubeop-operator` and inspect controller logs. |
| LoadBalancer service never provisions IP | `MAX_LOADBALANCERS_PER_PROJECT` reached or LB driver misconfigured | Confirm quota values and ensure `LB_DRIVER` and `LB_METALLB_POOL` match your environment. |
| Git delivery fails with `file protocol disabled` | `ALLOW_GIT_FILE_PROTOCOL` unset | Enable only when testing local repositories: `export ALLOW_GIT_FILE_PROTOCOL=true`. |
| `/metrics` empty | Metrics endpoint scraped before readiness | Wait until `/readyz` is healthy; metrics populate after first scheduler tick. |

## Debugging checklist

1. Inspect API logs (`docker compose logs -f api` or journald) for stack traces.
2. Verify environment variables with `printenv | grep KUBEOP_` (mask secrets when sharing output).
3. Confirm filesystem permissions for `${LOGS_ROOT}`; the API needs read/write access.
4. Run `go test -count=1 ./testcase` to reproduce integration regressions locally.
5. For Kubernetes deployments, capture `kubectl describe pod` output to review events and probes.

## Collecting diagnostics

- Capture `/v1/version`, `/v1/projects/{id}/events`, and relevant controller logs before filing issues.
- Redact sensitive values (tokens, kubeconfigs) from support bundles.
- Attach your `.env` (with secrets removed) when reporting configuration problems.
