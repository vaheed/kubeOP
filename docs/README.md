> **What this page explains**: kubeOP's purpose, the jobs it solves, and the quickstart.
> **Who it's for**: Engineers adopting kubeOP as the fleet control plane.
> **Why it matters**: Helps you understand the core features before going deeper.

# Overview

kubeOP is an out-of-cluster control plane that keeps Kubernetes tenancy, app delivery, and observability tidy. Everything runs from a Go API binary that connects to PostgreSQL and speaks to your target clusters using stored kubeconfigs.

## Key features

- **Multi-cluster aware**: register clusters with a base64 kubeconfig and target workloads anywhere.
- **Tenant-first design**: bootstrap users, namespaces, quotas, and kubeconfigs in one request.
- **Delivery pipeline**: deploy manifests, Helm charts, or OCI bundles with identical rollout status.
- **Observability hooks**: structured logs, event fan-out, and Prometheus `/metrics` out of the box.

## Quickstart

```bash
git clone https://github.com/vaheed/kubeOP.git
cd kubeOP
LOGS_ROOT=$PWD/logs docker compose up -d
curl http://localhost:8080/healthz
```

## Next steps

### Configure access
Export an admin JWT signed by `ADMIN_JWT_SECRET` with `{ "role": "admin" }` to unlock the admin APIs. Use `POST /v1/users/bootstrap` to create the first tenant namespace.

### Register a cluster
Use the `/v1/clusters` endpoint with a base64 encoded kubeconfig (`kubeconfig_b64`). The service stores it encrypted and schedules health probes.

```bash
B64=$(base64 -w0 < ~/.kube/config)
curl -s -X POST "http://localhost:8080/v1/clusters" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d "{\"name\":\"dev\",\"kubeconfig_b64\":\"$B64\"}"
```

### Deploy an application
Create an app manifest that references either a Helm chart or OCI image. The API tracks rollout status and exposes logs for each revision.

```yaml
apiVersion: platform.kubeop.dev/v1alpha1
kind: App
metadata:
  name: web
spec:
  projectId: 84d4b1b4-7c9b-4f23-8b2f-5e6ab91a4a1a
  source:
    type: helm
    repo: https://charts.bitnami.com/bitnami
    chart: nginx
    version: 15.7.0
  values:
    service:
      type: ClusterIP
```

