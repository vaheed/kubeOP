# Minimal delivery walkthrough

This guide deploys a single container image and inspects the delivery metadata that kubeOP records alongside the application. Use it as a quick validation of the rendering engine, SBOM capture, and the new delivery API surface.

## Prerequisites

- A project created via `/v1/projects` with a working cluster binding.
- kubeOP API credentials with admin access and the base URL exported as `API`.

```bash
export API="http://localhost:8080"
export TOKEN="$(cat admin.jwt)"
export AUTH="Authorization: Bearer $TOKEN"
```

## Deploy an image-backed app

```bash
curl -s -XPOST "$API/v1/projects/$PROJECT_ID/apps" \
  -H "$AUTH" \
  -H 'Content-Type: application/json' \
  -d '{
        "name": "hello",
        "image": "ghcr.io/example/hello:1.0.0",
        "ports": [
          {"containerPort": 8080, "servicePort": 80, "serviceType": "ClusterIP"}
        ]
      }' | jq
```

kubeOP renders the deployment, applies it via server-side apply, and stores the delivery plan. The response contains the app ID required for the next steps.

## Inspect delivery metadata

```bash
curl -s "$API/v1/projects/$PROJECT_ID/apps/$APP_ID/delivery" -H "$AUTH" | jq
```

The payload includes:

- `source` — raw deployment request metadata (image, ports, Helm values).
- `delivery` — resolved delivery plan type (`image`, `helm`, `git`, or `ociBundle`) plus git/registry references.
- `sbom` — document digests and aggregate fingerprints derived from the rendered manifests.

## Validate a new rollout

Use the validation endpoint to preview the next release and inspect the SBOM prior to deployment.

```bash
curl -s "$API/v1/apps/validate" -H "$AUTH" -H 'Content-Type: application/json' \
  -d '{
        "projectId": "'$PROJECT_ID'",
        "name": "hello",
        "image": "ghcr.io/example/hello:1.1.0"
      }' | jq '.sbom'
```

The SBOM object mirrors the structure returned by the delivery endpoint, making it easy to diff future releases before they reach the cluster.

## Canonical tenancy labels

Every manifest applied during this walkthrough now carries kubeOP tenancy labels so operators can audit cluster resources without guessing conventions. Inspect any object and you will find `kubeop.cluster.id`, `kubeop.project.id`, `kubeop.project.name`, `kubeop.app.id`, `kubeop.app.name`, and `kubeop.tenant.id` alongside the legacy `kubeop.app-id`. These labels are injected automatically for Helm renders, raw manifest bundles, and Git/Kustomize deliveries alike.
