> **What this page explains**: kubeOP security controls around credentials and access.
> **Who it's for**: Security engineers and auditors reviewing the platform.
> **Why it matters**: Details how kubeconfigs, RBAC, and tokens stay under control.

# Security controls

Security starts with how kubeOP stores secrets and issues credentials. Each mechanism aims to minimize blast radius while keeping automation friendly.

## Kubeconfig issuance

### Storage
All cluster kubeconfigs are supplied via the `kubeconfig_b64` field and encrypted using `internal/crypto` with the `KCFG_ENCRYPTION_KEY` environment variable.

### Tenant kubeconfigs
Tenants receive scoped kubeconfigs that reference dedicated service accounts. kubeOP signs them as-needed and avoids long-lived admin credentials.

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: payments-kubeconfig
  namespace: payments-prod
type: Opaque
data:
  kubeconfig: {{ .Status.kubeconfig_b64 }}
```

## RBAC templates

### Role generation
Roles and RoleBindings are rendered from templates per cluster. The defaults restrict tenants to their namespaces while allowing read access to shared resources like ConfigMaps tagged with `tenant.kubeop.dev/shared=true`.

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  name: tenant-admin
  namespace: payments-prod
rules:
  - apiGroups: ["", "apps", "batch"]
    resources: ["deployments", "statefulsets", "jobs", "pods", "configmaps", "secrets"]
    verbs: ["get", "list", "watch", "create", "update", "patch", "delete"]
```

## Token policies

### Admin tokens
Admin endpoints demand JWTs with `{ "role": "admin" }`. Rotate the signing secret regularly and store it only in secrets managers or GitHub Actions secrets.

### Tenant tokens
Tenant-facing APIs issue signed tokens with per-project claims. Expiry defaults to 24 hours but can be tuned via configuration.

```go
token, err := jwt.Sign(jwt.HS256, []byte(os.Getenv("ADMIN_JWT_SECRET")), jwt.Claims{
    "role": "admin",
    "exp": time.Now().Add(4 * time.Hour).Unix(),
})
if err != nil {
    return fmt.Errorf("issue admin token: %w", err)
}
```

### Audit
Every credential issuance logs the requesting principal and target tenant. Set up alerts on unusual issuance spikes to catch compromised accounts early.

