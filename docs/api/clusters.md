# Clusters API

Manage target clusters and inspect health summaries.

## `POST /v1/clusters`

Register a new cluster with kubeOP.

### Request body

| Field | Type | Required | Description |
| --- | --- | --- | --- |
| `name` | string | Yes | Display name for logs and watcher provisioning. |
| `kubeconfig` | string | Conditional | Raw kubeconfig YAML. Ignored when `kubeconfig_b64` provided. |
| `kubeconfig_b64` | string | Conditional | Base64-encoded kubeconfig. Preferred for automation. |

Provide exactly one of `kubeconfig` or `kubeconfig_b64`.

### Example

```bash
B64=$(base64 -w0 <kubeconfig)
curl -s -X POST \
  -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{"name":"talos-stage","kubeconfig_b64":"'"$B64"'"}' \
  http://localhost:8080/v1/clusters | jq
```

### Responses

- `201 Created` – returns `{ "id": "...", "name": "...", "created_at": "..." }`.
- `400 Bad Request` – missing name, invalid kubeconfig, or decode failure.

kubeOP encrypts kubeconfigs with `KCFG_ENCRYPTION_KEY` and stores them in PostgreSQL. When watcher auto-deploy is enabled, `RegisterCluster` also provisions namespace/RBAC/PVC/Deployment resources and waits for readiness before returning.

## `GET /v1/clusters`

List registered clusters.

```bash
curl -s -H "Authorization: Bearer $TOKEN" http://localhost:8080/v1/clusters | jq
```

Returns an array of cluster objects.

## `GET /v1/clusters/health`

Fetch a health snapshot for every cluster. kubeOP decrypts each kubeconfig, lists namespaces (limit 1), and reports success or failure.

```bash
curl -s -H "Authorization: Bearer $TOKEN" http://localhost:8080/v1/clusters/health | jq
```

Response structure:

```json
[
  {"id":"cluster-uuid","name":"talos-stage","healthy":true,"checked_at":"2025-01-05T12:00:00Z"},
  {"id":"cluster-uuid","name":"prod","healthy":false,"error":"context deadline exceeded","checked_at":"..."}
]
```

## `GET /v1/clusters/{id}/health`

Check one cluster by ID. Uses the same namespace list probe as the bulk endpoint.

```bash
curl -s -H "Authorization: Bearer $TOKEN" \
  http://localhost:8080/v1/clusters/$CLUSTER_ID/health | jq
```

- `200 OK` – health summary for the cluster (including error text when unhealthy).
- `500 Internal Server Error` – kubeconfig decrypt failed or the Kubernetes API call returned an error string.
