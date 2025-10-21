# API overview

All admin APIs live under `/v1` and require an HS256 JWT signed with `ADMIN_JWT_SECRET` and the claim `{"role":"admin"}`. Health, readiness, metrics, and version endpoints are unauthenticated.

- Base URL: `http://<host>:<port>` (default `http://localhost:8080`).
- Content type: `application/json` for requests and responses unless noted.
- Errors return `{ "error": "message" }` with an appropriate HTTP status.

## Authentication

Tokens must contain `role=admin`. kubeOP records `sub`, `user_id`, or `email` claims in audit logs when present.

Example token generation (requires `pip install pyjwt`):

```bash
python - <<'PY'
import jwt, time
secret = 'changeme'
now = int(time.time())
print(jwt.encode({'role': 'admin', 'sub': 'ops@example.com', 'iat': now, 'exp': now + 3600}, secret, algorithm='HS256'))
PY
```

Set the header on every request:

```bash
curl -H "Authorization: Bearer $TOKEN" http://localhost:8080/v1/projects
```

## Endpoint map

| Group | Endpoints |
| --- | --- |
| Health | `GET /healthz`, `GET /readyz`, `GET /metrics`, `GET /v1/version` |
| Clusters | `POST /v1/clusters`, `GET /v1/clusters`, `GET /v1/clusters/{id}`, `PATCH /v1/clusters/{id}`, `GET /v1/clusters/{id}/status`, `GET /v1/clusters/health`, `GET /v1/clusters/{id}/health` |
| Users | `POST /v1/users/bootstrap`, `GET /v1/users`, `GET /v1/users/{id}`, `DELETE /v1/users/{id}`, `POST /v1/users/{id}/kubeconfig/renew`, `GET /v1/users/{id}/projects` |
| Projects | `POST /v1/projects`, `GET /v1/projects`, `GET /v1/projects/{id}`, `DELETE /v1/projects/{id}`, `POST /v1/projects/{id}/suspend`, `POST /v1/projects/{id}/unsuspend`, `GET /v1/projects/{id}/quota`, `PATCH /v1/projects/{id}/quota`, `GET /v1/projects/{id}/logs`, `GET /v1/projects/{id}/events`, `POST /v1/projects/{id}/events` |
| Event bridge | `POST /v1/events/ingest` |
| Apps | `POST /v1/projects/{id}/apps`, `GET /v1/projects/{id}/apps`, `GET /v1/projects/{id}/apps/{appId}`, `DELETE /v1/projects/{id}/apps/{appId}`, `PATCH /v1/projects/{id}/apps/{appId}/scale`, `PATCH /v1/projects/{id}/apps/{appId}/image`, `POST /v1/projects/{id}/apps/{appId}/rollout/restart`, `GET /v1/projects/{id}/apps/{appId}/logs`, `POST /v1/projects/{id}/apps/{appId}/configs/attach`, `POST /v1/projects/{id}/apps/{appId}/configs/detach`, `POST /v1/projects/{id}/apps/{appId}/secrets/attach`, `POST /v1/projects/{id}/apps/{appId}/secrets/detach` |
| Configs & secrets | `POST /v1/projects/{id}/configs`, `GET /v1/projects/{id}/configs`, `DELETE /v1/projects/{id}/configs/{name}`, `POST /v1/projects/{id}/secrets`, `GET /v1/projects/{id}/secrets`, `DELETE /v1/projects/{id}/secrets/{name}` |
| Kubeconfigs | `POST /v1/kubeconfigs`, `POST /v1/kubeconfigs/rotate`, `DELETE /v1/kubeconfigs/{id}` |
| Templates | `POST /v1/templates`, `GET /v1/templates`, `GET /v1/templates/{id}`, `POST /v1/templates/{id}/render`, `POST /v1/projects/{id}/templates/{templateId}/deploy` |
| Webhooks | `POST /v1/webhooks/git` |

Use the linked pages to find payload schemas, curl examples, and error codes for each group.
