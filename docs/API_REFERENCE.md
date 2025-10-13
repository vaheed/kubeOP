API Reference

Auth

- All `/v1/*` endpoints require a Bearer token signed with `ADMIN_JWT_SECRET` and claim `{ "role": "admin" }`.
- To test without auth, set `DISABLE_AUTH=true` (development only). Examples below show both modes.

Conventions

- Base URL: `http://localhost:8080` unless noted.
- Variables used in examples:
  - `TOKEN` is a valid JWT; examples omit token generation.
  - `AUTH_H="-H 'Authorization: Bearer $TOKEN'"`
- Every response includes `X-Request-Id`. Provide the same header on follow-up support requests so operators can locate the access/audit log entries.

Quickstart (Default: Shared User Namespace)

1) Register a cluster (base64 kubeconfig required)

- Linux/macOS: `B64=$(base64 -w0 < kubeconfig)`
- Windows (PowerShell): `$B64 = [Convert]::ToBase64String([IO.File]::ReadAllBytes('kubeconfig'))`
- `curl -s $AUTH_H -H 'Content-Type: application/json' -d "$(jq -n --arg n 'my-cluster' --arg b64 "$B64" '{name:$n,kubeconfig_b64:$b64}')" http://localhost:8080/v1/clusters`

2) Bootstrap a user (get a user-namespace kubeconfig)

- Create/reuse by email: `curl -s $AUTH_H -H 'Content-Type: application/json' -d '{"name":"Alice","email":"alice@example.com","clusterId":"<cluster-id>"}' http://localhost:8080/v1/users/bootstrap`
- Or use an existing user: `curl -s $AUTH_H -H 'Content-Type: application/json' -d '{"userId":"<user-id>","clusterId":"<cluster-id>"}' http://localhost:8080/v1/users/bootstrap`
- Save kubeconfig: `echo "BASE64_FROM_RESPONSE" | base64 -d > user.kubeconfig`
- Use: user kubeconfigs are namespace-scoped. Cluster-wide actions like `kubectl get ns` are forbidden. Verify with namespaced commands, e.g. `KUBECONFIG=./user.kubeconfig kubectl -n user-<userId> get pods` or `kubectl -n user-<userId> get resourcequota`.

3) Create projects inside the user namespace

- `curl -s $AUTH_H -H 'Content-Type: application/json' -d '{"userId":"<user-id>","clusterId":"<cluster-id>","name":"demo"}' http://localhost:8080/v1/projects`
- Response omits kubeconfig (reuse the user kubeconfig)

End-to-end example

- See docs/QUICKSTART_API.md:1 for a concise, copy/paste walkthrough:
  - Register cluster → bootstrap user → create project → create app → delete app → delete project.
  - Includes listings for IDs and notes on user deletion (not yet an API).

Common mistakes

- 404 for /v1/project → path is plural: `/v1/projects`.

Health

- GET `/healthz` → 200 `{ "status": "ok" }`
  - curl: `curl -s http://localhost:8080/healthz`

- GET `/readyz` → 200 `{ "status": "ready" }` if DB reachable. Returns 503 `{ "status": "not_ready", "error": "service unavailable" }` when the service layer is missing or dependencies fail health checks, incrementing the `readyz_failures_total` Prometheus counter with `reason=service_missing|health_check_failed`.
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
  - Response reflects the latest execution of the `ClusterHealthScheduler`, which bounds each probe to 20s by default.

- GET `/v1/clusters/{id}/health` → 200 `{ "id": "...", "name": "...", "healthy": true|false, "error": "...", "checked_at": "..." }`
  - With auth: `curl -s $AUTH_H http://localhost:8080/v1/clusters/<id>/health`
  - Use when triaging individual clusters surfaced by the scheduler summary.

Projects

- POST `/v1/projects`
  - Request (either form):
    - With userId: `{ "userId": "<uuid>", "clusterId": "<uuid>", "name": "my-project", "quotaOverrides": {"limits.cpu":"256"} }`
    - With userEmail (auto-create or reuse): `{ "userEmail": "alice@example.com", "userName": "Alice", "clusterId": "<uuid>", "name": "my-project" }`
  - Behavior (v0.1.2): default is shared user namespace; project responses omit kubeconfig. If `PROJECTS_IN_USER_NAMESPACE=false`, each project gets its own namespace and returns a namespace-scoped kubeconfig.
  - Response: `201 { "project": {"id":"...","user_id":"...","cluster_id":"...","name":"...","namespace":"...","created_at":"..."}, "kubeconfig_b64":"..." }`
  - Curl: `curl -s $AUTH_H -H 'Content-Type: application/json' -d '{"userId":"<uuid>","clusterId":"<uuid>","name":"demo"}' http://localhost:8080/v1/projects`

- GET `/v1/projects/{id}` → Status (exists, details)
  - Curl: `curl -s $AUTH_H http://localhost:8080/v1/projects/<id>`

- GET `/v1/projects` → List all projects
  - Curl: `curl -s $AUTH_H http://localhost:8080/v1/projects`
  - Output: `[{"id":"...","user_id":"...","cluster_id":"...","name":"...","namespace":"...","created_at":"..."}, ... ]`

- GET `/v1/users/{id}/projects` → List projects for a user
  - Curl: `curl -s $AUTH_H http://localhost:8080/v1/users/<user-id>/projects`
  - Output: `[{"id":"...","user_id":"<user-id>","cluster_id":"...","name":"...","namespace":"...","created_at":"..."}, ... ]`

- PATCH `/v1/projects/{id}/quota`
  - Request: `{ "overrides": { "pods": "100", "limits.cpu": "256" } }`
  - Response: `200 { "status": "ok" }`

- POST `/v1/projects/{id}/suspend` and `/v1/projects/{id}/unsuspend`
  - Only for per-project namespace mode. Sets or removes quota blocks.

- DELETE `/v1/projects/{id}`
  - Behavior: soft-delete in DB; hard delete in Kubernetes.
  - Per-project mode: deletes the project namespace in the cluster.
  - Shared user namespace mode: removes project-specific LimitRange (namespace remains).

Apps

- POST `/v1/projects/{id}/apps` — see examples above and docs/APPS.md:1
- GET `/v1/projects/{id}/apps` — list apps with summary
  - Copy: `curl -s $AUTH_H http://localhost:8080/v1/projects/<project-id>/apps`
- GET `/v1/projects/{id}/apps/{appId}` — detailed status
  - Copy: `curl -s $AUTH_H http://localhost:8080/v1/projects/<project-id>/apps/<appId>`
- GET `/v1/projects/{id}/apps/{appId}/logs` — stream container logs
- PATCH `/v1/projects/{id}/apps/{appId}/scale` — `{ "replicas": 2 }`
- PATCH `/v1/projects/{id}/apps/{appId}/image` — `{ "image": "nginx:1.27" }`
- POST `/v1/projects/{id}/apps/{appId}/rollout/restart` — restart rollout
- DELETE `/v1/projects/{id}/apps/{appId}` — soft-delete in DB; delete labeled K8s resources (Deployment/Service/Ingress, etc.)

Configs & Secrets

1. **Manage ConfigMaps**
   - POST `/v1/projects/{id}/configs` — `{ "name":"app-config", "data": {"KEY":"VAL"} }`
   - GET `/v1/projects/{id}/configs`
   - DELETE `/v1/projects/{id}/configs/{name}`
2. **Attach a ConfigMap to an app**
   - All keys (envFrom): `POST /v1/projects/{id}/apps/{appId}/configs/attach` with `{ "name":"app-config" }`
   - Selective keys with prefix: send `{ "name":"app-config", "keys":["LOG_LEVEL"], "prefix":"APP_" }`
3. **Detach ConfigMaps**
   - `POST /v1/projects/{id}/apps/{appId}/configs/detach` — `{ "name":"app-config" }` (removes envFrom + env vars)
4. **Manage Secrets**
   - POST `/v1/projects/{id}/secrets` — `{ "name":"app-secret", "stringData": {"PASSWORD":"..."} }`
   - GET `/v1/projects/{id}/secrets`
   - DELETE `/v1/projects/{id}/secrets/{name}`
5. **Attach Secrets**
   - `POST /v1/projects/{id}/apps/{appId}/secrets/attach` with `{ "name":"app-secret" }`
   - To map keys individually: `{ "name":"app-secret", "keys":["TOKEN"], "prefix":"APP_" }`
6. **Detach Secrets**
   - `POST /v1/projects/{id}/apps/{appId}/secrets/detach` — `{ "name":"app-secret" }`

Users

- GET `/v1/users/{id}` — fetch a single user
  - Copy: `curl -s $AUTH_H http://localhost:8080/v1/users/<user-id>`

- DELETE `/v1/users/{id}` — soft-delete user in DB; delete user namespaces across clusters.

Users (Shared Namespace Mode)

- POST `/v1/users/bootstrap`
  - Purpose: provision a per-user namespace on a specific cluster and return a user-scoped kubeconfig. “Bootstrap” sets up Kubernetes resources; it is not only a user creation API.
  - Request (either form):
    - Create/reuse by email: `{ "name": "Alice", "email": "alice@example.com", "clusterId": "<uuid>" }`
    - Or existing user: `{ "userId": "<uuid>", "clusterId": "<uuid>" }`
  - Why `userId` may be required: the API must know which user to provision on which cluster. If you already have a user, send `userId`. If not, send `name`+`email` and it will create or reuse a user by email.
  - Effect: creates namespace `user-<userId>` on the target cluster with quotas/limits/PSA labels, creates ServiceAccount and Role/Binding, mints a token, stores an encrypted kubeconfig, and returns it base64.
  - Response: `201 { "user": { ... }, "namespace": "user-...", "kubeconfig_b64": "..." }`
  - Curl (create by email): `curl -s $AUTH_H -H 'Content-Type: application/json' -d '{"name":"Alice","email":"alice@example.com","clusterId":"<uuid>"}' http://localhost:8080/v1/users/bootstrap`
  - Curl (existing user): `curl -s $AUTH_H -H 'Content-Type: application/json' -d '{"userId":"<uuid>","clusterId":"<uuid>"}' http://localhost:8080/v1/users/bootstrap`
- RBAC note: user kubeconfigs are namespace-scoped and cannot list cluster-scoped resources like `namespaces`. Use namespaced commands, e.g. `kubectl -n user-<userId> get pods` or `kubectl -n user-<userId> get resourcequota` to verify access.

- Kubeconfig renew: `POST /v1/users/{id}/kubeconfig/renew` with `{ "clusterId":"<uuid>" }`
  - Copy: `curl -s $AUTH_H -H 'Content-Type: application/json' -d '{"clusterId":"<cluster-id>"}' http://localhost:8080/v1/users/<user-id>/kubeconfig/renew`

Tenancy modes quick guide

- Shared user namespace (default, `PROJECTS_IN_USER_NAMESPACE=true`):
  - Bootstrap user once per cluster via `/v1/users/bootstrap` to get the kubeconfig for the user namespace.
  - Create projects via `/v1/projects { userId, clusterId, name }` — response omits kubeconfig; reuse the user kubeconfig for all projects.
  - Quotas/suspend: manage limits at the user namespace level; project suspend/quota endpoints are not applicable.
- Per-project namespaces (`PROJECTS_IN_USER_NAMESPACE=false`):
  - Create project via `/v1/projects` — response includes a project-scoped kubeconfig.
  - Use project quota/suspend endpoints for per-project control.

Examples (Copy + Expected Output)

- GET /healthz
  - Copy: `curl -s http://localhost:8080/healthz`
  - Output: `{"status":"ok"}`

- GET /readyz
  - Copy: `curl -s http://localhost:8080/readyz`
  - Output (ready): `{"status":"ready"}`
  - Output (service missing): `{"status":"not_ready","error":"service unavailable"}`

- GET /v1/version
  - Copy: `curl -s http://localhost:8080/v1/version`
  - Output: `{"version":"0.3.1","commit":"<git-sha>","date":"<build-date>"}`

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

Apps & Templates

- POST `/v1/templates`
  - Request: `{ "name":"nginx", "kind":"manifests", "spec": {"docs": ["...yaml..."] } }`
  - Response: `201 { "id":"...", "name":"...", "kind":"..." }`

- POST `/v1/projects/{id}/apps`
  - Request (image): `{ "name":"web", "image":"nginx:1.27", "ports":[{"containerPort":80,"servicePort":80,"serviceType":"LoadBalancer"}] }`
  - Optional: `flavor`, `resources`, `replicas`, `env`, `secrets`, `domain`, `repo`, `webhookSecret`.
  - Response: `201 { "appId":"...", "name":"web", "service":"web", "ingress":"web" }`

- GET `/v1/projects/{id}/apps/{appId}/logs`
  - Query params: `container`, `tailLines`, `follow=true|false`
  - Response: `200` stream text/plain

Kubeconfig Lifecycle

- POST `/v1/projects/{id}/kubeconfig/renew`
  - Per-project mode only. Returns new short-lived token kubeconfig.
  - Response: `200 { "kubeconfig_b64": "..." }`

CI Webhooks

- POST `/v1/webhooks/git`
  - Verifies signature using per-app `webhookSecret` if provided, otherwise `GIT_WEBHOOK_SECRET` if set.
  - On push, redeploys associated apps (best-effort).

Metrics

- GET `/metrics` → Prometheus text exposition format.
