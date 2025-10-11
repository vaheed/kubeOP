Tenancy And Projects

Model

- Default: projects share a per-user namespace (`user-<userId>`). Bootstrap the user once; create many projects in that namespace.
- Optional per-project mode: set `PROJECTS_IN_USER_NAMESPACE=false` to create a dedicated namespace per project.

Namespace Naming

- Format: `tenant-<userId>-<slug(name)>` truncated to 63 chars.

Lifecycle

- Bootstrap user: `POST /v1/users/bootstrap` to create the user namespace and get a user-scoped kubeconfig.
- Create project: `POST /v1/projects` creates project resources inside the user namespace (shared mode). Provide `userId` (or use `userEmail` on per-project mode to auto-create user).
- Update quotas: in legacy mode, `PATCH /v1/projects/{id}/quota`. In shared-namespace mode, adjust the user namespace `ResourceQuota`.
- Suspend/unsuspend: in legacy mode, `POST /v1/projects/{id}/suspend|unsuspend`. In shared-namespace mode, suspend at the namespace level.
- Status: `GET /v1/projects/{id}` returns DB + basic presence checks.
- Delete: `DELETE /v1/projects/{id}` deletes project resources; in legacy mode, deletes the namespace; in shared-namespace mode, removes project LimitRange.

Config via ENV

- Pod Security Admission level: `POD_SECURITY_LEVEL` (default `restricted`).
- NetworkPolicy selectors for DNS and ingress namespaces: `DNS_NS_LABEL_*`, `DNS_POD_LABEL_*`, `INGRESS_NS_LABEL_*`.
- Service Account token TTL: `SA_TOKEN_TTL_SECONDS` (default 3600).
- Quota and limits defaults: `DEFAULT_QUOTA_*`, `DEFAULT_LR_*`.
