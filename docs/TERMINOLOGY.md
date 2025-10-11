Terminology And Synonyms

Core Terms

- User: an identity in the database with `id`, `name`, `email` used to determine namespace ownership and project placement.
- Cluster: a target Kubernetes cluster registered by uploading a base64 kubeconfig (`kubeconfig_b64`).
- Project: a logical app/workspace entry associated to a user and a cluster. Depending on tenancy mode, either shares a user namespace or has its own namespace.
- Namespace: the Kubernetes namespace where project resources live.
- Userspace (aka User Namespace, Tenant Namespace): the per-user namespace created by user bootstrap (name: `user-<userId>`).
- Bootstrap (aka Provision User Namespace): the process of creating a user’s namespace, defaults, SA/RBAC, token, and returning a kubeconfig.
- Kubeconfig (namespace-scoped): kubeconfig limited to a namespace via ServiceAccount token and context.
- Quota: Kubernetes ResourceQuota that caps resources (pods, CPU, memory, etc.).
- Suspend: temporarily restrict compute by setting quotas (per-project mode).

Tenancy Modes

- Shared User Namespace (default; `PROJECTS_IN_USER_NAMESPACE=true`):
  - Synonyms: shared mode, per-user namespace mode, userspace mode.
  - Effects: bootstrap returns kubeconfig; project creation omits kubeconfig and reuses user kubeconfig.
  - Controls: manage limits at the user namespace level.

- Per-Project Namespaces (`PROJECTS_IN_USER_NAMESPACE=false`):
  - Synonyms: isolated project mode, dedicated namespace mode.
  - Effects: project creation returns a project-scoped kubeconfig.
  - Controls: manage per-project quotas and suspend/unsuspend.

API Fields

- `kubeconfig_b64`: base64-encoded kubeconfig used only when registering clusters.
- `userId`: existing user identifier when bootstrapping or creating projects.
- `name` + `email`: used to create or reuse a user on bootstrap; also optional on project create (per-project mode) to auto-create a user if missing.
- `clusterId`: identifies the target cluster for bootstrap/project operations.

