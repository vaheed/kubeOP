# Custom Resource Definitions

Schemas originate from the YAML under [`deploy/k8s/crds`](https://github.com/vaheed/kubeOP/tree/main/deploy/k8s/crds) and the Go structs in [`internal/operator/apis/paas/v1alpha1`](https://github.com/vaheed/kubeOP/blob/main/internal/operator/apis/paas/v1alpha1/types.go#L12-L118).

## Tenant (`tenants.paas.kubeop.io`)

- Scope: Cluster
- Spec fields (from [`paas.kubeop.io_tenants.yaml`](https://github.com/vaheed/kubeOP/blob/main/deploy/k8s/crds/paas.kubeop.io_tenants.yaml#L19-L36)):
  | Field | Type | Required | Validation |
  |-------|------|----------|------------|
  | `spec.name` | string | Yes | CEL ensures presence and non-empty (`has(self.name) && size(self.name) > 0`).
- Status fields: `status.ready` (bool), `status.conditions[]` with `type`, `status`, `reason`, `message`, `lastTransitionTime`.
- Controller behavior: [`TenantReconciler`](https://github.com/vaheed/kubeOP/blob/main/internal/operator/controllers/controllers.go#L29-L63) marks `status.ready=true` and adds a `Ready` condition with reason `Bootstrapped`.

Minimal manifest:
```yaml
apiVersion: paas.kubeop.io/v1alpha1
kind: Tenant
metadata:
  name: example
spec:
  name: example
```

Realistic manifest (labels, annotations for auditing):
```yaml
apiVersion: paas.kubeop.io/v1alpha1
kind: Tenant
metadata:
  name: example
  labels:
    app.kubeop.io/environment: dev
spec:
  name: Example Corp
```

Status example:
```yaml
status:
  ready: true
  conditions:
    - type: Ready
      status: "True"
      reason: Bootstrapped
      message: Tenant initialized
```

## Project (`projects.paas.kubeop.io`)

- Scope: Cluster
- Spec fields (from [`paas.kubeop.io_projects.yaml`](https://github.com/vaheed/kubeOP/blob/main/deploy/k8s/crds/paas.kubeop.io_projects.yaml#L19-L39)):
  | Field | Type | Required | Validation |
  |-------|------|----------|------------|
  | `spec.tenantRef` | string | Yes | Must be non-empty (`has(self.tenantRef)` and `size > 0`).
  | `spec.name` | string | Yes | Must be non-empty (`has(self.name)` and `size > 0`).
- Status fields: `status.namespace`, `status.ready`, `status.conditions[]`.
- Controller behavior: [`ProjectReconciler`](https://github.com/vaheed/kubeOP/blob/main/internal/operator/controllers/controllers.go#L65-L149) creates a namespace `kubeop-<tenantRef>-<name>`, ensures a `LimitRange`, `ResourceQuota`, and `NetworkPolicy`, then sets `status.namespace`, `status.ready=true`, and a `Ready` condition.

Minimal manifest:
```yaml
apiVersion: paas.kubeop.io/v1alpha1
kind: Project
metadata:
  name: example-project
spec:
  tenantRef: example
  name: example-project
```

Realistic manifest (matching controller-managed namespace policies):
```yaml
apiVersion: paas.kubeop.io/v1alpha1
kind: Project
metadata:
  name: example-project
spec:
  tenantRef: example
  name: payments
```

Status example:
```yaml
status:
  namespace: kubeop-example-example-project
  ready: true
  conditions:
    - type: Ready
      status: "True"
      reason: Bootstrapped
      message: Project namespace ready
```

## App (`apps.paas.kubeop.io`)

- Scope: Namespaced
- Spec fields (from [`paas.kubeop.io_apps.yaml`](https://github.com/vaheed/kubeOP/blob/main/deploy/k8s/crds/paas.kubeop.io_apps.yaml#L20-L45)):
  | Field | Type | Required | Validation |
  |-------|------|----------|------------|
  | `spec.type` | string | No | Enumerated: `Image`, `Git`, `Helm`, `Raw`.
  | `spec.image` | string | Conditionally | Required when `spec.type == "Image"`.
  | `spec.host` | string | No | Free-form host value.
- Status fields: `status.ready`, `status.revision`, `status.conditions[]`.
- Controller behavior: [`AppReconciler`](https://github.com/vaheed/kubeOP/blob/main/internal/operator/controllers/controllers.go#L151-L217) manages an `apps/v1.Deployment` named `app-<metadata.name>` when `spec.type` is `Image`. It stamps `status.revision` with the current UTC timestamp and marks readiness once the Deployment reports at least one available replica. When replicas are unavailable it sets `Ready` to `False` with reason `Progressing` and requeues after 5 seconds.

Minimal manifest:
```yaml
apiVersion: paas.kubeop.io/v1alpha1
kind: App
metadata:
  name: app
  namespace: kubeop-example-example-project
spec:
  type: Image
  image: ghcr.io/vaheed/sample:latest
```

Realistic manifest (exposes a vanity host):
```yaml
apiVersion: paas.kubeop.io/v1alpha1
kind: App
metadata:
  name: app
  namespace: kubeop-example-example-project
spec:
  type: Image
  image: ghcr.io/vaheed/sample:1.2.3
  host: app.example.test
```

Status example:
```yaml
status:
  ready: true
  revision: 20250101-120000
  conditions:
    - type: Ready
      status: "True"
      reason: Converged
      message: App reconciled
```

## DNSRecord (`dnsrecords.paas.kubeop.io`)

- Scope: Cluster
- Spec fields (from [`paas.kubeop.io_dnsrecords.yaml`](https://github.com/vaheed/kubeOP/blob/main/deploy/k8s/crds/paas.kubeop.io_dnsrecords.yaml#L20-L37)):
  | Field | Type | Required | Validation |
  |-------|------|----------|------------|
  | `spec.host` | string | Yes | Must be present and non-empty.
  | `spec.target` | string | Yes | Must be present and non-empty.
- Status fields: `status.ready`, `status.message`.
- Controller behavior: [`DNSRecordReconciler`](https://github.com/vaheed/kubeOP/blob/main/internal/operator/controllers/controllers.go#L219-L244) optionally POSTs to `DNS_MOCK_URL`, then sets `status.ready=true` and `status.message="mocked"`.

Minimal manifest:
```yaml
apiVersion: paas.kubeop.io/v1alpha1
kind: DNSRecord
metadata:
  name: app-host
spec:
  host: app.example.test
  target: 203.0.113.10
```

Realistic manifest (paired with certificate automation):
```yaml
apiVersion: paas.kubeop.io/v1alpha1
kind: DNSRecord
metadata:
  name: app-host
spec:
  host: app.example.test
  target: app.example.test.
```

Status example:
```yaml
status:
  ready: true
  message: mocked
```

## Certificate (`certificates.paas.kubeop.io`)

- Scope: Cluster
- Spec fields (from [`paas.kubeop.io_certificates.yaml`](https://github.com/vaheed/kubeOP/blob/main/deploy/k8s/crds/paas.kubeop.io_certificates.yaml#L20-L37)):
  | Field | Type | Required | Validation |
  |-------|------|----------|------------|
  | `spec.host` | string | Yes | Must be present and non-empty.
  | `spec.dnsRecordRef` | string | No | Optional reference to a `DNSRecord`.
- Status fields: `status.ready`, `status.message`.
- Controller behavior: [`CertificateReconciler`](https://github.com/vaheed/kubeOP/blob/main/internal/operator/controllers/controllers.go#L246-L269) optionally POSTs to `ACME_MOCK_URL`, then sets `status.ready=true` and `status.message="issued"`.

Minimal manifest:
```yaml
apiVersion: paas.kubeop.io/v1alpha1
kind: Certificate
metadata:
  name: app-cert
spec:
  host: app.example.test
```

Realistic manifest (references the DNS record created above):
```yaml
apiVersion: paas.kubeop.io/v1alpha1
kind: Certificate
metadata:
  name: app-cert
spec:
  host: app.example.test
  dnsRecordRef: app-host
```

Status example:
```yaml
status:
  ready: true
  message: issued
```

## Policy (`policies.paas.kubeop.io`)

- Scope: Cluster
- Spec fields (from [`paas.kubeop.io_policies.yaml`](https://github.com/vaheed/kubeOP/blob/main/deploy/k8s/crds/paas.kubeop.io_policies.yaml#L18-L33)):
  | Field | Type | Required | Validation |
  |-------|------|----------|------------|
  | `spec.egressAllowCIDRs` | []string | No | Maximum of 64 entries.
- Status: no status subresource.
- Controller: none today (the reconciler is not implemented), so policies are declarative data.

Minimal manifest:
```yaml
apiVersion: paas.kubeop.io/v1alpha1
kind: Policy
metadata:
  name: default-egress
spec: {}
```

Realistic manifest:
```yaml
apiVersion: paas.kubeop.io/v1alpha1
kind: Policy
metadata:
  name: restricted-egress
spec:
  egressAllowCIDRs:
    - 10.0.0.0/24
    - 192.0.2.0/24
```

## Registry (`registries.paas.kubeop.io`)

- Scope: Cluster
- Spec fields (from [`paas.kubeop.io_registries.yaml`](https://github.com/vaheed/kubeOP/blob/main/deploy/k8s/crds/paas.kubeop.io_registries.yaml#L18-L36)):
  | Field | Type | Required | Validation |
  |-------|------|----------|------------|
  | `spec.host` | string | Yes | Must be present and non-empty.
  | `spec.username` | string | No | If provided, `spec.passwordRef` must also be set.
  | `spec.passwordRef` | string | No | Required when username is set.
- Status: no status subresource.
- Controller: none today.

Minimal manifest:
```yaml
apiVersion: paas.kubeop.io/v1alpha1
kind: Registry
metadata:
  name: public
spec:
  host: ghcr.io
```

Realistic manifest:
```yaml
apiVersion: paas.kubeop.io/v1alpha1
kind: Registry
metadata:
  name: private
spec:
  host: registry.internal
  username: ci-bot
  passwordRef: kubeop-registry-cred
```
