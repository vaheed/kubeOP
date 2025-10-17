# Deploying images, Helm charts, and manifests

`POST /v1/projects/{id}/apps` accepts three mutually exclusive sources: container images, remote Helm charts, or raw manifests. This guide explains payload structure, validation, and webhook integration.

## Container images

Minimal payload:

```bash
curl -s -X POST \
  -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{
    "name": "web",
    "image": "ghcr.io/example/web:1.4.2",
    "ports": [
      {"containerPort": 8080, "servicePort": 80, "protocol": "TCP", "serviceType": "ClusterIP"}
    ],
    "env": {"APP_MODE": "prod"},
    "secrets": ["db-creds"],
    "configs": ["app-config"],
    "flavor": "f2-medium"
  }' \
  http://localhost:8080/v1/projects/<project-id>/apps | jq
```

Behaviour:

- `flavor` applies CPU/memory defaults defined in `internal/service/apps.go` (`f1-small`, `f2-medium`, `f3-large`).
- Ports create a Service. `serviceType: "LoadBalancer"` counts against `MAX_LOADBALANCERS_PER_PROJECT`.
- `domain` (optional) feeds ingress host calculation. If empty, kubeOP derives a host using `PAAS_DOMAIN` and the slugified app name plus a deterministic short hash (for example, `web-02-f7f88c5b4-4ldbq`).
- Secrets/configs listed in the payload must exist. Use attach endpoints to add them later.

## Remote Helm charts

kubeOP downloads `.tgz` charts over HTTPS, renders manifests with provided values, and applies them to the project namespace.

```bash
curl -s -X POST \
  -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{
    "name": "grafana",
    "helm": {
      "chart": "https://charts.bitnami.com/bitnami/grafana-11.0.0.tgz",
      "values": {
        "adminPassword": "changeme",
        "service": {"type": "ClusterIP"}
      }
    }
  }' \
  http://localhost:8080/v1/projects/<project-id>/apps | jq
```

Validation rules:

- Only HTTPS charts are allowed. `renderHelmChartFromURL` resolves the host, validates IPs against RFC1918/loopback restrictions, and rejects plain HTTP.
- The chart must end in `.tgz`. kubeOP renders manifests using Helm’s engine and applies them via server-side apply.
- Provide explicit `values` to override defaults; omit to accept chart defaults.

## Raw manifests

Use `manifests` to apply YAML directly. kubeOP labels every resource with `kubeop.app-id` so later fetches map workloads to apps.

```bash
curl -s -X POST \
  -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{
    "name": "custom",
    "manifests": [
      "apiVersion: apps/v1\nkind: Deployment\nmetadata:\n  name: custom\nspec:\n  selector:\n    matchLabels:\n      app: custom\n  template:\n    metadata:\n      labels:\n        app: custom\n    spec:\n      containers:\n        - name: custom\n          image: ghcr.io/example/custom:1.0.0\n          ports:\n            - containerPort: 9000"
    ]
  }' \
  http://localhost:8080/v1/projects/<project-id>/apps | jq
```

Tips:

- Multi-document YAML is supported; kubeOP splits on `---`.
- Namespaced resources are forced into the project namespace unless the manifest sets a different namespace (rejected to avoid drift).

## Git webhooks and rollouts

Set `repo` and `webhookSecret` when creating an app to enable CI-triggered rollouts.

```json
{
  "name": "web",
  "image": "ghcr.io/example/web:main",
  "repo": "example/web",
  "webhookSecret": "sha256-key"
}
```

- `/v1/webhooks/git` expects JSON payloads containing `repository.full_name` (or `clone_url`) and `ref`. kubeOP finds apps by `repo` and patches their Deployment annotation `kubeop.io/redeploy=<timestamp>`.
- Signature verification prioritises per-app `webhookSecret`; falls back to global `GIT_WEBHOOK_SECRET` if set. Secrets are compared using `X-Hub-Signature-256` HMACs.
- CI pipelines should call `/v1/webhooks/git` after pushing manifests or images to trigger rollout restarts.

## Rollback and cleanup

- Delete an app using `DELETE /v1/projects/{id}/apps/{appId}`. kubeOP removes Deployments, Services, Ingresses, Secrets labeled with `kubeop.app-id`, and DNS records created for ingress hosts.
- Use project events (`POST /v1/projects/{id}/events`) to annotate deployments with commit SHAs or changelog links.
- When switching deployment strategy (e.g. from Helm to raw manifests), delete the existing app and create a new one with the desired source.
