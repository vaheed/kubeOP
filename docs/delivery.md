---
outline: deep
---

# Delivery Workflows

The kubeOP operator reconciles Apps into Kubernetes workloads using server-side apply, revision hashing, and hook orchestration. The diagram below illustrates the reconciliation pipeline.

![Delivery Controller](./diagrams/delivery-controller.svg)

## Delivery kinds

| Kind | Description | Key fields |
| --- | --- | --- |
| `Image` | Renders Deployments, Services, optional Ingress/DNS for a container image. | `spec.delivery.image.ref`, `replicas`, `ports`, `resources`, `rollout`, `probes`. |
| `Git` | Applies a list of already-rendered manifests fetched from Git (rendering happens upstream). | `spec.delivery.git.manifests[]` |
| `Helm` | Applies Helm-rendered manifests supplied in the App spec. | `spec.delivery.helm.manifests[]` |
| `Raw` | Applies arbitrary YAML manifest lists. | `spec.delivery.raw.manifests[]` |

All delivery modes share common behaviors:

- Namespace enforcement: Apps must live in their project namespace.
- Ownership labels: `app.kubeop.io/{cluster-id,tenant-id,project-id,app-id}` are injected.
- Revision history: Each successful rollout stores a ConfigMap (`kubeop-revision-<app>`) containing the spec hash and metadata for audit/rollback.
- Hooks: `spec.hooks.pre[]` and `spec.hooks.post[]` run as Jobs once per revision. Reconciliation pauses on hook failure.

## Sample App spec

```yaml
apiVersion: platform.kubeop.io/v1alpha1
kind: App
metadata:
  name: payments
spec:
  projectRef: web
  namespace: kubeop-web
  delivery:
    kind: Image
    image:
      ref: ghcr.io/acme/payments:2.3.1
      replicas: 3
      rollout:
        strategy: RollingUpdate
        maxSurge: 1
        maxUnavailable: 1
      resources:
        preset: medium
        requests:
          cpu: "500m"
          memory: 512Mi
      probes:
        preset: http
        readiness:
          path: /healthz
          port: 8080
  hooks:
    pre:
      - name: migrate
        image: ghcr.io/acme/payments-migrate:2.3.1
        command: ["/bin/sh", "-c", "./migrate.sh"]
```

## Rollout & rollback procedures

1. Apply the App manifest (manager API or `kubectl apply`).
2. Observe revision status:
   ```bash
   kubectl -n kubeop-web get app payments -o json | jq '.status.conditions'
   kubectl -n kubeop-web get configmap kubeop-revision-payments -o yaml
   ```
3. Monitor Deployment rollout:
   ```bash
   kubectl -n kubeop-web rollout status deploy/payments --timeout=180s
   ```
4. Trigger rollback by re-applying a previous spec hash or using Deployment history:
   ```bash
   kubectl -n kubeop-web rollout undo deploy/payments --to-revision=2
   ```

## Registry credentials & allowlist

- Registry hosts must match `KUBEOP_IMAGE_ALLOWLIST`. Endpoints validate and return `400` when a disallowed host is used.
- Operator mirrors secrets into project namespaces and sets owner references to ensure garbage collection.

## DNS & TLS integration

- Apps may request DNS/TLS by creating `DNSRecord` and `Certificate` CRs:
  ```bash
  kubectl apply -f - <<'YAML'
  apiVersion: platform.kubeop.io/v1alpha1
  kind: DNSRecord
  metadata:
    name: web-external
  spec:
    host: web.prod.example.com
    target: web.kubeop-web.svc.cluster.local
  ---
  apiVersion: platform.kubeop.io/v1alpha1
  kind: Certificate
  metadata:
    name: web-external
  spec:
    host: web.prod.example.com
    dnsRecordRef: web-external
  YAML
  ```
- The operator reconciles these resources and updates status conditions for downstream automation.

## Hooks and analytics

- Hooks emit Events with success/failure reasons (`kubectl -n kubeop-web get events`).
- Manager provides `/v1/analytics/summary` to understand delivery mix (image vs. git vs. helm) and registry usage.
