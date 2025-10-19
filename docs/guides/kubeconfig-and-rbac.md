# Kubeconfig lifecycle and RBAC

kubeOP provisions namespace-scoped service accounts for users and projects, encrypts their kubeconfigs, and exposes APIs to rotate or revoke credentials. This guide explains how RBAC is applied and how to manage kubeconfigs.

## How kubeOP scopes access

- Users bootstrapped via `/v1/users/bootstrap` receive a namespace-scoped ServiceAccount (`user-<uuid>` namespace, `user-admin` ServiceAccount).
- Projects create additional bindings tied to the same namespace (unless `PROJECTS_IN_USER_NAMESPACE=false`).
- `internal/service/kubeconfigs.go` encrypts kubeconfigs with `KCFG_ENCRYPTION_KEY` before persisting them.
- RBAC manifests live in `internal/service/labels.go` and grant:
  - Namespaced CRUD for Deployments, StatefulSets, DaemonSets, Jobs, CronJobs.
  - Access to Secrets, ConfigMaps, PVCs, Services, Ingresses within the namespace.
  - `deployments/scale` and `statefulsets/scale` subresources for horizontal scaling.
  - No cluster-wide permissions.

## Issue or rotate kubeconfigs

### Ensure a binding exists

`POST /v1/kubeconfigs` creates or returns an existing binding. You can scope by user or project.

```bash
curl -s -X POST \
  -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{
    "userId": "<user-id>",
    "clusterId": "<cluster-id>"
  }' \
  http://localhost:8080/v1/kubeconfigs | jq
```

Response includes `id`, `namespace`, `service_account`, `secret_name`, and `kubeconfig_b64`.

### Rotate credentials

`POST /v1/kubeconfigs/rotate` issues a new token and kubeconfig for an existing binding.

```bash
curl -s -X POST \
  -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{ "id": "<binding-id>" }' \
  http://localhost:8080/v1/kubeconfigs/rotate | jq
```

kubeOP creates a new Secret, patches the ServiceAccount token reference, updates the encrypted kubeconfig in PostgreSQL, and returns the new base64 payload.

### Revoke access

`DELETE /v1/kubeconfigs/{id}` removes the binding and deletes the underlying Kubernetes Secret.

## RBAC validation

Use the returned kubeconfig to verify permissions.

```bash
kubectl --kubeconfig kubeconfig.yaml auth can-i create deployments -n <namespace>
kubectl --kubeconfig kubeconfig.yaml get pods -n <namespace>
```


## Auditing and events

- Every kubeconfig issuance is logged through the audit middleware (`internal/http/middleware/audit.go`).
- Project events record rotations and deletions when invoked from project-scoped APIs.
- For compliance, export `logs/audit.log` (or your logging sink) and rotate `ADMIN_JWT_SECRET` periodically.

## Troubleshooting

- `invalid token` responses usually mean the presented JWT does not include `{"role":"admin"}` or was signed with the wrong secret.
- If kubeconfig secrets drift (deleted manually), re-run `POST /v1/kubeconfigs` to recreate them.
- Ensure the target namespace exists; kubeOP auto-creates it during bootstrap but manual deletions require re-bootstrap.
