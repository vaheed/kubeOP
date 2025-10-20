# Tutorial: Audit app release history

This tutorial walks through deploying a sample application and
inspecting the immutable release history captured after each rollout.
Follow along in a fresh environment after completing the quickstart or
using Docker Compose.

## Prerequisites

- kubeOP API running locally (Docker Compose or `go run ./cmd/api`).
- Admin token exported as `AUTH_H` (`export AUTH_H="-H 'Authorization: Bearer $TOKEN'"`).
- At least one project ID stored in `PROJECT_ID`:

```bash
PROJECT_ID=$(curl -s $AUTH_H "http://localhost:8080/v1/projects?limit=1" | jq -r '.[0].id')
```

## Step 1 — Deploy a sample application

Post a simple deployment that exposes port 80 through a LoadBalancer so
release metadata captures the rendered objects and load balancer quota.

```bash
APP_RESPONSE=$(curl -s $AUTH_H -H 'Content-Type: application/json' \
  -d '{
        "projectId": "'$PROJECT_ID'",
        "name": "web",
        "image": "ghcr.io/example/web:1.2.3",
        "ports": [
          {"containerPort": 80, "servicePort": 80, "serviceType": "LoadBalancer"}
        ]
      }' \
  http://localhost:8080/v1/projects/$PROJECT_ID/apps)

echo "$APP_RESPONSE" | jq
APP_ID=$(echo "$APP_RESPONSE" | jq -r '.appId')
```

The deploy response includes the generated `appId`, service name, and
ingress hostname if DNS is configured.

## Step 2 — Fetch the latest releases

List the most recent release records for the app. Each entry includes
the spec digest, rendered object summary, load balancer counts, Helm
values, and warnings captured during planning.

```bash
curl -s $AUTH_H \
  "http://localhost:8080/v1/projects/$PROJECT_ID/apps/$APP_ID/releases?limit=5" | jq
```

The response body looks similar to:

```json
{
  "releases": [
    {
      "id": "0f1b6abf-9df4-4b57-9011-3f34f2441e4d",
      "projectId": "...",
      "appId": "...",
      "createdAt": "2025-10-26T12:34:56Z",
      "source": "image",
      "specDigest": "f4c7...",
      "renderedObjects": [
        {"kind": "Deployment", "name": "web-abc123"},
        {"kind": "Service", "name": "web-abc123"}
      ],
      "loadBalancers": {"requested": 1, "existing": 0, "limit": 5},
      "warnings": []
    }
  ]
}
```

## Step 3 — Paginate through history

When multiple deployments exist, use the `nextCursor` token to walk
backwards through time.

```bash
NEXT=$(curl -s $AUTH_H \
  "http://localhost:8080/v1/projects/$PROJECT_ID/apps/$APP_ID/releases?limit=10" | jq -r '.nextCursor')

if [ "$NEXT" != "null" ] && [ -n "$NEXT" ]; then
  curl -s $AUTH_H \
    "http://localhost:8080/v1/projects/$PROJECT_ID/apps/$APP_ID/releases?limit=10&cursor=$NEXT" | jq
fi
```

Pagination stops when `nextCursor` is empty.

## Step 4 — Compare spec or render digests

Use `jq` to diff digests between releases and confirm when inputs
changed.

```bash
curl -s $AUTH_H \
  "http://localhost:8080/v1/projects/$PROJECT_ID/apps/$APP_ID/releases?limit=3" \
  | jq '.releases | map({id, specDigest, renderDigest, source})'
```

Matching digests indicate identical specs, while differences highlight
new Helm values, manifests, or image tags shipped in a rollout.
