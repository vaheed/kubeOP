# Tenant isolation guide

kubeOP enforces namespace-level multi-tenancy through a set of admission webhooks and RBAC conventions. This document explains
mandatory labels, failure modes, and how platform operators can validate tenant-scoped access with `kubectl`.

## Required labels

All namespace-scoped custom resources managed by kubeOP must carry the following labels to establish tenant, project, and
application ownership:

| Label | Purpose |
| --- | --- |
| `paas.kubeop.io/tenant` | Identifies the owning tenant. Used for cross-object validation and RBAC scoping. |
| `paas.kubeop.io/project` | Maps the object back to a Project (namespace) record. |
| `paas.kubeop.io/app` | Associates the object with an application. Required even for supporting resources so audit trails remain intact. |

Resources missing any of these labels are rejected by the validating webhook with a clear error message. Ensure your GitOps or
CLI workflows populate the labels before applying manifests.

## Admission failure examples

### Missing tenant label

Applying an `App` without the tenant label fails validation:

```bash
kubectl apply -n team-a -f - <<'YAML'
apiVersion: paas.kubeop.io/v1alpha1
kind: App
metadata:
  name: demo
  labels:
    paas.kubeop.io/project: web
    paas.kubeop.io/app: demo
spec:
  type: raw
YAML
```

Output:

```
The App "demo" is invalid: metadata.labels.paas.kubeop.io/tenant: Required value: tenant label is required for tenant isolation
```

Add `paas.kubeop.io/tenant` (and ensure it matches the owning tenant) to resolve the rejection.

### Cross-tenant reference

Referencing a `SecretRef` from another tenant is denied to prevent data leaks:

```bash
kubectl apply -n team-a -f - <<'YAML'
apiVersion: paas.kubeop.io/v1alpha1
kind: App
metadata:
  name: demo
  labels:
    paas.kubeop.io/tenant: tenant-a
    paas.kubeop.io/project: web
    paas.kubeop.io/app: demo
spec:
  type: raw
  secretsRefs:
    - shared-secret
YAML
```

```
The App "demo" is invalid: spec.secretsRefs[0]: Forbidden: SecretRef "shared-secret" belongs to tenant "tenant-b"
```

Ensure supporting resources live in the same namespace and carry the same tenant label before wiring them to an `App`.

### Privileged pod configurations

Jobs that request host-level access or root execution without an approved justification are blocked. The webhook inspects
`hostNetwork`, `hostPID`, `hostIPC`, hostPath volumes, and container capabilities.

```bash
kubectl apply -n team-a -f - <<'YAML'
apiVersion: paas.kubeop.io/v1alpha1
kind: Job
metadata:
  name: nightly-maintenance
  labels:
    paas.kubeop.io/tenant: tenant-a
    paas.kubeop.io/project: ops
    paas.kubeop.io/app: maintenance
spec:
  template:
    spec:
      hostNetwork: true
      containers:
        - name: runner
          image: busybox
          command: ["sh", "-c", "echo ok"]
YAML
```

```
The Job "nightly-maintenance" is invalid: spec.template.spec.hostNetwork: Forbidden: hostNetwork is not allowed for tenant workloads
```

To run a pod as root, set the `paas.kubeop.io/run-as-root-justification` annotation explaining the exception and ensure the pod
security context is as narrow as possible.

## Operational checklist

- Review webhook denial messages in the API server audit logs to confirm tenants are constrained to their namespaces.
- Teach application teams to include the required labels in Helm charts and Kustomize overlays.
- Regularly run `kubectl get` with `--show-labels` to validate label drift:

```bash
kubectl get app -n team-a --show-labels
```

- Document the root-execution justification process for compliance reviews.

For broader hardening guidance, see [docs/SECURITY.md](../SECURITY.md).
