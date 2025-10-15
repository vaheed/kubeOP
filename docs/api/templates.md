# Templates API

Templates store reusable deployment specifications. Currently only creation is exposed.

## `POST /v1/templates`

Create a template record.

| Field | Type | Required | Description |
| --- | --- | --- | --- |
| `name` | string | Yes | Template name. |
| `kind` | string | Yes | Template type (`helm`, `manifests`, `blueprint`). |
| `spec` | object | Yes | Arbitrary specification stored as JSON. Interpretation is caller-defined. |

Example:

```bash
curl -s $AUTH -H 'Content-Type: application/json' \
  -d '{"name":"nginx-blueprint","kind":"helm","spec":{"chart":{"repo":"https://charts.bitnami.com/bitnami","name":"nginx"}}}' \
  http://localhost:8080/v1/templates | jq
```

- `201 Created` – `{ "id": "...", "name": "...", "kind": "..." }`.
- `400 Bad Request` – missing fields or invalid JSON.
