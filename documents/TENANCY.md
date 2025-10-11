Tenancy And Projects

Model

- A user has a dedicated namespace per cluster (`user-<userId>`), provisioned at bootstrap.
- The user can create multiple projects inside that namespace by default (`PROJECTS_IN_USER_NAMESPACE=true`).
- Optionally (legacy mode), each project can get its own namespace if `PROJECTS_IN_USER_NAMESPACE=false`.

Namespace Naming

- Format: `tenant-<userId>-<slug(name)>` truncated to 63 chars.

Lifecycle

- Create user: `POST /v1/users` and obtain the `id` for the next step.
- Bootstrap user: `POST /v1/users/bootstrap` with `userId` and `clusterId` to provision the user namespace with quotas, limits, PSA, SA + RBAC, mint a token, and return a base64 kubeconfig scoped to that namespace.
- Create project: `POST /v1/projects` applies project defaults (LimitRange) inside the user namespace (default). In legacy mode, it creates a namespace per project and returns kubeconfig.
- Update quotas: in legacy mode, `PATCH /v1/projects/{id}/quota`. In shared-namespace mode, adjust the user namespace `ResourceQuota`.
- Suspend/unsuspend: in legacy mode, `POST /v1/projects/{id}/suspend|unsuspend`. In shared-namespace mode, suspend at the namespace level.
- Status: `GET /v1/projects/{id}` returns DB + basic presence checks.
- Delete: `DELETE /v1/projects/{id}` deletes project resources; in legacy mode, deletes the namespace; in shared-namespace mode, removes project LimitRange.

Config via ENV

- Pod Security Admission level: `POD_SECURITY_LEVEL` (default `restricted`).
- NetworkPolicy selectors for DNS and ingress namespaces: `DNS_NS_LABEL_*`, `DNS_POD_LABEL_*`, `INGRESS_NS_LABEL_*`.
- Service Account token TTL: `SA_TOKEN_TTL_SECONDS` (default 3600).
- Quota and limits defaults: `DEFAULT_QUOTA_*`, `DEFAULT_LR_*`.
