# Tutorial: Application templates from zero to deployment

This tutorial walks through registering a template, rendering it for review, and
finally deploying it to a project. The flow assumes a fresh environment following
the quickstart.

## Prerequisites

- kubeOP API running locally (Docker Compose or `go run ./cmd/api`).
- Admin token exported as `AUTH_H` (`export AUTH_H="-H 'Authorization: Bearer $TOKEN'"`).
- At least one project ID available in `PROJECT_ID`:

```bash
PROJECT_ID=$(curl -s $AUTH_H "http://localhost:8080/v1/projects?limit=1" | jq -r '.[0].id')
```

## Step 1 — Register a reusable template

```bash
TEMPLATE_ID=$(curl -s $AUTH_H -H 'Content-Type: application/json' \
  -d '{
        "name": "nginx-template",
        "kind": "helm",
        "description": "Baseline nginx deployment",
        "schema": {"type":"object","properties":{"name":{"type":"string"}},"required":["name"]},
        "defaults": {"name": "web"},
        "deliveryTemplate": "{\\n  \\\"name\\\": \\\"{{ .values.name }}\\\",\\n  \\\"image\\\": \\\"ghcr.io/library/nginx:1.27\\\"\\n}"
      }' \
  http://localhost:8080/v1/templates | jq -r '.id')
```

The response returns the template identifier used in later steps.

## Step 2 — Render without touching the cluster

```bash
curl -s $AUTH_H -H 'Content-Type: application/json' \
  -d '{"values":{"name":"web-blue"}}' \
  http://localhost:8080/v1/templates/${TEMPLATE_ID}/render | jq
```

The payload includes:

- `template` — summary metadata (name, kind, description).
- `values` — merged defaults plus overrides.
- `app` — the deploy-ready spec that can be posted to `/v1/projects/{id}/apps`.

## Step 3 — Deploy via the template endpoint

```bash
curl -s $AUTH_H -H 'Content-Type: application/json' \
  -d '{"values":{"name":"web-blue"}}' \
  http://localhost:8080/v1/projects/${PROJECT_ID}/templates/${TEMPLATE_ID}/deploy | jq
```

The response matches the regular deploy API, returning the new `appId`, service
name, and ingress host if one was created.

## Step 4 — Track template usage

List templates to confirm the blueprint is catalogued:

```bash
curl -s $AUTH_H http://localhost:8080/v1/templates | jq
```

Fetch the detail view to review defaults and the delivery template:

```bash
curl -s $AUTH_H http://localhost:8080/v1/templates/${TEMPLATE_ID} | jq
```

Use the `/render` endpoint whenever new overrides are required. The JSON Schema
stored with the template guarantees every deployment honours the approved
contract.
