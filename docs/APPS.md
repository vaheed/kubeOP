# Application workflows

kubeOP manages application lifecycles from a single API. This page highlights the most common flows and the new validation workflow.

## Dry-run before deploy

1. Collect the project ID from `/v1/projects` or `/v1/users/bootstrap`.
2. POST the app spec to `/v1/apps/validate`.
3. Inspect the JSON output for:
   - `kubeName` – deterministic resource name that will be used for Deployment/Service/Ingress.
   - `replicas` and `resources` – effective values after applying flavors or overrides.
   - `loadBalancers` – ensures the request stays within quota limits.
   - `renderedObjects` – summary of Kubernetes objects produced by raw manifests or Helm charts.
4. Fix any issues (missing names, quota errors, Helm rendering failures) before calling `/v1/projects/{id}/apps`.

## Deploy after validation

Send the same payload to `/v1/projects/{id}/apps` to create the resources. kubeOP persists the deployment metadata, applies labels, and emits audit events so you can track history via `/v1/projects/{id}/events`.

## Accelerate delivery with templates

Templates let platform teams publish curated blueprints that application owners
can render or deploy in one step.

```bash
# Register a reusable blueprint
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

# Preview without touching the cluster
curl -s $AUTH_H -H 'Content-Type: application/json' \
  -d '{"values":{"name":"web-blue"}}' \
  http://localhost:8080/v1/templates/${TEMPLATE_ID}/render | jq

# Deploy in one call
curl -s $AUTH_H -H 'Content-Type: application/json' \
  -d '{"values":{"name":"web-blue"}}' \
  http://localhost:8080/v1/projects/<project-id>/templates/${TEMPLATE_ID}/deploy | jq
```

Every render enforces the stored JSON Schema and merges defaults with overrides,
keeping deployments consistent across environments.

## Review release history and audits

Each successful deployment records a release snapshot with the spec digest,
rendered object summaries, Helm values, load balancer usage, and warnings.

```bash
curl -s $AUTH_H \
  "http://localhost:8080/v1/projects/<project-id>/apps/<app-id>/releases?limit=5" | jq
```

The response includes `releases[]` entries ordered newest-first plus a
`nextCursor` for pagination. Use `cursor=<nextCursor>` (a release ID) with the
same `projectId`/`appId` combination to fetch older entries.
Each release captures `spec.source` (image/helm/manifests), the computed
`renderDigest`, load balancer counts, and any warnings emitted during planning,
allowing teams to audit rollouts and identify the exact manifest changes that
reached Kubernetes.

## Secure delivery credentials

Before configuring Git- or registry-backed deliveries, store tokens or passwords
in the credential vault so app specs never embed secrets. The credential APIs
encrypt payloads using `KCFG_ENCRYPTION_KEY` and scope them to either a user or a
project.

```bash
curl -s $AUTH_H -H 'Content-Type: application/json' \
  -d '{
        "name": "git-main",
        "scope": {"type": "user", "id": "<user-id>"},
        "auth": {"type": "token", "token": "ghp_example"}
      }' \
  http://localhost:8080/v1/credentials/git | jq

curl -s $AUTH_H -H 'Content-Type: application/json' \
  -d '{
        "name": "registry",
        "registry": "https://index.docker.io/v1/",
        "scope": {"type": "project", "id": "<project-id>"},
        "auth": {"type": "basic", "username": "repo", "password": "s3cret"}
      }' \
  http://localhost:8080/v1/credentials/registries | jq
```

Use the returned credential IDs in delivery specs instead of raw secrets. See
[`docs/TUTORIALS/credential-stores.md`](./TUTORIALS/credential-stores.md) for a
full copy-paste walkthrough.

## Troubleshooting tips

- Validation errors return HTTP `400` with an `error` message. Common cases include unknown flavors, exceeding load balancer quotas, or malformed YAML/Helm manifests.
- Warnings appear in the `warnings[]` array and do not block deployment, but they call out missing names or namespaces that kubeOP will auto-fill.
- When validation succeeds but deployment later fails, check the project event feed or the per-app log file under `${LOGS_ROOT}/projects/<project_id>/apps/<app_id>/deploy.log`.
