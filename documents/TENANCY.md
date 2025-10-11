Tenancy And Projects

Model

- Default (shared user namespace): one namespace per user (`user-<userId>`). Bootstrap the user once per cluster; create many projects inside that namespace.
- Optional (per-project namespaces): set `PROJECTS_IN_USER_NAMESPACE=false` to create one dedicated namespace per project.

Quick comparison

- Namespace placement:
  - Shared: one namespace per user; projects share it.
  - Per-project: one namespace per project.
- Where kubeconfig is returned:
  - Shared: on `POST /v1/users/bootstrap` (user-scoped kubeconfig). `POST /v1/projects` omits kubeconfig.
  - Per-project: on `POST /v1/projects` (project-scoped kubeconfig).
- Quotas and suspend:
  - Shared: manage at the user namespace level (ResourceQuota). Project suspend/quota endpoints not applicable.
  - Per-project: manage per project via `/v1/projects/{id}/quota` and `/v1/projects/{id}/suspend|unsuspend`.

Namespace Naming

- Format: `tenant-<userId>-<slug(name)>` truncated to 63 chars.

Lifecycle

- Shared mode flow:
  - Bootstrap user: `POST /v1/users/bootstrap` (returns user-scoped kubeconfig).
  - Create project: `POST /v1/projects` creates project resources inside the user namespace. Provide `userId` (or switch to per-project mode and use `userEmail` to auto-create user).
  - Adjust limits: update the user namespace `ResourceQuota`.
- Per-project mode flow:
  - Create project: `POST /v1/projects` with `userId` or `userEmail`+`userName`; a dedicated namespace is created and kubeconfig is returned.
  - Adjust per-project limits: `PATCH /v1/projects/{id}/quota`; suspend/unsuspend via `/v1/projects/{id}/suspend|unsuspend`.
- Update quotas: in legacy mode, `PATCH /v1/projects/{id}/quota`. In shared-namespace mode, adjust the user namespace `ResourceQuota`.
- Suspend/unsuspend: in legacy mode, `POST /v1/projects/{id}/suspend|unsuspend`. In shared-namespace mode, suspend at the namespace level.
- Status: `GET /v1/projects/{id}` returns DB + basic presence checks.
- Delete: `DELETE /v1/projects/{id}` deletes project resources; in legacy mode, deletes the namespace; in shared-namespace mode, removes project LimitRange.

Config via ENV

- Pod Security Admission level: `POD_SECURITY_LEVEL` (default `restricted`).
- NetworkPolicy selectors for DNS and ingress namespaces: `DNS_NS_LABEL_*`, `DNS_POD_LABEL_*`, `INGRESS_NS_LABEL_*`.
- Service Account token TTL: `SA_TOKEN_TTL_SECONDS` (default 3600).
- Quota and limits defaults: `DEFAULT_QUOTA_*`, `DEFAULT_LR_*`.
