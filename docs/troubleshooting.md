# Troubleshooting

Common issues and quick resolutions.

| Symptom | Likely cause | Fix |
| --- | --- | --- |
| `401 Unauthorized` on `/v1/*` | Missing or invalid admin JWT (`role` claim not `admin`) | Regenerate token with correct secret and claims. Ensure clock skew < token TTL. |
| `/readyz` returns `not_ready` | PostgreSQL unreachable or migrations pending | Check database connectivity, run `docker compose logs postgres`, and ensure schema migrations succeeded. |
| Project creation fails with `user required` | Neither `userId` nor `userEmail`/`userName` supplied | Provide either an existing ID or both email and name to bootstrap the user. |
| App deployment returns `invalid json` | Payload not valid JSON or missing source field (`image`, `helm`, or `manifests`) | Validate JSON (e.g., with `jq`) and include exactly one source. |
| App stuck pending | ResourceQuota exhausted or insufficient cluster capacity | Inspect `/v1/projects/{id}/quota` usage and Kubernetes events. Increase quotas or scale cluster. |
| `failed to deliver batch` in watcher logs | kubeOP ingest unreachable or TLS failure | Verify `KUBEOP_EVENTS_URL`, certificate trust, and network policies allowing HTTPS to kubeOP. |
| Watcher auto-deploy skipped | `KUBEOP_BASE_URL` unset or auto-deploy disabled via env/config | Set `KUBEOP_BASE_URL=https://...` or `WATCHER_AUTO_DEPLOY=true`. Check startup logs for explanation. |
| `config error: CLUSTER_ID is required (this container runs the watcher agent; use the :latest tag for the API)` when running `docker compose up` | Docker built or reused the watcher image instead of the API stage | Use the bundled `docker-compose.yml` (pins `target: api` + `image: ghcr.io/vaheed/kubeop-api:latest` with `pull_policy: always`) and remove any locally tagged watcher image via `docker image rm ghcr.io/vaheed/kubeop` or `docker image rm ghcr.io/vaheed/kubeop-watcher` before re-running. |
| `/v1/projects/{id}/logs` returns 404 | Log file not created yet (no events) | Trigger activity (deploy app) or check `${LOGS_ROOT}` permissions. |
| Git webhook responds 400 | Signature mismatch | Confirm `GIT_WEBHOOK_SECRET` or per-app `webhookSecret` matches the sender. |
| Kubeconfig rotation returns 404 | Binding ID unknown or cluster mismatch | List bindings via `/v1/kubeconfigs` (ensure created) and pass the correct `clusterId` for user renewals. |

## Debug tips

- Enable debug logs: `LOG_LEVEL=debug`. Sensitive data stays redacted by audit middleware.
- Use `kubectl --kubeconfig <tenant>.kubeconfig auth can-i` to verify RBAC for tenants and projects.
- For watchers, hit `http://<watcher-host>:8081/metrics` to ensure informers synchronised (`kubeop_watcher_ready` gauge equals `1`).
- Tail API logs from `${LOGS_ROOT}`; each request has `request_id` to correlate with audit entries.
