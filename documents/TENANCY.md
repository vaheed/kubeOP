Tenancy And Projects

Model

- v0.1.1 default: each project has its own namespace (per-project mode).
- Optional shared mode: set `PROJECTS_IN_USER_NAMESPACE=true` to place projects into a pre-provisioned user namespace (`user-<userId>`). In that mode, kubeconfig is managed externally.

Namespace Naming

- Format: `tenant-<userId>-<slug(name)>` truncated to 63 chars.

Lifecycle

- Create project: `POST /v1/projects` creates a dedicated namespace per project by default and returns a namespace-scoped kubeconfig.
- Update quotas: in legacy mode, `PATCH /v1/projects/{id}/quota`. In shared-namespace mode, adjust the user namespace `ResourceQuota`.
- Suspend/unsuspend: in legacy mode, `POST /v1/projects/{id}/suspend|unsuspend`. In shared-namespace mode, suspend at the namespace level.
- Status: `GET /v1/projects/{id}` returns DB + basic presence checks.
- Delete: `DELETE /v1/projects/{id}` deletes project resources; in legacy mode, deletes the namespace; in shared-namespace mode, removes project LimitRange.

Config via ENV

- Pod Security Admission level: `POD_SECURITY_LEVEL` (default `restricted`).
- NetworkPolicy selectors for DNS and ingress namespaces: `DNS_NS_LABEL_*`, `DNS_POD_LABEL_*`, `INGRESS_NS_LABEL_*`.
- Service Account token TTL: `SA_TOKEN_TTL_SECONDS` (default 3600).
- Quota and limits defaults: `DEFAULT_QUOTA_*`, `DEFAULT_LR_*`.
