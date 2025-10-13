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
  - Create project: `POST /v1/projects` with `userId` or `userEmail`+`userName`; a dedicated namespace is created (with PSA + default NetworkPolicies) and kubeconfig is returned.
  - Adjust per-project limits: `PATCH /v1/projects/{id}/quota`; suspend/unsuspend via `/v1/projects/{id}/suspend|unsuspend`.
- Update quotas: in legacy mode, `PATCH /v1/projects/{id}/quota`. In shared-namespace mode, adjust the user namespace `ResourceQuota`.
- Suspend/unsuspend: in legacy mode, `POST /v1/projects/{id}/suspend|unsuspend`. In shared-namespace mode, suspend at the namespace level.
- Status: `GET /v1/projects/{id}` returns DB + basic presence checks.
- Delete: `DELETE /v1/projects/{id}` deletes project resources; in legacy mode, deletes the namespace; in shared-namespace mode, removes project LimitRange.

RBAC for User Kubeconfigs

- Each user kubeconfig authenticates as a ServiceAccount `user-sa` in the user namespace.
- The bound Role allows managing common namespaced resources and inspecting app rollouts:
  - Core: pods, services, configmaps, secrets, persistentvolumeclaims.
  - Apps: deployments, replicasets, statefulsets, daemonsets.
  - Batch: jobs, cronjobs.
- Note: ReplicaSets are included to allow `kubectl describe deploy` and rollout visibility. Cluster-scoped resources remain forbidden.

Deletion semantics

- All delete operations are soft-deleted in the database (set `deleted_at`), and hard-deleted in Kubernetes where applicable.
- Delete app: marks app row deleted; deletes labeled resources in the project namespace.
- Delete project: marks project row (and its apps) deleted; deletes the project namespace in per-project mode or removes project-specific LimitRange in shared mode.
- Delete user: marks user row deleted; deletes user namespaces across all clusters.

Network Policies (isolation)

- Both per-project and shared user namespaces receive default NetworkPolicies:
  - default-deny for Ingress and Egress
  - allow egress to DNS (namespace/pod selectors configurable by ENV)
  - allow ingress from namespaces labeled to host ingress controllers

Config via ENV

- Pod Security Admission level: `POD_SECURITY_LEVEL` (default `baseline`; set to `restricted` to require non-root containers).
- NetworkPolicy selectors for DNS and ingress namespaces: `DNS_NS_LABEL_*`, `DNS_POD_LABEL_*`, `INGRESS_NS_LABEL_*`.
- Service Account token TTL: `SA_TOKEN_TTL_SECONDS` (deprecated; tokens are non-expiring and minted from annotated Secrets).
- Quota and limits defaults: `DEFAULT_QUOTA_*`, `DEFAULT_LR_*`.
