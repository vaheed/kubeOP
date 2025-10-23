# API reference

All kubeOP APIs speak JSON over HTTP and listen on `PORT` (default `8080`). Authentication uses a Bearer JWT signed with
`ADMIN_JWT_SECRET` unless `DISABLE_AUTH=true`.

- Include `Authorization: Bearer <token>` on every `/v1/*` request.
- Set `Content-Type: application/json` on requests with a body.
- Responses include an `X-Request-ID` header that matches structured logs.
- See [`docs/openapi.yaml`](openapi.yaml) for full schemas.

## Health and metadata

| Method | Path | Description | Auth |
| --- | --- | --- | --- |
| `GET` | `/healthz` | Liveness probe. | No |
| `GET` | `/readyz` | Readiness probe (database + migrations). | No |
| `GET` | `/metrics` | Prometheus metrics (HTTP, scheduler). | No |
| `GET` | `/v1/version` | Build metadata (semantic version, commit, date). | No |

```bash
curl http://localhost:8080/v1/version | jq
```

## Clusters

| Method | Path | Description |
| --- | --- | --- |
| `POST` | `/v1/clusters` | Register a cluster with metadata and kubeconfig (plain or base64). |
| `GET` | `/v1/clusters` | List clusters (supports `?owner=`, `?environment=`, `?region=` filters). |
| `GET` | `/v1/clusters/{id}` | Retrieve metadata for a cluster. |
| `PATCH` | `/v1/clusters/{id}` | Update metadata (owner, contact, tags). |
| `GET` | `/v1/clusters/health` | Aggregate health summary for all clusters. |
| `GET` | `/v1/clusters/{id}/health` | Detailed health snapshot for a cluster. |
| `GET` | `/v1/clusters/{id}/status` | Scheduler history (healthy/unhealthy ticks). |

### Register a cluster

```bash
B64=$(base64 -w0 < kubeconfig)
curl -sS "${AUTH_HEADER[@]}" \
  -H 'Content-Type: application/json' \
  -d "{\"name\":\"edge-cluster\",\"kubeconfig_b64\":\"${B64}\",\"owner\":\"platform\",\"environment\":\"staging\",\"region\":\"eu-west\"}" \
  http://localhost:8080/v1/clusters | jq
```

The response includes the cluster ID, metadata, and timestamps. kubeOP encrypts the kubeconfig with AES-GCM before storing it.

## Users

| Method | Path | Description |
| --- | --- | --- |
| `POST` | `/v1/users/bootstrap` | Create a user, namespace, default project, and kubeconfig. |
| `GET` | `/v1/users` | List users with pagination. |
| `GET` | `/v1/users/{id}` | Retrieve user metadata. |
| `DELETE` | `/v1/users/{id}` | Delete a user and associated kubeconfigs. |
| `POST` | `/v1/users/{id}/kubeconfig/renew` | Rotate the user's kubeconfig. |
| `GET` | `/v1/users/{id}/projects` | List projects accessible by the user. |

### Bootstrap a user

```bash
curl -sS "${AUTH_HEADER[@]}" \
  -H 'Content-Type: application/json' \
  -d '{"name":"Alice","email":"alice@example.com","clusterId":"<cluster-id>"}' \
  http://localhost:8080/v1/users/bootstrap | jq
```

## Projects

| Method | Path | Description |
| --- | --- | --- |
| `GET` | `/v1/projects` | List projects with filters (`?clusterId=`, `?tenantId=`). |
| `POST` | `/v1/projects` | Create a project in a tenant namespace. |
| `GET` | `/v1/projects/{id}` | Fetch project metadata and status. |
| `GET` | `/v1/projects/{id}/quota` | Retrieve effective quota and usage. |
| `PATCH` | `/v1/projects/{id}/quota` | Patch quota overrides (partial updates). |
| `POST` | `/v1/projects/{id}/suspend` | Suspend mutating operations for a project. |
| `POST` | `/v1/projects/{id}/unsuspend` | Resume suspended operations. |
| `DELETE` | `/v1/projects/{id}` | Delete a project and its workloads. |
| `GET` | `/v1/projects/{id}/logs` | Stream project-level logs. |
| `GET` | `/v1/projects/{id}/events` | List stored project events. |
| `POST` | `/v1/projects/{id}/events` | Append a custom event. |

## Applications (per project)

| Method | Path | Description |
| --- | --- | --- |
| `GET` | `/v1/projects/{id}/apps` | List apps with status summaries. |
| `POST` | `/v1/projects/{id}/apps` | Deploy/update an app (image, Helm, Git, OCI bundle). |
| `GET` | `/v1/projects/{id}/apps/{appId}` | Retrieve full app status (pods, services, ingress). |
| `GET` | `/v1/projects/{id}/apps/{appId}/logs` | Stream application logs. |
| `GET` | `/v1/projects/{id}/apps/{appId}/delivery` | Delivery metadata (rendered manifests, SBOM). |
| `GET` | `/v1/projects/{id}/apps/{appId}/releases` | Release history with timestamps and digests. |
| `PATCH` | `/v1/projects/{id}/apps/{appId}/scale` | Update replica count (requires `If-Match` header). |
| `PATCH` | `/v1/projects/{id}/apps/{appId}/image` | Update container image (requires `If-Match`). |
| `POST` | `/v1/projects/{id}/apps/{appId}/rollout/restart` | Trigger a rolling restart. |
| `DELETE` | `/v1/projects/{id}/apps/{appId}` | Remove an application. |

### Validate before deploying

```bash
curl -sS "${AUTH_HEADER[@]}" \
  -H 'Content-Type: application/json' \
  -d '{"projectId":"<project-id>","name":"web","image":"ghcr.io/example/web:1.2.3","ports":[{"containerPort":80,"servicePort":80,"serviceType":"LoadBalancer"}]}' \
  http://localhost:8080/v1/apps/validate | jq '.summary'
```

`/v1/apps/validate` returns the rendered Kubernetes objects, deterministic labels (`kubeop.*`), and resource usage projections so
you can gate deployments before writing state.

### Deploy an image-backed app

```bash
curl -sS "${AUTH_HEADER[@]}" \
  -H 'Content-Type: application/json' \
  -d '{"name":"web","image":"ghcr.io/example/web:1.2.3","replicas":2,"ports":[{"containerPort":80,"servicePort":80,"serviceType":"LoadBalancer"}]}' \
  http://localhost:8080/v1/projects/<project-id>/apps | jq
```

Use the `resourceVersion` from the response when scaling or updating images:

```bash
curl -sS "${AUTH_HEADER[@]}" \
  -H 'Content-Type: application/json' \
  -H 'If-Match: "<resourceVersion>"' \
  -d '{"replicas":3}' \
  http://localhost:8080/v1/projects/<project-id>/apps/<app-id>/scale | jq
```

## Configs and secrets

| Method | Path | Description |
| --- | --- | --- |
| `POST` | `/v1/projects/{id}/configs` | Create or update a ConfigMap. |
| `GET` | `/v1/projects/{id}/configs` | List ConfigMaps managed by kubeOP. |
| `DELETE` | `/v1/projects/{id}/configs/{name}` | Delete a ConfigMap. |
| `POST` | `/v1/projects/{id}/secrets` | Create or update a Secret (Base64 data). |
| `GET` | `/v1/projects/{id}/secrets` | List Secrets managed by kubeOP. |
| `DELETE` | `/v1/projects/{id}/secrets/{name}` | Delete a Secret. |
| `POST` | `/v1/projects/{id}/apps/{appId}/configs/attach` | Mount ConfigMaps into an app. |
| `POST` | `/v1/projects/{id}/apps/{appId}/configs/detach` | Remove ConfigMap mounts. |
| `POST` | `/v1/projects/{id}/apps/{appId}/secrets/attach` | Mount Secrets into an app. |
| `POST` | `/v1/projects/{id}/apps/{appId}/secrets/detach` | Remove Secret mounts. |

## Credentials

| Method | Path | Description |
| --- | --- | --- |
| `POST` | `/v1/credentials/git` | Create Git credentials (supports personal access tokens, SSH keys). |
| `GET` | `/v1/credentials/git` | List Git credentials (filter by scope). |
| `GET` | `/v1/credentials/git/{id}` | Retrieve credential metadata. |
| `DELETE` | `/v1/credentials/git/{id}` | Delete a Git credential. |
| `POST` | `/v1/credentials/registries` | Create container registry credentials. |
| `GET` | `/v1/credentials/registries` | List registry credentials. |
| `GET` | `/v1/credentials/registries/{id}` | Retrieve registry credential metadata. |
| `DELETE` | `/v1/credentials/registries/{id}` | Delete a registry credential. |

Credential payloads include `scope` (`global`, `tenant`, or `project`) and optional bindings to restrict usage.

## Kubeconfigs

| Method | Path | Description |
| --- | --- | --- |
| `POST` | `/v1/kubeconfigs` | Ensure a kubeconfig exists for a user or project. |
| `POST` | `/v1/kubeconfigs/rotate` | Rotate an existing kubeconfig. |
| `DELETE` | `/v1/kubeconfigs/{id}` | Revoke a kubeconfig. |

## Templates

| Method | Path | Description |
| --- | --- | --- |
| `POST` | `/v1/templates` | Create a reusable application template. |
| `GET` | `/v1/templates` | List templates. |
| `GET` | `/v1/templates/{id}` | Fetch template details. |
| `POST` | `/v1/templates/{id}/render` | Render a template without deploying. |
| `POST` | `/v1/projects/{id}/templates/{templateId}/deploy` | Deploy an app from a template into a project. |

## Maintenance and operations

| Method | Path | Description |
| --- | --- | --- |
| `GET` | `/v1/admin/maintenance` | Fetch the current maintenance state. |
| `PUT` | `/v1/admin/maintenance` | Enable/disable maintenance mode with a message. |

Maintenance mode blocks mutating endpoints (clusters, projects, apps) and returns HTTP 503 with the configured message.

## Events and webhooks

| Method | Path | Description |
| --- | --- | --- |
| `POST` | `/v1/events/ingest` | Batch ingest of Kubernetes events from clusters. Optional `?clusterId=` query parameter. |
| `POST` | `/v1/webhooks/git` | Receive Git webhook payloads (validated with `GIT_WEBHOOK_SECRET`). |

When `EVENT_BRIDGE_ENABLED=true`, send an array of event objects (`projectId`, `appId`, `kind`, `severity`, `message`, `meta`) to
`/v1/events/ingest`. Responses summarise accepted events and any validation errors.

## Error handling

- Validation errors return HTTP 400 with `{"error": "message"}`.
- Authentication failures return HTTP 401 or 403 with a textual message.
- Maintenance mode returns HTTP 503 with a descriptive message.
- Server errors include a correlation ID (request ID) in the payload.

## Rate limits and timeouts

- Default request timeout is 60 seconds. Long-running operations (deployments) stream status via release endpoints.
- Health checks have a dedicated scheduler interval (see [ENVIRONMENT](ENVIRONMENT.md)).
- Apply external rate limiting via your ingress/proxy if required; kubeOP does not enforce built-in rate limits.
