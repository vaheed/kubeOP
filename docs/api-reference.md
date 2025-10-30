---
outline: deep
---

# API Reference

kubeOP exposes a JSON REST API served by `cmd/manager`, plus Kubernetes Custom Resources handled by the operator. All endpoints default to `http://localhost:8080` in development and require a Bearer token with `role=admin` when `KUBEOP_REQUIRE_AUTH=true`.

## REST endpoints

| Method | Path | Description |
| --- | --- | --- |
| `GET` | `/healthz` | Liveness probe. |
| `GET` | `/metrics` | Prometheus metrics. |
| `GET` | `/version` | Build metadata. |
| `GET` | `/v1/clusters` | List registered clusters. |
| `POST` | `/v1/clusters` | Register a cluster (kubeconfig YAML). |
| `GET`/`DELETE` | `/v1/clusters/{id}` | Describe or delete a cluster. Delete is blocked while tenants exist. |
| `POST` | `/v1/tenants` | Create tenant with quota envelope. |
| `GET`/`DELETE` | `/v1/tenants/{id}` | Describe or delete tenant. |
| `POST` | `/v1/projects` | Create project bound to a tenant and namespace. |
| `GET`/`DELETE` | `/v1/projects/{id}` | Describe or delete project. |
| `POST` | `/v1/apps` | Create application spec (delivery metadata). |
| `GET`/`DELETE` | `/v1/apps/{id}` | Describe or delete app. |
| `GET` | `/v1/usage/snapshot` | Aggregate quotas, project/app counts, and namespaces. |
| `GET` | `/v1/invoices/{tenantID}` | Generate monthly invoice summary for a tenant. |
| `GET` | `/v1/analytics/summary` | Delivery and registry analytics. |

### Sample: register a cluster

```bash
curl -sS -H "Authorization: Bearer $ADMIN" -H 'Content-Type: application/json' \
  -X POST http://localhost:18080/v1/clusters \
  -d "$(jq -n --arg name dev --arg kc "$KUBECONFIG" '{name:$name,kubeconfig:$kc}')"
```

Response:

```json
{
  "id": "0a2c91c8-4422-4c4b-94fa-1ad2db2c6c63",
  "name": "dev",
  "labels": {"app.kubeop.io/cluster-id": "0a2c91c8-..."},
  "finalizers": ["kubeop.io/cleanup"],
  "created_at": "2025-11-02T10:15:00Z"
}
```

### Sample: usage snapshot and invoice

```bash
curl -sS -H "Authorization: Bearer $ADMIN" http://localhost:18080/v1/usage/snapshot | jq '.totals'
curl -sS -H "Authorization: Bearer $ADMIN" http://localhost:18080/v1/invoices/$TENANT_ID | jq '{tenant_name, subtotal, lines}'
```

## Kubernetes Custom Resources

While the manager API covers the hierarchy (clusters → tenants → projects → apps), platform features are expressed via CRDs applied directly to Kubernetes. All CRDs are cluster-scoped unless noted.

| Kind | Scope | Purpose | Sample command |
| --- | --- | --- | --- |
| `Tenant` | Cluster | Operator reconciliation anchor; labels ensure namespace and quota propagation. | `kubectl get tenants.platform.kubeop.io` |
| `Project` | Cluster | Declares tenant-scoped namespaces, quotas, policies. | `kubectl get projects.platform.kubeop.io` |
| `App` | Namespaced | Delivery spec converged into workloads. | `kubectl -n kubeop-web get apps.platform.kubeop.io` |
| `Policy` | Cluster | Defines egress CIDR allow lists and ingress policies. | `kubectl get policies.platform.kubeop.io` |
| `DNSRecord` | Cluster | Declares external host → service mapping. | `kubectl get dnsrecords.platform.kubeop.io` |
| `Certificate` | Cluster | Requests TLS materials for DNS records. | `kubectl get certificates.platform.kubeop.io` |
| `Hook` (Jobs) | Namespaced | Pre/post deployment Jobs spawned automatically per revision. | `kubectl -n kubeop-web get jobs -l app.kubeop.io/app-id=<app>` |

Refer to `deploy/k8s/crds/` for schema definitions and `deploy/k8s/samples/` for ready-made examples.

## Error responses

All API errors share the structure `{ "error": "message" }` with appropriate HTTP status codes:

- `400` – Validation failure (e.g., quota overflow, namespace mismatch, disallowed registry).
- `401` – Missing/invalid token when auth is enabled.
- `403` – Authenticated but lacking the `admin` role.
- `404` – Resource not found.
- `409` – Attempting to delete a parent with dependent resources.

## Authentication & headers

Set `Authorization: Bearer <jwt>` where the token uses HS256 and includes a `role` claim. Example claim payload:

```json
{
  "iss": "kubeop",
  "sub": "admin",
  "role": "admin",
  "iat": 1730544000,
  "exp": 1730544900
}
```

Tokens are validated by the manager and surfaced to handler context for RBAC enforcement.
