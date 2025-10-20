# API highlights

## Validate application specs before deployment

Use `POST /v1/apps/validate` to dry-run an application payload before deploying it to a project. The endpoint returns the computed Kubernetes resource names, replicas, resource overrides, load balancer quota consumption, and a summary of manifests that would be applied.

```bash
curl -s $AUTH_H -H 'Content-Type: application/json' \
  -d '{
        "projectId": "<project-id>",
        "name": "web",
        "image": "ghcr.io/example/web:1.2.3",
        "ports": [
          {"containerPort": 80, "servicePort": 80, "serviceType": "LoadBalancer"}
        ]
      }' \
  http://localhost:8080/v1/apps/validate | jq
```

A successful response resembles:

```json
{
  "projectId": "f8f2d7aa-2c86-4e7b-8b42-0c741ef7d4f2",
  "projectNamespace": "tenant-f8f2d7aa",
  "clusterId": "cluster-1",
  "source": "image",
  "kubeName": "web-abc123def",
  "replicas": 1,
  "loadBalancers": {"requested": 1, "existing": 0, "limit": 2},
  "renderedObjects": [
    {"kind": "Deployment", "name": "web-abc123def", "namespace": "tenant-f8f2d7aa"},
    {"kind": "Service", "name": "web-abc123def", "namespace": "tenant-f8f2d7aa"},
    {"kind": "Ingress", "name": "web-abc123def", "namespace": "tenant-f8f2d7aa"}
  ]
}
```

Validation errors return HTTP `400` with a descriptive `error` message.

## Publish and reuse application templates

Templates provide JSON Schema–validated blueprints that teams can render, review,
and deploy with a single command.

```bash
# Register a template with defaults and rendering logic
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

# Render without touching Kubernetes
curl -s $AUTH_H -H 'Content-Type: application/json' \
  -d '{"values":{"name":"web-blue"}}' \
  http://localhost:8080/v1/templates/${TEMPLATE_ID}/render | jq

# Deploy directly to a project
curl -s $AUTH_H -H 'Content-Type: application/json' \
  -d '{"values":{"name":"web-blue"}}' \
  http://localhost:8080/v1/projects/<project-id>/templates/${TEMPLATE_ID}/deploy | jq
```

`/render` merges defaults with overrides and validates the payload against the
stored JSON Schema, making it safe to hand off to `/deploy` or CI pipelines.

## Store Git and registry credentials securely

The new credential vault exposes `/v1/credentials/*` endpoints so automation can
store Git tokens, SSH keys, and registry passwords without embedding them in app
payloads. Secrets are encrypted with AES-256 using `KCFG_ENCRYPTION_KEY` and are
only returned when explicitly fetched.

Create a Git token scoped to a user:

```bash
curl -s $AUTH_H -H 'Content-Type: application/json' \
  -d '{
        "name": "git-main",
        "scope": {"type": "user", "id": "<user-id>"},
        "auth": {"type": "token", "token": "ghp_example"}
      }' \
  http://localhost:8080/v1/credentials/git | jq
```

List credentials for a project and fetch the decrypted secret when needed:

```bash
curl -s $AUTH_H "http://localhost:8080/v1/credentials/git?projectId=<project-id>" | jq

curl -s $AUTH_H http://localhost:8080/v1/credentials/git/<credential-id> | jq
```

Registry passwords follow the same pattern:

```bash
curl -s $AUTH_H -H 'Content-Type: application/json' \
  -d '{
        "name": "dockerhub",
        "registry": "https://index.docker.io/v1/",
        "scope": {"type": "project", "id": "<project-id>"},
        "auth": {"type": "basic", "username": "repo", "password": "s3cret"}
      }' \
  http://localhost:8080/v1/credentials/registries | jq
```

Deleting `/v1/credentials/git/{id}` or `/v1/credentials/registries/{id}` removes
the encrypted secret and frees the unique name within the user or project scope.

## Manage cluster inventory and health

Cluster registration accepts ownership metadata so operators can filter clusters
by team, environment, or region. The `/v1/clusters` endpoints expose this data
alongside live health probes.

```bash
# Register a cluster with metadata
B64=$(base64 -w0 < kubeconfig)
curl -s $AUTH_H -H 'Content-Type: application/json' \
  -d "$(jq -n --arg name 'talos-stage' --arg b64 "$B64" '{name:$name,kubeconfig_b64:$b64,"owner":"platform","environment":"staging","region":"eu-west","apiServer":"https://10.0.0.10:6443","tags":["platform","staging"]}')" \
  http://localhost:8080/v1/clusters | jq

# Update metadata without touching the stored kubeconfig
curl -s $AUTH_H -X PATCH -H 'Content-Type: application/json' \
  -d '{"environment":"production","tags":["platform","prod"]}' \
  http://localhost:8080/v1/clusters/<cluster-id> | jq

# List clusters with last-seen health snapshots
curl -s $AUTH_H http://localhost:8080/v1/clusters | jq

# Retrieve the most recent health checks (newest first)
curl -s $AUTH_H 'http://localhost:8080/v1/clusters/<cluster-id>/status?limit=5' | jq
```

Each status entry includes the probe timestamp, success flag, message, API
server version, and structured details describing the reconciliation stage. Use
`GET /v1/clusters/{id}` for detailed metadata and `GET /v1/clusters/{id}/health`
to trigger an on-demand check.

## Audit app release history

Retrieve the immutable deployment history for an app with
`GET /v1/projects/{id}/apps/{appId}/releases`. The endpoint returns the spec
digest, rendered object summaries, load balancer counts, Helm inputs, and any
warnings captured during planning.

```bash
curl -s $AUTH_H "http://localhost:8080/v1/projects/<project-id>/apps/<app-id>/releases?limit=10" | jq
```

The response includes `releases[]` ordered newest-first plus `nextCursor` for
pagination. Pass `cursor=<nextCursor>` (the release ID from the previous page)
with the same `projectId` and `appId` to fetch older releases while preserving
ordering.
