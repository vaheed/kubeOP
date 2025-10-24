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

## Service exposure policy

Ingress-facing workloads must align with the tenant's service exposure policy. kubeOP reads the
`servicePolicy` settings from each tenant's `NetworkPolicyProfile` to determine which service types are
permitted. By default only `ClusterIP` services are allowed. To expose a service externally:

1. Create or update the tenant's `NetworkPolicyProfile` with an approved `servicePolicy` that sets
   `allowLoadBalancer: true` or lists specific `externalIPs`.
2. Label the profile with the same tenant metadata as other namespace-scoped resources.
3. Reconcile the profile before applying an `App` or `AppRelease` that creates a `Service`.

Attempts to deploy a `Service` that violates this policy are rejected by the validating webhook with a
message indicating the forbidden service type or external IP. Review the profile's history before
expanding the allowlist.

## Tenant RBAC templates

Each tenant automatically receives scoped access via reconciled RoleBindings. The operator provisions
three ClusterRoles:

- `tenant-owner` – full administrative access within the tenant's namespaces.
- `tenant-developer` – CRUD access to application resources but restricted from managing RBAC bindings.
- `tenant-viewer` – read-only visibility into the tenant's workloads.

When a `Tenant` object is created or updated, the controller binds these ClusterRoles to identities
listed in the tenant spec. The controller refuses to bind subjects that reference namespaces outside the
tenant boundary, preventing cross-tenant privilege escalation. Platform teams should audit the generated
RoleBindings by running `kubectl get rolebinding -n <tenant-namespace>` and confirming the subjects match
their onboarding roster.

## Operational checklist

- Review webhook denial messages in the API server audit logs to confirm tenants are constrained to their namespaces.
- Teach application teams to include the required labels in Helm charts and Kustomize overlays.
- Regularly run `kubectl get` with `--show-labels` to validate label drift:

```bash
kubectl get app -n team-a --show-labels
```

- Document the root-execution justification process for compliance reviews.

For broader hardening guidance, see [docs/SECURITY.md](../SECURITY.md).
