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

### Validate Helm charts from OCI registries

`helm.oci` sources are validated with the same endpoint. Provide the registry reference and optional credential ID to confirm the
rendered manifests and quota impact before deployment.

```bash
curl -s $AUTH_H -H 'Content-Type: application/json' \
  -d '{
        "projectId": "<project-id>",
        "name": "grafana",
        "helm": {
          "oci": {
            "ref": "oci://ghcr.io/example/charts/grafana:11.0.0",
            "registryCredentialId": "<registry-credential-id>"
          }
        }
      }' \
  http://localhost:8080/v1/apps/validate | jq
```

The response echoes `source: "helm"`, the OCI reference in `helmChart`, and the objects produced by the render. When
`registryCredentialId` is present, kubeOP verifies that the credential scope matches the project (or the project owner) before
logging into the registry during validation and deploy. Set `"insecure": true` only for trusted HTTP registries during
development.

### Deploy OCI manifest bundles

When manifests are published as OCI artifacts (for example, `oras`-pushed tarballs),
use the `ociBundle` source. kubeOP fetches the referenced artifact, enforces the
same outbound network safeguards as Helm OCI charts, and extracts Kubernetes YAML
documents before applying them to the project namespace.

```bash
curl -s $AUTH_H -H 'Content-Type: application/json' \
  -d '{
        "projectId": "<project-id>",
        "name": "bundle-app",
        "ociBundle": {
          "ref": "oci://ghcr.io/example/bundles/web:1.2.0",
          "credentialId": "<registry-credential-id>"
        }
      }' \
  http://localhost:8080/v1/apps/validate | jq
```

The validation output reports `source: "ociBundle"` alongside `ociBundleRef` and
`ociBundleDigest` so you can confirm the exact artifact that will be deployed.
During validation and deploy, kubeOP rejects archives with unsafe paths, enforces a
size limit (8 MiB by default), and ensures the registry host resolves to global
addresses. Set `"insecure": true` only for trusted development registries served
over plain HTTP.

`ociBundle` fields:

| Field | Description |
| --- | --- |
| `ociBundle.ref` | OCI reference (`oci://host/repo/artifact:tag` or `oci://host/repo@sha256:...`). |
| `ociBundle.credentialId` | Optional registry credential created via `/v1/credentials/registries`. |
| `ociBundle.insecure` | Allow HTTP registries during development (disables TLS). |

### Deploy from Git repositories

Applications can source manifests directly from Git. Supply a `git` object alongside the app name and kubeOP will clone the
repository, optionally authenticate using a stored Git credential, and render either raw YAML files or a Kustomize overlay.

```bash
curl -s $AUTH_H -H 'Content-Type: application/json' \
  -d '{
        "name": "git-app",
        "git": {
          "url": "https://github.com/example/platform-configs.git",
          "ref": "refs/heads/main",
          "path": "apps/web/overlays/prod",
          "mode": "kustomize"
        }
      }' \
  http://localhost:8080/v1/projects/<project-id>/apps | jq
```

Git payload fields:

| Field | Description |
| --- | --- |
| `git.url` | Repository clone URL (`https://`, `ssh://`, or `git@` syntax). |
| `git.ref` | Branch, tag, or full ref (defaults to `refs/heads/main`). |
| `git.path` | Optional directory within the repo. Files outside the repo are rejected. |
| `git.mode` | `manifests` (default) or `kustomize`. |
| `git.credentialId` | Optional credential created via `/v1/credentials/git`. |
| `git.insecureSkipTLS` | Allow TLS verification to be skipped for trusted internal servers. |

The validation response now includes Git metadata:

```json
{
  "source": "git:kustomize",
  "gitRepo": "https://github.com/example/platform-configs.git",
  "gitRef": "refs/heads/main",
  "gitCommit": "1f3d8c0c6af5e2d8a0217b6bb61efef20e1f4c45",
  "gitPath": "apps/web/overlays/prod",
  "gitMode": "kustomize",
  "renderedObjects": [
    {"kind": "Deployment", "name": "git-app-1f3d8c0", "namespace": "tenant-ns"}
  ]
}
```

Git support honours the existing release history workflow: every deploy persists the commit hash and manifest digest so you can
audit which revision shipped. For local testing with `file://` repositories (for example, when using a temp repo in automated
tests) set `ALLOW_GIT_FILE_PROTOCOL=true`; keep it disabled in shared environments.

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

## Pause mutating operations during maintenance

When the platform is undergoing upgrades, toggle global maintenance mode to block
mutating APIs (cluster registration, project creation, app deploy/scale/image update,
template deploys, etc.). The state is persisted in PostgreSQL so every API replica
enforces the same guard.

```bash
# Inspect current state
curl -s $AUTH_H http://localhost:8080/v1/admin/maintenance | jq

# Enable maintenance with a message
curl -s $AUTH_H -H 'Content-Type: application/json' \
  -d '{"enabled":true,"message":"Upgrading control plane nodes"}' \
  http://localhost:8080/v1/admin/maintenance | jq

# Disable maintenance again
curl -s $AUTH_H -H 'Content-Type: application/json' \
  -d '{"enabled":false}' \
  http://localhost:8080/v1/admin/maintenance | jq
```

While enabled, attempts to deploy, scale, or delete apps (and other mutating calls)
receive HTTP `503` with an error such as `maintenance mode enabled: Upgrading control
plane nodes`, allowing automation to back off until the upgrade completes.

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
