# App validation walkthrough

This guide shows how to use the `/v1/apps/validate` endpoint to check an application payload before deploying it to Kubernetes.

## Prerequisites

- kubeOP API running locally via Docker Compose or `go run ./cmd/api`.
- Admin token exported in `AUTH_H` (see the README quickstart).
- A project ID created via `/v1/projects` or `/v1/users/bootstrap`.

## 1. Craft a validation payload

Validation accepts the same structure as `/v1/projects/{id}/apps` but includes the target `projectId` in the body:

```bash
cat <<'PAYLOAD' > validate.json
{
  "projectId": "${PROJECT_ID}",
  "name": "web",
  "image": "ghcr.io/example/web:1.2.3",
  "ports": [
    {"containerPort": 80, "servicePort": 80, "serviceType": "LoadBalancer"}
  ]
}
PAYLOAD
```

## 2. Call `/v1/apps/validate`

```bash
curl -s $AUTH_H -H 'Content-Type: application/json' \
  -d @validate.json \
  http://localhost:8080/v1/apps/validate | jq > validate-response.json
```

## 3. Review the summary

The response highlights:

- `kubeName`: deterministic resource name derived from the app ID.
- `replicas` and `resources`: effective replica count and resource overrides after applying flavors.
- `loadBalancers`: requested versus available quota (rejects if over limit).
- `renderedObjects`: list of Kubernetes objects that would be applied.

Example snippet:

```json
{
  "source": "image",
  "replicas": 1,
  "loadBalancers": {"requested": 1, "existing": 0, "limit": 2},
  "renderedObjects": [
    {"kind": "Deployment", "name": "web-abc123def", "namespace": "tenant-f8f2d7aa"},
    {"kind": "Service", "name": "web-abc123def", "namespace": "tenant-f8f2d7aa"}
  ]
}
```

Warnings (e.g. missing manifest names) appear in the `warnings[]` array. Validation errors return HTTP `400` and an `error` string.

## 4. Deploy for real

Once satisfied with the dry-run, send the same payload to `/v1/projects/{id}/apps` (using the project ID in the URL) to create the resources.

```bash
curl -s $AUTH_H -H 'Content-Type: application/json' \
  -d @validate.json \
  http://localhost:8080/v1/projects/${PROJECT_ID}/apps | jq
```
