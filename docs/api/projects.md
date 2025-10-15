# Projects and apps API

Projects organise workloads within a tenant namespace. This page covers project lifecycle, application deployment, logs, events, and configuration endpoints.

## Projects

### `POST /v1/projects`

Create a project for a user on a cluster.

| Field | Type | Required | Description |
| --- | --- | --- | --- |
| `userId` | string | Conditional | Existing user ID; optional if `userEmail` and `userName` provided. |
| `userEmail` | string | Conditional | Used to resolve/create user when `userId` missing. |
| `userName` | string | Conditional | Display name when creating a user by email. |
| `clusterId` | string | Yes | Target cluster ID. |
| `name` | string | Yes | Project display name. |
| `quotaOverrides` | object | No | ResourceQuota overrides (e.g. `{ "limits.cpu": "6" }`). |

Response: `{ "project": {...}, "kubeconfig_b64": "..." }`. kubeOP stores the project metadata, applies quota/limit objects, and returns a namespace-scoped kubeconfig for the project service account.

### `GET /v1/projects`

List projects. Supports `limit` and `offset` query parameters (defaults: 100 and 0).

### `GET /v1/projects/{id}`

Return project metadata (`namespace`, `cluster_id`, `suspended`, timestamps). This call does not embed application status.

### `DELETE /v1/projects/{id}`

Remove kubeOP-managed resources (Deployments, Services, Ingresses, ConfigMaps, Secrets) and soft-delete the project record.

### Suspension

- `POST /v1/projects/{id}/suspend` – scale workloads to zero / delete Services and Ingresses.
- `POST /v1/projects/{id}/unsuspend` – reapply workloads from stored specs and reconcile quota/limit objects.

### Quotas

- `GET /v1/projects/{id}/quota` – returns defaults, overrides, current usage, and the active LimitRange snapshot.
- `PATCH /v1/projects/{id}/quota` – provide `{ "overrides": { "limits.cpu": "6" } }` to adjust ResourceQuota values. Overrides persist in PostgreSQL.

## Apps

### `POST /v1/projects/{id}/apps`

Deploy a new app. Provide exactly one source (`image`, `helm`, or `manifests`). See [deployment guide](../guides/helm-oci-deployments.md) for payload examples.

- `201 Created` – returns `{ "appId": "...", "name": "...", "service": "...", "ingress": "..." }`.
- `400 Bad Request` – missing project ID/name, multiple sources, invalid manifests, Helm render error, etc.

### `GET /v1/projects/{id}/apps`

List app statuses. Response is an array of `AppStatus` objects (desired/ready/available replicas, service summary, ingress hosts, pod readiness).

### `GET /v1/projects/{id}/apps/{appId}`

Return detailed status for one app. kubeOP queries live Deployment, Service, Ingress, and Pod data.

### `DELETE /v1/projects/{id}/apps/{appId}`

Remove an app and associated Kubernetes resources created by kubeOP.

### Scaling and updates

| Endpoint | Body | Description |
| --- | --- | --- |
| `PATCH /v1/projects/{id}/apps/{appId}/scale` | `{ "replicas": 3 }` | Update Deployment replicas (must be ≥0). |
| `PATCH /v1/projects/{id}/apps/{appId}/image` | `{ "image": "repo/image:tag" }` | Update container image and trigger rollout. |
| `POST /v1/projects/{id}/apps/{appId}/rollout/restart` | _none_ | Annotate Deployment to force rollout. |

### Logs

- `GET /v1/projects/{id}/apps/{appId}/logs?container=<name>&tailLines=200&follow=true`
  - Streams container logs. `follow` defaults to true; set `follow=false` to exit after streaming. `tailLines` max is enforced by the Kubernetes API.
- `GET /v1/projects/{id}/logs?tail=500`
  - Returns the last N lines of the consolidated project log file. Without `tail`, serves the entire file (limit 5000 lines). Missing logs return `404`.

### Events

- `GET /v1/projects/{id}/events?kind=APP_DEPLOYMENT&severity=INFO`
  - Filters: `kind`, `severity`, `actor`, `since` (RFC3339), `limit`, `cursor`, `grep`/`search`.
- `POST /v1/projects/{id}/events`
  - Body: `{ "kind": "DEPLOY", "severity": "INFO", "message": "rollout complete", "appId": "...", "meta": {"sha": "..."} }`
  - Returns the persisted event (with generated `id` and timestamp).

### Configs and secrets

| Operation | Endpoint | Body |
| --- | --- | --- |
| Create ConfigMap | `POST /v1/projects/{id}/configs` | `{ "name": "app-config", "data": {"KEY": "value"} }` |
| List ConfigMaps | `GET /v1/projects/{id}/configs` | _none_ |
| Delete ConfigMap | `DELETE /v1/projects/{id}/configs/{name}` | _none_ |
| Create Secret | `POST /v1/projects/{id}/secrets` | `{ "name": "db-creds", "type": "Opaque", "stringData": {"USER": "alice"} }` |
| List Secrets | `GET /v1/projects/{id}/secrets` | _none_ |
| Delete Secret | `DELETE /v1/projects/{id}/secrets/{name}` | _none_ |

### Attach/detach configuration

- `POST /v1/projects/{id}/apps/{appId}/configs/attach`
  - `{ "name": "app-config", "keys": ["APP_ENV"], "prefix": "APP_" }`
- `POST /v1/projects/{id}/apps/{appId}/configs/detach`
  - `{ "name": "app-config" }`
- `POST /v1/projects/{id}/apps/{appId}/secrets/attach`
  - `{ "name": "db-creds", "prefix": "DB_" }`
- `POST /v1/projects/{id}/apps/{appId}/secrets/detach`
  - `{ "name": "db-creds" }`

Attach endpoints mutate the Deployment PodSpec via server-side apply so mounts and environment variables stay consistent across rollouts.
