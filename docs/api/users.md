# Users API

Bootstrap tenant namespaces, manage user records, and rotate credentials.

## `POST /v1/users/bootstrap`

Create or reuse a user and provision their namespace on a cluster.

### Request body

| Field | Type | Required | Description |
| --- | --- | --- | --- |
| `userId` | string | No | Existing user ID. Reuses the namespace when provided. |
| `name` | string | Required if `userId` omitted | Display name for new users. |
| `email` | string | Required if `userId` omitted | Used to deduplicate users; case-insensitive. |
| `clusterId` | string | Yes | Target cluster ID. |

### Example

```bash
curl -s -X POST \
  -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{"name":"Alice","email":"alice@example.com","clusterId":"'"$CLUSTER"'"}' \
  http://localhost:8080/v1/users/bootstrap | jq
```

### Responses

- `201 Created` – `{ "user": {...}, "namespace": "...", "kubeconfig_b64": "..." }`.
- `400 Bad Request` – missing fields or validation errors (blank email, unknown cluster, etc.).

kubeOP labels namespaces, applies quota/limit policies, and stores the kubeconfig encrypted before returning.

## `GET /v1/users`

List users. The current implementation returns up to 100 records starting at offset 0.

```bash
curl -s -H "Authorization: Bearer $TOKEN" http://localhost:8080/v1/users | jq
```

## `GET /v1/users/{id}`

Fetch a single user by ID. Returns `404 Not Found` if the user does not exist.

## `DELETE /v1/users/{id}`

Soft-delete the user, associated projects/apps, and remove namespaces from every cluster.

- `200 OK` – `{ "status": "deleted" }`.
- `400 Bad Request` – invalid ID or store failure.

## `POST /v1/users/{id}/kubeconfig/renew`

Rotate credentials for the user namespace ServiceAccount.

### Request body

| Field | Type | Required | Description |
| --- | --- | --- | --- |
| `clusterId` | string | Yes | Cluster hosting the namespace. |

### Example

```bash
curl -s -X POST \
  -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{"clusterId":"'"$CLUSTER"'"}' \
  http://localhost:8080/v1/users/$USER_ID/kubeconfig/renew | jq
```

Response: `{ "kubeconfig_b64": "..." }`.

## `GET /v1/users/{id}/projects`

List projects owned by a user.

```bash
curl -s -H "Authorization: Bearer $TOKEN" http://localhost:8080/v1/users/$USER_ID/projects | jq
```

Supports optional `limit` and `offset` query parameters.
