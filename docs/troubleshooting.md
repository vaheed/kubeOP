# Troubleshooting

Common issues and quick resolutions.

| Symptom | Likely cause | Fix |
| --- | --- | --- |
| `401 Unauthorized` on `/v1/*` | Missing or invalid admin JWT (`role` claim not `admin`) | Regenerate token with correct secret and claims. Ensure clock skew < token TTL. |
| `/readyz` returns `not_ready` | PostgreSQL unreachable or migrations pending | Check database connectivity, run `docker compose logs postgres`, and ensure schema migrations succeeded. |
| Project creation fails with `user required` | Neither `userId` nor `userEmail`/`userName` supplied | Provide either an existing ID or both email and name to bootstrap the user. |
| App deployment returns `invalid json` | Payload not valid JSON or missing source field (`image`, `helm`, or `manifests`) | Validate JSON (e.g., with `jq`) and include exactly one source. |
| App stuck pending | ResourceQuota exhausted or insufficient cluster capacity | Inspect `/v1/projects/{id}/quota` usage and Kubernetes events. Increase quotas or scale cluster. |
| `/v1/projects/{id}/logs` returns 404 | Log file not created yet (no events) | Trigger activity (deploy app) or check `${LOGS_ROOT}` permissions. |
| Git webhook responds 400 | Signature mismatch | Confirm `GIT_WEBHOOK_SECRET` or per-app `webhookSecret` matches the sender. |
| Kubeconfig rotation returns 404 | Binding ID unknown or cluster mismatch | List bindings via `/v1/kubeconfigs` (ensure created) and pass the correct `clusterId` for user renewals. |

## Debug tips

- Enable debug logs: `LOG_LEVEL=debug`. Sensitive data stays redacted by audit middleware.
- Use `kubectl --kubeconfig <tenant>.kubeconfig auth can-i` to verify RBAC for tenants and projects.
- Tail API logs from `${LOGS_ROOT}`; each request has `request_id` to correlate with audit entries.
