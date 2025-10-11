API Reference

Auth

- All `/v1/*` endpoints require a Bearer token signed with `ADMIN_JWT_SECRET` and claim `{ "role": "admin" }`.
- To test without auth, set `DISABLE_AUTH=true` (development only). Examples below show both modes.

Conventions

- Base URL: `http://localhost:8080` unless noted.
- Variables used in examples:
  - `TOKEN` is a valid JWT; examples omit token generation.
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
  - With auth: `curl -s $AUTH_H http://localhost:8080/v1/clusters`

- GET `/v1/clusters/health` → 200 `[ { "id": "...", "name": "...", "healthy": true|false, "error": "...", "checked_at": "..." }, ... ]`
  - With auth: `curl -s $AUTH_H http://localhost:8080/v1/clusters/health`

- GET `/v1/clusters/{id}/health` → 200 `{ "id": "...", "name": "...", "healthy": true|false, "error": "...", "checked_at": "..." }`
  - With auth: `curl -s $AUTH_H http://localhost:8080/v1/clusters/<id>/health`

Projects

- POST `/v1/projects`
  - Request (either form):
    - With userId: `{ "userId": "<uuid>", "clusterId": "<uuid>", "name": "my-project", "quotaOverrides": {"limits.cpu":"256"} }`
    - With userEmail (auto-create or reuse): `{ "userEmail": "alice@example.com", "userName": "Alice", "clusterId": "<uuid>", "name": "my-project" }`
  - Behavior (v0.1.2): default is per-project namespaces; returns a namespace-scoped kubeconfig for the project in `kubeconfig_b64`.
    - If `PROJECTS_IN_USER_NAMESPACE=true`: applies project defaults (LimitRange) inside a pre-provisioned user namespace; no kubeconfig is returned.
  - Response: `201 { "project": {"id":"...","user_id":"...","cluster_id":"...","name":"...","namespace":"...","created_at":"..."}, "kubeconfig_b64":"..." }`
  - Curl: `curl -s $AUTH_H -H 'Content-Type: application/json' -d '{"userId":"<uuid>","clusterId":"<uuid>","name":"demo"}' http://localhost:8080/v1/projects`

- GET `/v1/projects/{id}` → Status (exists, details)
  - Curl: `curl -s $AUTH_H http://localhost:8080/v1/projects/<id>`

- PATCH `/v1/projects/{id}/quota`
  - Request: `{ "overrides": { "pods": "100", "limits.cpu": "256" } }`
  - Response: `200 { "status": "ok" }`

- POST `/v1/projects/{id}/suspend` and `/v1/projects/{id}/unsuspend`
  - Only for per-project namespace mode. Sets or removes quota blocks.

- DELETE `/v1/projects/{id}`
  - Per-project mode: deletes the project namespace and DB record.
  - Shared user namespace mode: removes project-specific LimitRange.

Examples (Copy + Expected Output)

- GET /healthz
  - Copy: `curl -s http://localhost:8080/healthz`
  - Output: `{"status":"ok"}`

- GET /readyz
  - Copy: `curl -s http://localhost:8080/readyz`
  - Output (ready): `{"status":"ready"}`

- GET /v1/version
  - Copy: `curl -s http://localhost:8080/v1/version`
  - Output: `{"version":"0.1.2","commit":"<git-sha>","date":"<build-date>"}`

- POST /v1/clusters (base64 kubeconfig)
  - Copy: `curl -s $AUTH_H -H 'Content-Type: application/json' -d "$(jq -n --arg n 'my-cluster' --arg b64 \"$B64\" '{name:$n,kubeconfig_b64:$b64}')" http://localhost:8080/v1/clusters`
  - Output: `{"id":"11111111-2222-3333-4444-555555555555","name":"my-cluster","created_at":"2025-01-01T12:00:00Z"}`

- GET /v1/clusters
  - Copy: `curl -s $AUTH_H http://localhost:8080/v1/clusters`
  - Output: `[{"id":"11111111-2222-3333-4444-555555555555","name":"my-cluster","created_at":"2025-01-01T12:00:00Z"}]`

- GET /v1/clusters/health
  - Copy: `curl -s $AUTH_H http://localhost:8080/v1/clusters/health`
  - Output: `[{"id":"11111111-2222-3333-4444-555555555555","name":"my-cluster","healthy":true,"error":"","checked_at":"2025-01-01T12:00:30Z"}]`

- GET /v1/clusters/{id}/health
  - Copy: `curl -s $AUTH_H http://localhost:8080/v1/clusters/11111111-2222-3333-4444-555555555555/health`
  - Output: `{"id":"11111111-2222-3333-4444-555555555555","name":"my-cluster","healthy":true,"error":"","checked_at":"2025-01-01T12:00:30Z"}`

- POST /v1/projects (using userEmail)
  - Copy: `curl -s $AUTH_H -H 'Content-Type: application/json' -d '{"userEmail":"alice@example.com","userName":"Alice","clusterId":"11111111-2222-3333-4444-555555555555","name":"demo"}' http://localhost:8080/v1/projects`
  - Output (per-project mode): `{"project":{"id":"99999999-8888-7777-6666-555555555555","user_id":"aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee","cluster_id":"11111111-2222-3333-4444-555555555555","name":"demo","namespace":"tenant-aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee-demo","created_at":"2025-01-01T12:01:00Z"},"kubeconfig_b64":"..."}`

- GET /v1/projects/{id}
  - Copy: `curl -s $AUTH_H http://localhost:8080/v1/projects/99999999-8888-7777-6666-555555555555`
  - Output: `{"project":{"id":"99999999-8888-7777-6666-555555555555","user_id":"aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee","cluster_id":"11111111-2222-3333-4444-555555555555","name":"demo","namespace":"tenant-aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee-demo","created_at":"2025-01-01T12:01:00Z"},"exists":true,"details":{"resourcequota":true,"limitrange":true,"serviceaccount":true}}`

- PATCH /v1/projects/{id}/quota
  - Copy: `curl -s $AUTH_H -X PATCH -H 'Content-Type: application/json' -d '{"overrides":{"pods":"100"}}' http://localhost:8080/v1/projects/99999999-8888-7777-6666-555555555555/quota`
  - Output: `{"status":"ok"}`

- POST /v1/projects/{id}/suspend
  - Copy: `curl -s $AUTH_H -X POST http://localhost:8080/v1/projects/99999999-8888-7777-6666-555555555555/suspend`
  - Output: `{"status":"suspended"}`

- POST /v1/projects/{id}/unsuspend
  - Copy: `curl -s $AUTH_H -X POST http://localhost:8080/v1/projects/99999999-8888-7777-6666-555555555555/unsuspend`
  - Output: `{"status":"unsuspended"}`

- DELETE /v1/projects/{id}
  - Copy: `curl -s $AUTH_H -X DELETE http://localhost:8080/v1/projects/99999999-8888-7777-6666-555555555555`
  - Output: `{"status":"deleted"}`
