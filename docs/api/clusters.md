# Clusters API

Manage target clusters and inspect health summaries.

## `POST /v1/clusters`

Register a new cluster with kubeOP, optionally attaching metadata used across the
inventory and health dashboards.

### Request body

| Field | Type | Required | Description |
| --- | --- | --- | --- |
| `name` | string | Yes | Friendly display name for the cluster. |
| `kubeconfig` | string | Conditional | Raw kubeconfig YAML. Ignored when `kubeconfig_b64` provided. |
| `kubeconfig_b64` | string | Conditional | Base64-encoded kubeconfig. Preferred for automation. |
| `owner` | string | No | Owning team or business unit. |
| `contact` | string | No | Optional escalation contact (email, Slack channel, etc.). |
| `environment` | string | No | Environment tag (e.g. `staging`, `production`). Defaults to `CLUSTER_DEFAULT_ENVIRONMENT` when omitted. |
| `region` | string | No | Deployment region or data centre. Defaults to `CLUSTER_DEFAULT_REGION` when omitted. |
| `apiServer` | string | No | API server URL used for documentation or dashboards. |
| `description` | string | No | Free-form notes about the cluster. |
| `tags` | array of strings | No | Optional tags for filtering (`["platform","shared"]`). Duplicates and casing are normalised. |

Provide exactly one of `kubeconfig` or `kubeconfig_b64`.

### Example

```bash
B64=$(base64 -w0 <kubeconfig)
curl -s -X POST \
  -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' \
  -d "$(jq -n --arg name 'talos-stage' --arg b64 "$B64" '{name:$name,kubeconfig_b64:$b64,"owner":"platform","environment":"staging","region":"eu-west-1","tags":["platform","staging"]}')" \
  http://localhost:8080/v1/clusters | jq
```

### Responses

- `201 Created` – returns the persisted cluster with metadata (`owner`, `environment`, `tags`, timestamps, and optional `lastStatus`).
- `400 Bad Request` – missing name, invalid kubeconfig, or decode failure.


## `GET /v1/clusters`

List registered clusters with metadata and the most recent health snapshot.

```bash
curl -s -H "Authorization: Bearer $TOKEN" http://localhost:8080/v1/clusters | jq
```

Returns an array of cluster objects with optional `lastStatus`:

```json
[
  {
    "id": "f7de1f39-0a78-4a2b-870d-1c6b48b62f6f",
    "name": "talos-stage",
    "owner": "platform",
    "environment": "staging",
    "region": "eu-west-1",
    "tags": ["platform", "staging"],
    "lastSeen": "2025-10-27T14:06:55Z",
    "lastStatus": {
      "id": "1af9f0fe-6dc7-4b40-b276-0b7a4b21ff79",
      "healthy": true,
      "message": "connected",
      "apiServerVersion": "v1.30.0",
      "checkedAt": "2025-10-27T14:06:55Z",
      "details": {"stage": "listNamespaces"}
    }
  }
]
```

## `GET /v1/clusters/{id}`

Retrieve a single cluster with full metadata and last status.

```bash
curl -s -H "Authorization: Bearer $TOKEN" http://localhost:8080/v1/clusters/$CLUSTER_ID | jq
```

- `200 OK` – cluster object or
- `404 Not Found` – unknown cluster ID.

## `PATCH /v1/clusters/{id}`

Update cluster metadata without rotating stored credentials.

```bash
curl -s -X PATCH -H "Authorization: Bearer $TOKEN" -H 'Content-Type: application/json' \
  -d '{"owner":"sre","environment":"production","tags":["platform","prod"]}' \
  http://localhost:8080/v1/clusters/$CLUSTER_ID | jq
```

The payload accepts the same metadata fields as registration. Tags are trimmed,
lowercased, and deduplicated server side.

## `GET /v1/clusters/{id}/status`

List historical health checks for a cluster (newest first). The optional
`limit` query parameter defaults to `20` and caps at `100`.

```bash
curl -s -H "Authorization: Bearer $TOKEN" \
  "http://localhost:8080/v1/clusters/$CLUSTER_ID/status?limit=5" | jq
```

Each entry contains the probe timestamp, success flag, message, API server
version, optional node count, and structured `details`.

## `GET /v1/clusters/health`

Fetch a health snapshot for every cluster. kubeOP decrypts each kubeconfig, lists namespaces (limit 1), and reports success or failure.

```bash
curl -s -H "Authorization: Bearer $TOKEN" http://localhost:8080/v1/clusters/health | jq
```

Response structure:

```json
[
  {
    "id":"cluster-uuid",
    "name":"talos-stage",
    "healthy":true,
    "message":"connected",
    "apiServerVersion":"v1.30.0",
    "checked_at":"2025-01-05T12:00:00Z",
    "details":{"stage":"listNamespaces"}
  },
  {
    "id":"cluster-uuid",
    "name":"prod",
    "healthy":false,
    "error":"context deadline exceeded",
    "message":"context deadline exceeded",
    "checked_at":"...",
    "details":{"stage":"client"}
  }
]
```

## `GET /v1/clusters/{id}/health`

Check one cluster by ID. Uses the same namespace list probe as the bulk endpoint.

```bash
curl -s -H "Authorization: Bearer $TOKEN" \
  http://localhost:8080/v1/clusters/$CLUSTER_ID/health | jq
```

- `200 OK` – health summary for the cluster including `message`, `details`, and API server version when available.
- `500 Internal Server Error` – kubeconfig decrypt failed or the Kubernetes API call returned an error string.
