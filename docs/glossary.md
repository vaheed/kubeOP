# Glossary

| Term | Definition |
| --- | --- |
| **Tenant** | A user managed by kubeOP. See `store.User` and `/v1/users/bootstrap`. |
| **User space** | Namespace + ServiceAccount created per tenant per cluster. Stores encrypted kubeconfig. |
| **Project** | Logical grouping of apps under a tenant. Maps to a namespace (shared or dedicated). |
| **App** | Deployment unit managed by kubeOP (image, Helm chart, or raw manifests). |
| **Watcher** | Out-of-cluster agent (`cmd/kubeop-watcher`) that streams labelled Kubernetes events to kubeOP. |
| **Watcher sink** | Batching pipeline (`internal/sink`) handling deduplication, compression, and HTTPS delivery. |
| **Quota** | Namespace ResourceQuota enforced per tenant/project. Configurable via `KUBEOP_DEFAULT_*` and `/v1/projects/{id}/quota`. |
| **LimitRange** | Namespace defaults for container requests/limits. Controlled by `KUBEOP_DEFAULT_LR_*` variables. |
| **Cluster health scheduler** | Background job (`service.NewClusterHealthScheduler`) that pings registered clusters and records results. |
| **Audit log** | Middleware (`internal/http/middleware.AuditLog`) capturing request metadata for compliance. |
| **Watcher token** | Short-lived JWT issued via `/v1/watchers/register` or `/v1/watchers/refresh` that authenticates watcher batches. |
