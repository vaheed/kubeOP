Tenancy And Projects

Model

- A user owns one or more projects.
- A project targets a specific cluster and gets a dedicated Kubernetes namespace.
- KubeOP provisions namespace resources and returns a namespace-scoped kubeconfig (base64) for the user.

Namespace Naming

- Format: `tenant-<userId>-<slug(name)>` truncated to 63 chars.

Lifecycle

- Create: `POST /v1/projects` creates namespace, quota, limitrange, network policies, service account, role, rolebinding, mints SA token, and returns a kubeconfig (base64). The kubeconfig is also stored encrypted.
- Suspend: `POST /v1/projects/{id}/suspend` sets ResourceQuota to block new pods (and most new resources) while preserving existing workloads.
- Unsuspend: `POST /v1/projects/{id}/unsuspend` restores quotas to defaults/overrides.
- Update Quotas: `PATCH /v1/projects/{id}/quota` applies overrides.
- Status: `GET /v1/projects/{id}` returns DB + basic reconciliation status.
- Delete: `DELETE /v1/projects/{id}` deletes the namespace and metadata.

Config via ENV

- Pod Security Admission level: `POD_SECURITY_LEVEL` (default `restricted`).
- NetworkPolicy selectors for DNS and ingress namespaces: `DNS_NS_LABEL_*`, `DNS_POD_LABEL_*`, `INGRESS_NS_LABEL_*`.
- Service Account token TTL: `SA_TOKEN_TTL_SECONDS` (default 3600).
- Quota and limits defaults: `DEFAULT_QUOTA_*`, `DEFAULT_LR_*`.

