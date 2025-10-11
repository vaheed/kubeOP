API Reference

Auth

- All `/v1/*` endpoints require a Bearer token signed with `ADMIN_JWT_SECRET` and claim `{ "role": "admin" }`.
- To test without auth, set `DISABLE_AUTH=true` (development only). Examples below show both modes.

Conventions

- Base URL: `http://localhost:8080` unless noted.
- Variables used in examples:
  - `TOKEN="$(printf '{"role":"admin"}' | jq -r '@base64' >/dev/null 2>&1; echo 'generate JWT separately')"` (replace with a real JWT)
  - `AUTH_H="-H 'Authorization: Bearer $TOKEN'"`

Health

- GET `/healthz` → 200 `{ "status": "ok" }`
  - curl: `curl -s http://localhost:8080/healthz`

- GET `/readyz` → 200 `{ "status": "ready" }` if DB reachable, else 503 `{ "status": "not_ready" }`
  - curl: `curl -s http://localhost:8080/readyz`

Version

- GET `/v1/version` → 200 `{ "version": "...", "commit": "...", "date": "..." }`
  - Without auth (DISABLE_AUTH=true): `curl -s http://localhost:8080/v1/version`
  - With auth: `curl -s $AUTH_H http://localhost:8080/v1/version`

Clusters

- POST `/v1/clusters`
  - Fields:
    - `name` (string)
    - `kubeconfig_b64` (string, required): Base64-encoded kubeconfig file contents
  - Response: `201 { "id": "uuid", "name": "my-cluster", "created_at": "RFC3339" }`
  - Notes: kubeconfig is encrypted and stored at rest. Not returned in responses.
  - Create base64 and upload:
    - Linux/macOS: `B64=$(base64 -w0 < kubeconfig)`
    - Windows (PowerShell): `$B64 = [Convert]::ToBase64String([IO.File]::ReadAllBytes('kubeconfig'))`
    - `curl -s $AUTH_H -H 'Content-Type: application/json' -d "$(jq -n --arg n 'my-cluster' --arg b64 "$B64" '{name:$n,kubeconfig_b64:$b64}')" http://localhost:8080/v1/clusters`

- GET `/v1/clusters` → 200 `[ { "id": "uuid", "name": "...", "created_at": "..." }, ... ]`
  - Without auth: `curl -s http://localhost:8080/v1/clusters` (only if `DISABLE_AUTH=true`)
  - With auth: `curl -s $AUTH_H http://localhost:8080/v1/clusters`

- GET `/v1/clusters/health` → 200 `[ { "id": "...", "name": "...", "healthy": true|false, "error": "...", "checked_at": "..." }, ... ]`
  - With auth: `curl -s $AUTH_H http://localhost:8080/v1/clusters/health`

- GET `/v1/clusters/{id}/health` → 200 `{ "id": "...", "name": "...", "healthy": true|false, "error": "...", "checked_at": "..." }`
  - With auth: `curl -s $AUTH_H http://localhost:8080/v1/clusters/<id>/health`

Users

Projects

- POST `/v1/projects`
  - Request: `{ "userId": "<uuid>", "clusterId": "<uuid>", "name": "my-project", "quotaOverrides": {"limits.cpu":"256"} }`
  - Response: `201 { "project": {"id":"...","user_id":"...","cluster_id":"...","name":"...","namespace":"...","created_at":"..."}, "kubeconfig_b64":"..." }`
  - Curl: `curl -s $AUTH_H -H 'Content-Type: application/json' -d '{"userId":"<uuid>","clusterId":"<uuid>","name":"demo"}' http://localhost:8080/v1/projects`

- GET `/v1/projects/{id}` → Status (exists, details)
  - Curl: `curl -s $AUTH_H http://localhost:8080/v1/projects/<id>`

- PATCH `/v1/projects/{id}/quota`
  - Request: `{ "overrides": { "limits.memory": "128Gi", "pods": "100" } }`
  - Curl: `curl -s $AUTH_H -X PATCH -H 'Content-Type: application/json' -d '{"overrides":{"pods":"100"}}' http://localhost:8080/v1/projects/<id>/quota`

- POST `/v1/projects/{id}/suspend`
  - Curl: `curl -s $AUTH_H -X POST http://localhost:8080/v1/projects/<id>/suspend`

- POST `/v1/projects/{id}/unsuspend`
  - Curl: `curl -s $AUTH_H -X POST http://localhost:8080/v1/projects/<id>/unsuspend`

- DELETE `/v1/projects/{id}`
  - Curl: `curl -s $AUTH_H -X DELETE http://localhost:8080/v1/projects/<id>`

- POST `/v1/users`
  - Request: `{ "name": "Alice", "email": "alice@example.com" }`
  - Response: `201 { "id": "uuid", "name": "Alice", "email": "alice@example.com", "created_at": "..." }`
  - Without auth: `curl -s -H 'Content-Type: application/json' -d '{"name":"Alice","email":"alice@example.com"}' http://localhost:8080/v1/users`
  - With auth: `curl -s $AUTH_H -H 'Content-Type: application/json' -d '{"name":"Alice","email":"alice@example.com"}' http://localhost:8080/v1/users`

- GET `/v1/users` → 200 `[ { "id": "uuid", "name": "...", "email": "...", "created_at": "..." }, ... ]`
  - Without auth: `curl -s http://localhost:8080/v1/users`
  - With auth: `curl -s $AUTH_H http://localhost:8080/v1/users`

- GET `/v1/users/{id}` → 200 `{ "id": "uuid", "name": "...", "email": "...", "created_at": "..." }` or 404
  - With auth: `curl -s $AUTH_H http://localhost:8080/v1/users/<id>`

Error Format

- Errors return JSON `{ "error": "message" }` with the appropriate HTTP status.
