> **What this page explains**: kubeOP's tenant, project, and app hierarchy.
> **Who it's for**: Platform teams planning isolation and quotas.
> **Why it matters**: Shows how to map business units into Kubernetes namespaces safely.

# Tenancy model

kubeOP treats a tenant as the root object for users, projects, and apps. Each tenant owns namespaces across clusters but stays isolated through RBAC templates and resource quotas.

## Tenant objects

Tenants bundle human metadata with automation toggles. The bootstrap endpoint creates tenants and associated namespaces.

```json
{
  "name": "payment",
  "displayName": "Payments Team",
  "owners": ["alice@example.com", "ops@example.com"],
  "labels": {
    "cost-center": "fin-001",
    "env": "prod"
  }
}
```

### Namespace layout
Each tenant receives a root namespace per cluster. Projects default to that namespace when `PROJECTS_IN_USER_NAMESPACE=true`.

## Projects

### Purpose
Projects represent deployable surfaces for apps. They inherit quotas and network policies from the tenant but can override fine-grained settings.

```yaml
apiVersion: platform.kubeop.dev/v1alpha1
kind: Project
metadata:
  name: payments-api
spec:
  tenantId: 7fc81d8c-4b58-4ccd-91dd-8c91a60a3de5
  clusterId: a310f8a4-21d7-4a4e-9bf8-09db1a189481
  namespace: payments-prod
  limits:
    cpu: "20"
    memory: 40Gi
  networkPolicy:
    ingress:
      - fromTenants: ["observability"]
```

## Applications

### Deployment units
Apps reference a project and define the desired workload. They can point to Helm charts, OCI artifacts, or raw manifests.

```json
{
  "projectId": "7fc81d8c-4b58-4ccd-91dd-8c91a60a3de5",
  "name": "payments-worker",
  "deployment": {
    "type": "oci",
    "image": "ghcr.io/vaheed/payments-worker:1.4.2",
    "env": [
      { "name": "QUEUE", "value": "settlements" },
      { "name": "LOG_LEVEL", "value": "info" }
    ]
  }
}
```

### Lifecycle
Revisions capture spec changes, rollout status, and log offsets. Use `/v1/apps/{id}/revisions` to inspect history and `/v1/apps/{id}/rollback` to revert.

