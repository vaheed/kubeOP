# Tenants, projects, and apps

This guide walks through provisioning tenants, managing project lifecycles, and deploying applications via kubeOP.

## Bootstrap a tenant user

Use `/v1/users/bootstrap` to create or reuse a user, provision the namespace, and mint a kubeconfig binding.

```bash
curl -s -X POST \
  -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{
    "name": "Alice",
    "email": "alice@example.com",
    "clusterId": "<cluster-id>"
  }' \
  http://localhost:8080/v1/users/bootstrap | jq
```

Response fields:

- `user` – object containing `id`, `name`, `email`, and timestamps.
- `namespace` – Kubernetes namespace created for the user (`user-<uuid>` by default).
- `kubeconfig_b64` – base64-encoded namespace-scoped kubeconfig with service account credentials.

The namespace receives managed `ResourceQuota`, `LimitRange`, and labels from `internal/service/quota.go` and `internal/service/labels.go`.

## Create a project

Projects scope applications, quotas, and configuration to a namespace.

```bash
curl -s -X POST \
  -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{
    "userId": "<user-id>",
    "clusterId": "<cluster-id>",
    "name": "payments-api"
  }' \
  http://localhost:8080/v1/projects | jq
```

- Projects inherit the user namespace when `PROJECTS_IN_USER_NAMESPACE=true` (default). Override by setting the flag to `false`.
- `quotaOverrides` accepts a map to tweak ResourceQuota values at creation time (e.g. `{ "limits.cpu": "6" }`).

## Inspect and list projects

- `GET /v1/projects?limit=50&offset=0` – paginated list of all projects.
- `GET /v1/users/{id}/projects` – projects owned by a specific user.
- `GET /v1/projects/{id}` – project metadata (`namespace`, `cluster_id`, `suspended`, `created_at`).

## Suspend or delete a project

- `POST /v1/projects/{id}/suspend` – scales deployments to zero, deletes Services/Ingresses, and marks the project suspended.
- `POST /v1/projects/{id}/unsuspend` – reapplies workloads using stored specs.
- `DELETE /v1/projects/{id}` – removes workloads, secrets/configs managed by kubeOP, and deletes the record.

## Deploy an application

`POST /v1/projects/{id}/apps` accepts multiple source types. A minimal container image payload:

```bash
curl -s -X POST \
  -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{
    "name": "web",
    "image": "nginx:1.27",
    "ports": [
      {"containerPort": 80, "servicePort": 80, "protocol": "TCP", "serviceType": "ClusterIP"}
    ]
  }' \
  http://localhost:8080/v1/projects/<project-id>/apps | jq
```

Other options:

- `helm`: object with chart archive (`chart`), `values`, and optional `repo` metadata.
- `manifests`: array of raw Kubernetes manifests (strings). kubeOP server-side applies them.
- `flavor`: reference to built-in sizing presets (`internal/service/apps.go` defines `f1-small`, `f2-medium`, `f3-large`).

## Publish template blueprints

Platform teams can pre-build JSON Schema–guarded templates and let application
teams instantiate them with optional overrides.

```bash
# Register a template once
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

# Render for review or CI pipelines
curl -s $AUTH_H -H 'Content-Type: application/json' \
  -d '{"values":{"name":"web-blue"}}' \
  http://localhost:8080/v1/templates/${TEMPLATE_ID}/render | jq

# Deploy the rendered payload to a project
curl -s $AUTH_H -H 'Content-Type: application/json' \
  -d '{"values":{"name":"web-blue"}}' \
  http://localhost:8080/v1/projects/<project-id>/templates/${TEMPLATE_ID}/deploy | jq
```

Templates enforce the stored schema on every render, guaranteeing consistent
defaults while still allowing safe overrides.

## Application status and management

- `GET /v1/projects/{id}/apps` – returns an array of `AppStatus` (desired/ready/available replicas, service summary, ingress hosts, pod readiness).
- `GET /v1/projects/{id}/apps/{appId}` – detailed status for one app. kubeOP queries live cluster state via controller-runtime clients.
- `PATCH /v1/projects/{id}/apps/{appId}/scale` with `{ "replicas": 3 }` – updates Deployment replicas.
- `PATCH /v1/projects/{id}/apps/{appId}/image` with `{ "image": "repo/image:tag" }` – updates container image and triggers rollout.
- `POST /v1/projects/{id}/apps/{appId}/rollout/restart` – annotates Deployment to force a restart.
- `DELETE /v1/projects/{id}/apps/{appId}` – removes Deployment, Service, Ingress, Jobs, PVCs created for the app.

## Configuration attachments

- ConfigMaps: `POST /v1/projects/{id}/configs` with `{ "name": "app-config", "data": { "KEY": "value" } }`.
- Secrets: `POST /v1/projects/{id}/secrets` with `{ "name": "db-creds", "stringData": { "USER": "alice" } }`.
- Attach to apps:
  - `POST /v1/projects/{id}/apps/{appId}/configs/attach` – body `{ "name": "app-config", "keys": ["KEY"], "prefix": "APP_" }`.
  - `POST /v1/projects/{id}/apps/{appId}/secrets/attach` – body `{ "name": "db-creds", "prefix": "DB_" }`.
  - Detach endpoints mirror attach with `.../detach` paths.

kubeOP patches Pod specs using server-side apply so mounts/env vars remain consistent across rollouts.

## Logs and events

- `GET /v1/projects/{id}/apps/{appId}/logs?tailLines=200&follow=true` – streams container logs via the Kubernetes API with optional `follow=false`.
- `GET /v1/projects/{id}/logs?tail=500` – tails the disk-backed project log file (max 5000 lines).
- `GET /v1/projects/{id}/events?kind=DEPLOY&severity=INFO&limit=20` – returns paginated events with cursoring. Filters include `kind`, `severity`, `actor`, `since` (RFC3339), `limit`, `cursor`, and `grep`/`search`.
- `POST /v1/projects/{id}/events` – create custom timeline entries (`{ "kind": "DEPLOY", "severity": "INFO", "message": "rollout complete", "appId": "...", "meta": {"sha": "..."} }`).

## Cleaning up

- Delete apps before deleting projects to ensure all workloads are removed cleanly.
- Rotate kubeconfigs via `POST /v1/kubeconfigs/rotate` to issue fresh credentials when service accounts are compromised or rotated.
- Use `DELETE /v1/kubeconfigs/{id}` to revoke bindings for departing users.
