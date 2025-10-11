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

- POST `/v1/users`
  - Request: `{ "name": "Alice", "email": "alice@example.com" }`
  - Response: `201 { "id": "uuid", "name": "Alice", "email": "alice@example.com", "created_at": "..." }`
  - Without auth: `curl -s -H 'Content-Type: application/json' -d '{"name":"Alice","email":"alice@example.com"}' http://localhost:8080/v1/users`
  - With auth: `curl -s $AUTH_H -H 'Content-Type: application/json' -d '{"name":"Alice","email":"alice@example.com"}' http://localhost:8080/v1/users`

- POST `/v1/users/bootstrap`
  - Request (recommended): `{ "userId": "<uuid>", "clusterId": "<uuid>" }`
  - Alternate (create or reuse by email): `{ "name": "Alice", "email": "alice@example.com", "clusterId": "<uuid>" }`
  - Effect: creates namespace `user-<userId>` on the target cluster with quotas/limits/PSA, creates ServiceAccount and role/binding, mints token, returns base64 kubeconfig for that namespace.
  - Response: `201 { "user": { ... }, "namespace": "user-...", "kubeconfig_b64": "..." }`
  - Curl (existing user): `curl -s $AUTH_H -H 'Content-Type: application/json' -d '{"userId":"<uuid>","clusterId":"<uuid>"}' http://localhost:8080/v1/users/bootstrap`
  - Curl (create by email): `curl -s $AUTH_H -H 'Content-Type: application/json' -d '{"name":"Alice","email":"alice@example.com","clusterId":"<uuid>"}' http://localhost:8080/v1/users/bootstrap`

- GET `/v1/users` → 200 `[ { "id": "uuid", "name": "...", "email": "...", "created_at": "..." }, ... ]`
  - Without auth: `curl -s http://localhost:8080/v1/users`
  - With auth: `curl -s $AUTH_H http://localhost:8080/v1/users`

- GET `/v1/users/{id}` → 200 `{ "id": "uuid", "name": "...", "email": "...", "created_at": "..." }` or 404
  - With auth: `curl -s $AUTH_H http://localhost:8080/v1/users/<id>`

Projects

- POST `/v1/projects`
  - Request: `{ "userId": "<uuid>", "clusterId": "<uuid>", "name": "my-project", "quotaOverrides": {"limits.cpu":"256"} }`
  - Behavior:
    - If `PROJECTS_IN_USER_NAMESPACE=true` (default): applies project defaults (LimitRange) inside the user's namespace. No kubeconfig is returned (use the user kubeconfig from bootstrap).
    - If false: creates a dedicated namespace per project (legacy mode) and returns a namespace-scoped kubeconfig for that project.
  - Response: `201 { "project": {"id":"...","user_id":"...","cluster_id":"...","name":"...","namespace":"...","created_at":"..."}, "kubeconfig_b64":"" }`
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

## Legacy placement (appendix)

Error Format

- Errors return JSON `{ "error": "message" }` with the appropriate HTTP status.

Roadmaps (Step-by-Step Scenarios)

- Bootstrap a user for a cluster
  - 1) Register cluster with base64 kubeconfig (`POST /v1/clusters`).
  - 2) Create user (`POST /v1/users`) and take the returned `id`.
  - 3) Bootstrap user (`POST /v1/users/bootstrap`) with `userId` and `clusterId` to create `user-<userId>` namespace, SA/RBAC, quotas/limits, and receive a base64 kubeconfig scoped to that namespace.
  - 4) Use kubeconfig to access only that namespace with kubectl.

- Create a project (shared namespace mode, default)
  - Requires `PROJECTS_IN_USER_NAMESPACE=true`.
  - `POST /v1/projects` applies a per-project `LimitRange` inside the user namespace. No kubeconfig is returned; use the user’s kubeconfig.

- Create a project (legacy per-project namespace)
  - Set `PROJECTS_IN_USER_NAMESPACE=false`.
  - `POST /v1/projects` creates a namespace per project with quotas/limits/policies and returns a base64 kubeconfig for that project.

- Adjust quotas
  - Shared mode: adjust user namespace `ResourceQuota` (admin task). Project-level hard quotas are not supported inside one namespace by Kubernetes.
  - Legacy mode: `PATCH /v1/projects/{id}/quota` to apply overrides.

- Suspend/Unsuspend
  - Shared mode: suspend at namespace-level (admin task).
  - Legacy mode: `POST /v1/projects/{id}/suspend|unsuspend`.

- Delete
  - Shared mode: `DELETE /v1/projects/{id}` removes project-specific LimitRange.
  - Legacy mode: `DELETE /v1/projects/{id}` deletes the project namespace and DB record.

Examples (Copy + Expected Output)

- GET /healthz
  - Copy: `curl -s http://localhost:8080/healthz`
  - Output: `{"status":"ok"}`

- GET /readyz
  - Copy: `curl -s http://localhost:8080/readyz`
  - Output (ready): `{"status":"ready"}`

- GET /v1/version
  - Copy: `curl -s http://localhost:8080/v1/version`
  - Output: `{"version":"dev","commit":"<git-sha>","date":"<build-date>"}`

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

- POST /v1/users
  - Copy: `curl -s $AUTH_H -H 'Content-Type: application/json' -d '{"name":"Alice","email":"alice@example.com"}' http://localhost:8080/v1/users`
  - Output: `{"id":"aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee","name":"Alice","email":"alice@example.com","created_at":"2025-01-01T12:00:00Z"}`

- POST /v1/users/bootstrap (recommended: with userId)
  - Copy: `curl -s $AUTH_H -H 'Content-Type: application/json' -d '{"userId":"aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee","clusterId":"11111111-2222-3333-4444-555555555555"}' http://localhost:8080/v1/users/bootstrap`
  - Output: `{"user":{"id":"aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee","name":"Alice","email":"alice@example.com","created_at":"2025-01-01T12:00:00Z"},"namespace":"user-aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee","kubeconfig_b64":"..."}`

- GET /v1/users
  - Copy: `curl -s $AUTH_H http://localhost:8080/v1/users`
  - Output: `[{"id":"aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee","name":"Alice","email":"alice@example.com","created_at":"2025-01-01T12:00:00Z"}]`

- GET /v1/users/{id}
  - Copy: `curl -s $AUTH_H http://localhost:8080/v1/users/aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee`
  - Output: `{"id":"aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee","name":"Alice","email":"alice@example.com","created_at":"2025-01-01T12:00:00Z"}`

- POST /v1/projects
  - Copy: `curl -s $AUTH_H -H 'Content-Type: application/json' -d '{"userId":"aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee","clusterId":"11111111-2222-3333-4444-555555555555","name":"demo"}' http://localhost:8080/v1/projects`
  - Output (shared namespace mode): `{"project":{"id":"99999999-8888-7777-6666-555555555555","user_id":"aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee","cluster_id":"11111111-2222-3333-4444-555555555555","name":"demo","namespace":"user-aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee","created_at":"2025-01-01T12:01:00Z"},"kubeconfig_b64":""}`

- GET /v1/projects/{id}
  - Copy: `curl -s $AUTH_H http://localhost:8080/v1/projects/99999999-8888-7777-6666-555555555555`
  - Output: `{"project":{"id":"99999999-8888-7777-6666-555555555555","user_id":"aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee","cluster_id":"11111111-2222-3333-4444-555555555555","name":"demo","namespace":"user-aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee","created_at":"2025-01-01T12:01:00Z"},"exists":true,"details":{"resourcequota":true,"limitrange":true,"serviceaccount":true}}`

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
