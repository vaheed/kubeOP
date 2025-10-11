User Flows

Shared User Namespace (default)

1) Register a cluster
- Create base64 from kubeconfig and POST to `/v1/clusters`.

2) Bootstrap a user
- POST `/v1/users/bootstrap` with either `{userId, clusterId}` or `{name, email, clusterId}`.
- Receive `kubeconfig_b64` for the user namespace `user-<userId>`; decode and use for kubectl.

3) Create projects
- POST `/v1/projects` with `{userId, clusterId, name}`.
- Response omits kubeconfig; keep using the user kubeconfig for all projects.

4) Manage limits
- Adjust the user namespace `ResourceQuota` (project-level suspend/quota APIs aren’t applicable in this mode).

Per-Project Namespaces

0) Enable per-project mode
- Set `PROJECTS_IN_USER_NAMESPACE=false` and restart the API.

1) Register a cluster
- Same as shared mode.

2) Create a project
- POST `/v1/projects` with either `{userId, clusterId, name}` or `{userEmail, userName, clusterId, name}`.
- Receive a project-scoped `kubeconfig_b64` for that project’s namespace.

3) Manage per-project limits
- Use `PATCH /v1/projects/{id}/quota` and `/v1/projects/{id}/suspend|unsuspend`.

Status and Ops

- `GET /v1/projects/{id}` returns DB info and basic presence checks for key resources.
- Health/Readiness endpoints: `/healthz`, `/readyz`.

