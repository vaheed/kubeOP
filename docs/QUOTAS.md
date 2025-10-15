Quotas And Limits

Defaults (ENV)

- Namespace quota caps: `KUBEOP_DEFAULT_REQUESTS_CPU`, `KUBEOP_DEFAULT_LIMITS_CPU`, `KUBEOP_DEFAULT_REQUESTS_MEMORY`,
  `KUBEOP_DEFAULT_LIMITS_MEMORY`, `KUBEOP_DEFAULT_REQUESTS_EPHEMERAL`, `KUBEOP_DEFAULT_LIMITS_EPHEMERAL`, `KUBEOP_DEFAULT_PODS`,
  `KUBEOP_DEFAULT_SERVICES`, `KUBEOP_DEFAULT_SERVICES_LOADBALANCERS`, `KUBEOP_DEFAULT_CONFIGMAPS`, `KUBEOP_DEFAULT_SECRETS`,
  `KUBEOP_DEFAULT_PVCS`, `KUBEOP_DEFAULT_REQUESTS_STORAGE`, `KUBEOP_DEFAULT_DEPLOYMENTS_APPS`,
  `KUBEOP_DEFAULT_REPLICASETS_APPS`, `KUBEOP_DEFAULT_STATEFULSETS_APPS`, `KUBEOP_DEFAULT_JOBS_BATCH`,
  `KUBEOP_DEFAULT_CRONJOBS_BATCH`, `KUBEOP_DEFAULT_INGRESSES_NETWORKING_K8S_IO`.
- LimitRange defaults per container/pod: `KUBEOP_DEFAULT_LR_CONTAINER_*`, plus optional extended resource caps via
  `KUBEOP_DEFAULT_LR_EXT_*`.
- Project LimitRange defaults (shared-namespace mode): `PROJECT_LR_REQUEST_CPU`, `PROJECT_LR_REQUEST_MEMORY`,
  `PROJECT_LR_LIMIT_CPU`, `PROJECT_LR_LIMIT_MEMORY`.

Overrides

- Use `PATCH /v1/projects/{id}/quota` with `{ "overrides": { "limits.cpu": "256", "pods": "100" } }` to override.
- Inspect effective values and usage via `GET /v1/projects/{id}/quota` (includes ResourceQuota hard/used maps and load balancer caps).
- Keys/values are stored as JSON—quotes and surrounding whitespace are normalised automatically, so send valid JSON string values (for example `"1Gi"`).

Suspend/Unsuspend

- Suspend sets `pods: 0` to block new workloads; existing pods may continue running.
- Unsuspend reapplies defaults plus any overrides.

