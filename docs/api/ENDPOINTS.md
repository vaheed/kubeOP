# kubeOP API Endpoints

All endpoints live under the configured `KUBEOP_BASE_URL`. Unless noted otherwise, supply `Authorization: Bearer <admin-token>` with a JWT whose payload includes `{"role":"admin"}`. Payloads are JSON encoded with UTF-8.

## Authentication and metadata

### `GET /healthz`
- **Description:** Liveness probe. No authentication required.
- **Response:** `200 OK` with `{"status":"ok"}`.
- **Error:** `500` only when the handler panics.

### `GET /readyz`
- **Description:** Readiness probe that checks database connectivity and optional health checkers.
- **Response:** `200 OK` with `{"status":"ready"}` when healthy.
- **Error:** `503 Service Unavailable` with `{"status":"not_ready","error":"<reason>"}` when dependencies fail.

### `GET /v1/version`
- **Description:** Returns build metadata.
- **Response:** `200 OK` with `{"version":"<semver>","commit":"<git sha>","date":"<timestamp>"}`.
- **Error:** `401` or `403` when the bearer token is missing or invalid.

### `GET /metrics`
- **Description:** Exposes Prometheus metrics without authentication.
- **Response:** `200 OK` plaintext exposition format.


- **Body:** `{"cluster_id":"<cluster-id>"}`.
- **Errors:**
  - `401 Unauthorized` when the bootstrap token is missing or invalid.
  - `400 Bad Request` when the cluster cannot be found.

- **Description:** Exchange a valid refresh token for a new access token + refresh token pair.
- **Success:** `200 OK` with the same schema as the register response.
- **Errors:**
  - `401 Unauthorized` when the refresh token is invalid or expired.
  - `400 Bad Request` for missing fields.

- **Body (optional):** `{"cluster_id":"<cluster-id>"}` when older tokens omit the claim.
- **Success:** `200 OK` with `{"status":"ok","cluster_id":"<id>"}`.
- **Errors:**
  - `400 Bad Request` with `{"status":"error","error":"cluster_id missing"}` when neither token nor body supplies the cluster ID.
  - `401 Unauthorized` if the token is invalid.

_Minimal example_
```bash
```

_Failing example_
```bash
# → HTTP 401, body: missing bearer token / invalid token
```

### `POST /v1/events/ingest`
- **Description:** Accepts batched project events from remote collectors.
- **Headers:** `Authorization: Bearer <admin-token>` (unless `DISABLE_AUTH=true`).
- **Query params:**
  - `clusterId` – optional identifier for the emitting cluster; echoed in the response.
- **Body:**
  ```json
  [
    {
      "projectId": "proj-1",
      "kind": "KUBE_EVENT",
      "severity": "WARN",
      "message": "Pod restarted",
      "actorUserId": "bridge",
      "appId": "app-42",
      "meta": {"namespace": "user-alice", "reason": "BackOff"}
    }
  ]
  ```
- **Success:** `202 Accepted` with a summary such as `{"clusterId":"<id>","total":1,"accepted":1,"dropped":0,"errors":[]}`.
- **Errors:**
  - `400 Bad Request` for invalid JSON or bodies larger than 1 MiB (`{"error":"decode json: ..."}`).
  - `202 Accepted` with `{"status":"ignored","total":N}` when `K8S_EVENTS_BRIDGE` is disabled.

_Minimal example_
```bash
curl -sS -H "Authorization: Bearer ${TOKEN}" -H "Content-Type: application/json" \
  -d '[]' "${API_ORIGIN}/v1/events/ingest?clusterId=kind-dev"
```

## Clusters

### `POST /v1/clusters`
- **Body:**
  - `name` (string, required)
  - One of `kubeconfig` (inline YAML) or `kubeconfig_b64` (base64 string)
  - Optional metadata: `owner`, `contact`, `environment`, `region`, `apiServer`, `description`, `tags`
- **Success:** `201 Created` with the stored `Cluster`, including metadata, timestamps, and any available `lastStatus` snapshot.
- **Errors:** `400` with `{"error":"name and kubeconfig required"}` or a specific decrypt/encrypt error.

_Minimal example_
```bash
payload=$(jq -n --arg name staging --arg b64 "$CLUSTER_KCFG_B64" '{name:$name,kubeconfig_b64:$b64,"environment":"staging","region":"eu-west","tags":["platform","staging"]}')
curl -sS -H "Authorization: Bearer ${TOKEN}" -H "Content-Type: application/json" \
  -d "$payload" "${API_ORIGIN}/v1/clusters"
```

_Scenario example_
```bash
curl -sS -H "Authorization: Bearer ${TOKEN}" -H "Content-Type: application/json" \
  -d "$register_payload" "${API_ORIGIN}/v1/clusters" | jq
```

_Error example_
```bash
curl -sS -H "Authorization: Bearer ${TOKEN}" -H "Content-Type: application/json" \
  -d '{}' "${API_ORIGIN}/v1/clusters"
# → HTTP 400 {"error":"name and kubeconfig required"}
```

### `GET /v1/clusters`
- **Description:** Lists all registered clusters ordered by creation time with metadata and the most recent health status.
- **Success:** `200 OK` with `[{"id":"...","name":"...","owner":"...","environment":"...","tags":[...],"lastStatus":{...}}, ...]`.
- **Error:** `500` on store failures.

### `GET /v1/clusters/{id}`
- **Description:** Fetch a single cluster record with metadata and last status.
- **Success:** `200 OK` with a cluster object.
- **Error:** `404` when the cluster ID is unknown.

### `PATCH /v1/clusters/{id}`
- **Description:** Update cluster metadata (`owner`, `environment`, `region`, `tags`, etc.) without rotating credentials.
- **Success:** `200 OK` with the updated cluster.
- **Error:** `400` for validation issues, `404` for unknown clusters.

### `GET /v1/clusters/{id}/status`
- **Description:** List historical health checks for a cluster (newest first). Accepts `?limit=` (default `20`, max `100`).
- **Success:** `200 OK` with `[ClusterStatus]` entries containing `checkedAt`, `healthy`, `message`, `apiServerVersion`, and `details`.
- **Error:** `500` on store failures.

### `GET /v1/clusters/health`
- **Description:** Returns the health summary for every cluster.
- **Success:** `200 OK` with `[{"id":"...","name":"...","healthy":true,"message":"connected","apiServerVersion":"...","checked_at":"...","details":{"stage":"..."}}, ...]`.

### `GET /v1/clusters/{id}/health`
- **Description:** Performs a lightweight namespace list on the specified cluster and persists the result as a status entry.
- **Success:** `200 OK` with `{"id":"...","name":"...","healthy":true,"message":"connected","details":{"stage":"listNamespaces"},"checked_at":"..."}`.
- **Error:** `200 OK` with `healthy:false` and `error` populated when the call fails.

## Users

### `POST /v1/users/bootstrap`
- **Description:** Creates or reuses a user, provisions their namespace, and returns a namespace-scoped kubeconfig.
- **Body:**
  - Either `userId` (existing) or both `name` and `email`
  - `clusterId` (required)
- **Success:** `201 Created` with `{"user":{...},"namespace":"user-<id>","kubeconfig_b64":"..."}`.
- **Errors:** `400` with messages such as `clusterId is required` or store errors.

_Minimal example_
```bash
jq -n --arg user "$USER_ID" --arg cluster "$CLUSTER_ID" '{userId:$user,clusterId:$cluster}' \
  | curl -sS -H "Authorization: Bearer ${TOKEN}" -H "Content-Type: application/json" \
    -d @- "${API_ORIGIN}/v1/users/bootstrap"
```

_Scenario example_
```bash
curl -sS -H "Authorization: Bearer ${TOKEN}" -H "Content-Type: application/json" \
  -d "$bootstrap_payload" "${API_ORIGIN}/v1/users/bootstrap" | jq
```

_Error example_
```bash
curl -sS -H "Authorization: Bearer ${TOKEN}" -H "Content-Type: application/json" \
  -d '{"name":"Alice"}' "${API_ORIGIN}/v1/users/bootstrap"
# → HTTP 400 {"error":"either userId or name+email are required"}
```

### `GET /v1/users`
- **Description:** Lists users (default limit 100).
- **Success:** `200 OK` with `[{"id":"...","name":"...","email":"...","created_at":"..."}, ...]`.

### `GET /v1/users/{id}`
- **Description:** Retrieves a specific user.
- **Success:** `200 OK` with the `User` object.
- **Error:** `404` with `{"error":"not found"}`.

### `DELETE /v1/users/{id}`
- **Description:** Soft deletes the user, their projects, and managed namespaces.
- **Success:** `200 OK` with `{"status":"deleted"}`.

### `POST /v1/users/{id}/kubeconfig/renew`
- **Description:** Regenerates a user namespace kubeconfig.
- **Body:** `{"clusterId":"<cluster-id>"}`.
- **Success:** `200 OK` with `{"kubeconfig_b64":"..."}`.
- **Error:** `400` if the namespace does not exist.

### `GET /v1/users/{id}/projects`
- **Description:** Lists projects owned by the user.
- **Query params:** `limit` (default 100), `offset` (default 0).
- **Success:** `200 OK` with an array of `Project` records.

## Projects

### `GET /v1/projects`
- **Description:** Lists all projects with pagination (`limit`, `offset`).
- **Success:** `200 OK` with project objects.

### `POST /v1/projects`
- **Description:** Creates a project, provisioning namespace defaults and kubeconfig when required.
- **Body:**
  - `userId` or (`userEmail` + optional `userName`)
  - `clusterId`
  - `name`
  - Optional `quotaOverrides` map (`{"services.loadbalancers":"2"}`)
- **Success:** `201 Created` with `{"project":{...},"kubeconfig_b64":"..."}` (empty string when projects share the user namespace).
- **Error:** `400` with messages such as `clusterId and name are required`.

### `GET /v1/projects/{id}`
- **Description:** Returns project existence and key resource status.
- **Success:** `200 OK` with `{"project":{...},"exists":true,"details":{"resourcequota":true,...}}`.
- **Error:** `404` when the project is missing.

### `GET /v1/projects/{id}/quota`
- **Description:** Returns quota defaults, overrides, current usage, and load balancer counts. Only valid when `PROJECTS_IN_USER_NAMESPACE=false`.
- **Success:** `200 OK` with `ProjectQuotaSnapshot` (`defaults`, `overrides`, `effective`, `resourceQuota`, `loadBalancers`).
- **Error:** `400` with `{"error":"per-project quotas not supported..."}` when projects reuse the user namespace.

### `PATCH /v1/projects/{id}/quota`
- **Description:** Applies ResourceQuota overrides.
- **Body:** `{"overrides":{"services.loadbalancers":"3"}}`.
- **Success:** `200 OK` with `{"status":"ok"}`.

### `POST /v1/projects/{id}/suspend` and `POST /v1/projects/{id}/unsuspend`
- **Description:** Toggle the project quota to zero or restore defaults (exclusive to dedicated namespaces).
- **Success:** `200 OK` with `{"status":"suspended"}` or `{"status":"unsuspended"}`.

### `DELETE /v1/projects/{id}`
- **Description:** Soft deletes the project and (if dedicated) deletes the namespace.
- **Success:** `200 OK` with `{"status":"deleted"}`.

### `POST /v1/projects/{id}/kubeconfig/renew`
- **Description:** Regenerates the project-scoped kubeconfig (dedicated namespace mode).
- **Success:** `200 OK` with `{"kubeconfig_b64":"..."}`.
- **Error:** `400` when projects share the user namespace.

## Project applications

### `GET /v1/projects/{id}/apps`
- **Description:** Lists managed applications with summary status.
- **Success:** `200 OK` with `[AppStatus, ...]` including `desired`, `ready`, `available`, `service`, `ingressHosts`, and `domains`.

### `GET /v1/projects/{id}/apps/{appId}`
- **Description:** Detailed app status including pods, service, ingress, and domain certificate status.
- **Success:** `200 OK` with an `AppStatus` document.

### `POST /v1/projects/{id}/apps`
- **Description:** Deploys a workload.
- **Body fields:**
  - `name` (required)
  - Optional `flavor`, `resources`, `replicas`, `env`, `secrets`, `ports`, `domain`, `repo`, `webhookSecret`
  - One of:
    - `image` (string)
    - `helm` (object with `chart` URL and optional `values` map)
    - `manifests` (array of YAML documents)
- **Success:** `201 Created` with `{"appId":"...","name":"...","service":"<svc>","ingress":"<ing>"}`.
- **Errors:** `400` on invalid source definition (e.g., missing image and helm/manifests).

_Minimal example_
```bash
curl -sS -H "Authorization: Bearer ${TOKEN}" -H "Content-Type: application/json" \
  -d '{"name":"demo","image":"nginx","ports":[{"containerPort":80,"servicePort":80}]}' \
  "${API_ORIGIN}/v1/projects/${PROJECT_ID}/apps"
```

_Scenario example_
```bash
curl -sS -H "Authorization: Bearer ${TOKEN}" -H "Content-Type: application/json" \
  -d "$app_payload" "${API_ORIGIN}/v1/projects/${PROJECT_ID}/apps" | jq
```

_Error example_
```bash
curl -sS -H "Authorization: Bearer ${TOKEN}" -H "Content-Type: application/json" \
  -d '{"name":"broken"}' "${API_ORIGIN}/v1/projects/${PROJECT_ID}/apps"
# → HTTP 400 {"error":"must provide image, helm, or manifests"}
```

### `PATCH /v1/projects/{id}/apps/{appId}/scale`
- **Body:** `{"replicas":2}` (must be ≥ 0).
- **Success:** `200 OK` with `{"status":"scaled"}`.

### `PATCH /v1/projects/{id}/apps/{appId}/image`
- **Body:** `{"image":"registry.example/app:tag"}`.
- **Success:** `200 OK` with `{"status":"updated"}`.

### `POST /v1/projects/{id}/apps/{appId}/rollout/restart`
- **Description:** Annotates the deployment to force a restart.
- **Success:** `200 OK` with `{"status":"restarted"}`.

### `DELETE /v1/projects/{id}/apps/{appId}`
- **Description:** Deletes Kubernetes resources labelled with the app ID and soft-deletes the DB row.
- **Success:** `200 OK` with `{"status":"deleted"}`.

## Project logs and attachments

### `GET /v1/projects/{id}/logs`
- **Description:** Returns project logs from disk.
- **Query:** `tail=<lines>` (optional, max 5000).
- **Response:** `200 OK` plaintext (streamed) or `404` when the log file does not exist.

### `GET /v1/projects/{id}/apps/{appId}/logs`
- **Description:** Streams container logs.
- **Query:** `container=<name>`, `tailLines=<n>`, `follow=<bool>` (defaults to `true`).
- **Response:** `200 OK` plaintext stream, `400` when no pods exist.

### `POST /v1/projects/{id}/configs`
- **Body:** `{"name":"config","data":{"KEY":"value"}}`.
- **Success:** `201 Created` with `{"name":"config"}`.
- **Errors:** `400` when `name` is empty.

### `GET /v1/projects/{id}/configs`
- **Success:** `200 OK` with `[{"name":"config"}, ...]`.

### `DELETE /v1/projects/{id}/configs/{name}`
- **Success:** `200 OK` with `{"status":"deleted"}`.

### `POST /v1/projects/{id}/secrets`
- **Body:** `{"name":"secret","type":"Opaque","stringData":{"KEY":"value"}}`.
- **Success:** `201 Created` with `{"name":"secret"}`.

### `GET /v1/projects/{id}/secrets`
- **Success:** `200 OK` with `[{"name":"secret","type":"Opaque"}, ...]`.

### `DELETE /v1/projects/{id}/secrets/{name}`
- **Success:** `200 OK` with `{"status":"deleted"}`.

### `POST /v1/projects/{id}/apps/{appId}/configs/attach`
- **Body:** `{"name":"config","keys":["KEY"],"prefix":"CFG_"}` (`keys` optional; attaches all keys when omitted).
- **Success:** `200 OK` with `{"status":"attached"}`.

### `POST /v1/projects/{id}/apps/{appId}/configs/detach`
- **Body:** `{"name":"config"}`.
- **Success:** `200 OK` with `{"status":"detached"}`.

### `POST /v1/projects/{id}/apps/{appId}/secrets/attach`
- **Body:** `{"name":"secret","keys":["API_KEY"],"prefix":"SECRET_"}`.
- **Success:** `200 OK` with `{"status":"attached"}`.

### `POST /v1/projects/{id}/apps/{appId}/secrets/detach`
- **Body:** `{"name":"secret"}`.
- **Success:** `200 OK` with `{"status":"detached"}`.

## Project events

### `GET /v1/projects/{id}/events`
- **Description:** Lists project events with filtering.
- **Query params:**
  - `kind` / `severity` / `actor` (comma-separated lists)
  - `since` (RFC3339 timestamp)
  - `limit` (max 500)
  - `cursor` (opaque token from previous page)
  - `grep` or `search` (case-insensitive substring)
- **Success:** `200 OK` with `{"events":[...],"nextCursor":"..."}`.

### `POST /v1/projects/{id}/events`
- **Description:** Appends a custom event.
- **Body:** `{"kind":"deployment-note","severity":"INFO","message":"...","appId":"...","meta":{...}}`.
- **Success:** `201 Created` with the stored event.
- **Error:** `400` when required fields are missing.

## Kubeconfig lifecycle

### `POST /v1/kubeconfigs`
- **Description:** Ensures a kubeconfig binding for a user or project.
- **Body:** `{"userId":"...","clusterId":"..."}` (user scope) or `{"userId":"...","projectId":"..."}` (project scope).
- **Success:** `200 OK` with `{"id":"...","cluster_id":"...","namespace":"...","service_account":"...","secret_name":"...","kubeconfig_b64":"..."}`.

### `POST /v1/kubeconfigs/rotate`
- **Body:** `{"id":"<binding-id>"}`.
- **Success:** `200 OK` with `{"id":"...","secret_name":"...","kubeconfig_b64":"..."}`.
- **Error:** `404` when the binding is missing.

### `DELETE /v1/kubeconfigs/{id}`
- **Description:** Removes the binding, deletes backing secrets, and prunes the service account if unused.
- **Success:** `200 OK` with `{"status":"deleted"}`.

## Templates

### `POST /v1/templates`
- **Description:** Register a template with JSON Schema defaults and a delivery template.
- **Body:** `{"name":"demo","kind":"helm","description":"...","schema":{...},"defaults":{...},"deliveryTemplate":"..."}`.
- **Success:** `201 Created` with stored template metadata.
- **Error:** `400` when schema compilation fails or defaults do not satisfy the schema.

### `GET /v1/templates`
- **Description:** List template summaries.
- **Success:** `200 OK` with `[{"id":"...","name":"...","kind":"...","description":"...","createdAt":"..."}]`.

### `GET /v1/templates/{id}`
- **Description:** Retrieve full template detail (schema, defaults, base, delivery template).
- **Success:** `200 OK` with the stored definition.
- **Error:** `404` when the template is missing.

### `POST /v1/templates/{id}/render`
- **Description:** Validate and render a template without deploying.
- **Body:** Optional `{"values":{...}}` overrides merged with defaults.
- **Success:** `200 OK` with `{ "template": {...}, "values": {...}, "app": {...} }`.
- **Error:** `400` when schema validation fails; `404` when the template is missing.

### `POST /v1/projects/{id}/templates/{templateId}/deploy`
- **Description:** Render a template and immediately deploy it to a project.
- **Body:** Optional `{"values":{...}}` overrides.
- **Success:** `201 Created` with `{ "appId": "...", "name": "...", "service": "...", "ingress": "..." }`.
- **Error:** `400` when validation or deployment fails; `404` when the project or template is missing.

## Webhooks

### `POST /v1/webhooks/git`
- **Description:** Accepts GitHub-style push payloads and triggers rollouts for matching apps.
- **Headers:** Optional `X-Hub-Signature-256` for HMAC verification (per-app `webhookSecret` or global `GIT_WEBHOOK_SECRET`).
- **Body:** Arbitrary JSON payload. kubeOP inspects `repository.full_name` (or `repository.clone_url`) and `ref`.
- **Success:** `200 OK` with `{"status":"handled"}`.
- **Error:** `400` with the validation message (e.g., `unsupported payload: missing repository/ref` or `signature mismatch`).

_Minimal example_
```bash
curl -sS -H "Authorization: Bearer ${TOKEN}" -H "Content-Type: application/json" \
  -d '{"ref":"refs/heads/main","repository":{"full_name":"example/app"}}' \
  "${API_ORIGIN}/v1/webhooks/git"
```

_Error example_
```bash
curl -sS -H "Authorization: Bearer ${TOKEN}" -H "X-Hub-Signature-256: sha256=deadbeef" \
  -H "Content-Type: application/json" \
  -d '{"ref":"refs/heads/main"}' "${API_ORIGIN}/v1/webhooks/git"
# → HTTP 400 unsupported payload: missing repository/ref
```

---

Every endpoint mirrors the structs in `internal/api` and `internal/service`. Use the zero-to-production guide for end-to-end workflows and combine these references to build bespoke automation.
