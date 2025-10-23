# API reference

kubeOP exposes a REST API on port `8080` (configurable via `PORT`). All endpoints return JSON and require an administrator JWT in the `Authorization: Bearer <token>` header unless noted otherwise.

## Authentication

- **Token format** – HMAC-SHA256 signed with `ADMIN_JWT_SECRET`. A minimal payload is `{ "role": "admin" }`.
- **Versioning** – `/v1/version` returns immutable build metadata (version, commit, date). Compatibility ranges and deprecation windows were removed in v0.14.0.

```bash
curl -H "Authorization: Bearer ${KUBEOP_TOKEN}" http://localhost:8080/v1/version | jq
```

## Endpoint catalogue

| Method | Path | Description |
| --- | --- | --- |
| `GET` | `/healthz` | Liveness probe (no auth). |
| `GET` | `/readyz` | Readiness probe (no auth). |
| `GET` | `/v1/version` | Build metadata (version, commit, date). |
| `GET` | `/v1/openapi` | Machine-readable OpenAPI document. |
| `POST` | `/v1/clusters` | Register a Kubernetes cluster (raw or base64 kubeconfig). |
| `GET` | `/v1/clusters` | List clusters with metadata and health summary. |
| `GET` | `/v1/clusters/{id}` | Retrieve a cluster with registration metadata. |
| `GET` | `/v1/clusters/{id}/status` | View historical health snapshots. |
| `POST` | `/v1/users/bootstrap` | Provision a user, namespace, and default project. |
| `POST` | `/v1/projects` | Create a project within a tenant namespace. |
| `GET` | `/v1/projects` | List projects. Supports pagination. |
| `GET` | `/v1/projects/{id}` | Inspect project metadata, quotas, and status. |
| `POST` | `/v1/projects/{id}/apps` | Deploy an app (image, Helm, Git, or OCI). |
| `POST` | `/v1/apps/validate` | Dry-run validation for an app spec. |
| `GET` | `/v1/projects/{id}/apps` | List apps for a project. |
| `GET` | `/v1/projects/{id}/apps/{appId}` | Get detailed app status and service endpoints. |
| `POST` | `/v1/projects/{id}/apps/{appId}/scale` | Update replica count (requires `If-Match`). |
| `POST` | `/v1/projects/{id}/apps/{appId}/image` | Update container image (requires `If-Match`). |
| `POST` | `/v1/projects/{id}/apps/{appId}/delivery` | Retrieve delivery metadata (SBOM, render plan). |
| `POST` | `/v1/credentials/git` | Store Git credentials (user, project, or global scope). |
| `POST` | `/v1/credentials/registries` | Store container registry credentials. |
| `POST` | `/v1/admin/maintenance` | Enable/disable maintenance mode (pauses mutating APIs). |
| `POST` | `/v1/events/ingest` | Ingest Kubernetes events (when `EVENT_BRIDGE_ENABLED=true`). |

Refer to [`docs/openapi.yaml`](openapi.yaml) for schemas and optional fields.

## Cluster registration

```bash
B64=$(base64 -w0 < kubeconfig)
cat <<'JSON' > payload.json
{
  "name": "edge-cluster",
  "kubeconfig_b64": "${B64}",
  "owner": "platform",
  "environment": "staging",
  "region": "eu-west",
  "tags": ["platform", "staging"]
}
JSON

curl -s ${KUBEOP_AUTH_HEADER} \
  -H 'Content-Type: application/json' \
  -d @payload.json \
  http://localhost:8080/v1/clusters | jq
```

Successful responses include the cluster ID and metadata. kubeOP expects the `kubeop-operator` to be installed separately (see [`docs/INSTALL.md`](INSTALL.md)).

## Project bootstrap

```bash
curl -s ${KUBEOP_AUTH_HEADER} -H 'Content-Type: application/json' \
  -d '{"name":"Alice","email":"alice@example.com","clusterId":"<cluster-id>"}' \
  http://localhost:8080/v1/users/bootstrap | jq
```

The response contains:

- `user` – user metadata and generated kubeconfig reference
- `project` – default project ID, namespace, quotas, and load balancer allowance
- `credentials` – optional bootstrap credentials when enabled

## App deployment

### Validate before deploy

```bash
curl -s ${KUBEOP_AUTH_HEADER} -H 'Content-Type: application/json' \
  -d '{"projectId":"<project-id>","name":"web","image":"ghcr.io/example/web:1.2.3","ports":[{"containerPort":80,"servicePort":80,"serviceType":"LoadBalancer"}]}' \
  http://localhost:8080/v1/apps/validate | jq
```

Key fields:

- `summary.manifests` – rendered Kubernetes objects (Deployment, Service, Ingress, etc.)
- `summary.labels` – canonical label set (including `kubeop.app.id`)
- `summary.quotas` – projected `ResourceQuota` usage
- `delivery` – metadata that will be persisted after a real deployment

### Deploy

```bash
curl -s ${KUBEOP_AUTH_HEADER} -H 'Content-Type: application/json' \
  -d '{"projectId":"<project-id>","name":"web","image":"ghcr.io/example/web:1.2.3","replicas":2}' \
  http://localhost:8080/v1/projects/<project-id>/apps | jq
```

Use the `If-Match` header with the App CRD `resourceVersion` when scaling or updating images to avoid conflicting writes:

```bash
curl -s ${KUBEOP_AUTH_HEADER} -H 'If-Match: "12345"' -H 'Content-Type: application/json' \
  -d '{"replicas":3}' \
  http://localhost:8080/v1/projects/<project-id>/apps/<app-id>/scale | jq
```

## Maintenance mode

```bash
curl -s ${KUBEOP_AUTH_HEADER} -H 'Content-Type: application/json' \
  -d '{"enabled":true,"reason":"database upgrade"}' \
  http://localhost:8080/v1/admin/maintenance | jq
```

When maintenance is enabled, mutating endpoints return HTTP 503 with a descriptive message. Read-only operations continue to work.

## Observability

- `GET /v1/projects/{id}/logs` – stream project logs.
- `GET /v1/projects/{id}/apps/{appId}/status` – aggregated Kubernetes status via `CollectAppStatus`.
- `GET /metrics` – Prometheus-format metrics for scraping.

Combine these APIs with your existing logging and monitoring stack to build dashboards for tenant health, app readiness, and scheduler timings.
