# Kubeconfig bindings API

Manage ServiceAccount kubeconfigs for users and projects.

## `POST /v1/kubeconfigs`

Ensure a binding exists for the provided tuple.

| Field | Type | Required | Description |
| --- | --- | --- | --- |
| `userId` | string | Yes | Owner of the binding. |
| `projectId` | string | Conditional | Scope to a project. When omitted, binding is user-wide and `clusterId` is required. |
| `clusterId` | string | Conditional | Required when `projectId` is not provided. |

### Example

```bash
curl -s -X POST \
  -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{"userId":"'"$USER"'","clusterId":"'"$CLUSTER"'"}' \
  http://localhost:8080/v1/kubeconfigs | jq
```

Response fields:

- `id`, `namespace`, `service_account`, `secret_name` – metadata for the binding.
- `kubeconfig_b64` – base64-encoded kubeconfig.

Errors:

- `400 Bad Request` – missing identifiers.
- `404 Not Found` – user, project, or cluster not found.

## `POST /v1/kubeconfigs/rotate`

Rotate credentials by binding ID.

```bash
curl -s -X POST \
  -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{"id":"'"$BINDING"'"}' \
  http://localhost:8080/v1/kubeconfigs/rotate | jq
```

Response: `{ "id": "...", "secret_name": "...", "kubeconfig_b64": "..." }`.

## `DELETE /v1/kubeconfigs/{id}`

Delete a binding and clean up Kubernetes resources.

- kubeOP deletes the associated Secret and removes the binding from PostgreSQL.
- When no other bindings use the ServiceAccount, it is deleted as well.

Responses:

- `200 OK` – `{ "status": "deleted" }`.
- `404 Not Found` – unknown ID.
- `400 Bad Request` – invalid ID.
