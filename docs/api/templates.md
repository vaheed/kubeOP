# Templates API

Templates capture JSON Schema–validated blueprints that can be rendered and
deployed repeatedly.

## `POST /v1/templates`

Register a new template.

| Field | Type | Required | Description |
| --- | --- | --- | --- |
| `name` | string | Yes | Template display name. |
| `kind` | string | Yes | Categorisation label (`helm`, `manifests`, etc.). |
| `description` | string | Yes | Short summary shown in catalog listings. |
| `schema` | object | Yes | JSON Schema describing `values` supplied at render time. |
| `defaults` | object | Yes | Baseline values merged with user overrides. Must satisfy `schema`. |
| `example` | object | No | Optional example payload for documentation. |
| `base` | object | No | Static spec fragments merged into every render. |
| `deliveryTemplate` | string | Yes | Go text/template that emits the deploy payload (YAML or JSON). `values` and `base` are available in the template context. |

```bash
curl -s $AUTH_H -H 'Content-Type: application/json' \
  -d '{
        "name": "nginx-template",
        "kind": "helm",
        "description": "Baseline nginx deployment",
        "schema": {"type":"object","properties":{"name":{"type":"string"}},"required":["name"]},
        "defaults": {"name": "web"},
        "deliveryTemplate": "{\\n  \\\"name\\\": \\\"{{ .values.name }}\\\",\\n  \\\"image\\\": \\\"ghcr.io/library/nginx:1.27\\\"\\n}"
      }' \
  http://localhost:8080/v1/templates | jq
```

- `201 Created` – returns the stored template metadata.
- `400 Bad Request` – schema compilation errors or invalid defaults.

## `GET /v1/templates`

List templates ordered by creation time (newest first).

```bash
curl -s $AUTH_H http://localhost:8080/v1/templates | jq
```

- `200 OK` – array of template summaries (`id`, `name`, `kind`, `description`, `createdAt`).

## `GET /v1/templates/{id}`

Fetch the full template definition including schema, defaults, and delivery template.

```bash
curl -s $AUTH_H http://localhost:8080/v1/templates/${TEMPLATE_ID} | jq
```

- `200 OK` – full template detail.
- `404 Not Found` – unknown template ID.

## `POST /v1/templates/{id}/render`

Merge defaults with overrides, validate against the stored schema, and return the
rendered application spec without touching Kubernetes.

```bash
curl -s $AUTH_H -H 'Content-Type: application/json' \
  -d '{"values":{"name":"web-blue"}}' \
  http://localhost:8080/v1/templates/${TEMPLATE_ID}/render | jq
```

- `200 OK` – `{ "template": {...}, "values": {...}, "app": {...} }`.
- `400 Bad Request` – schema validation failed or template rendering error.
- `404 Not Found` – unknown template ID.

## `POST /v1/projects/{id}/templates/{templateId}/deploy`

Render and deploy in a single call. The response mirrors `POST /v1/projects/{id}/apps`.

```bash
curl -s $AUTH_H -H 'Content-Type: application/json' \
  -d '{"values":{"name":"web-blue"}}' \
  http://localhost:8080/v1/projects/<project-id>/templates/${TEMPLATE_ID}/deploy | jq
```

- `201 Created` – `{ "appId": "...", "name": "...", "service": "...", "ingress": "..." }`.
- `400 Bad Request` – template validation failed or deployment error.
- `404 Not Found` – project or template not found.
