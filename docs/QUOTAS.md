Quotas And Limits

Defaults (ENV)

- DEFAULT_QUOTA_LIMITS_MEMORY: default aggregate memory limit (e.g., 64Gi)
- DEFAULT_QUOTA_LIMITS_CPU: default aggregate CPU limit (cores, e.g., 128)
- DEFAULT_QUOTA_EPHEMERAL_STORAGE: default ephemeral storage limit
- DEFAULT_QUOTA_PVC_STORAGE: sum of PVC requests allowed
- DEFAULT_QUOTA_MAX_PODS: cap maps to "max 50 apps" guideline
- DEFAULT_LR_REQUEST_CPU / DEFAULT_LR_REQUEST_MEMORY: container default requests
- DEFAULT_LR_LIMIT_CPU / DEFAULT_LR_LIMIT_MEMORY: container default limits

Overrides

- Use `PATCH /v1/projects/{id}/quota` with `{ "overrides": { "limits.cpu": "256", "pods": "100" } }` to override.
- Inspect effective values and usage via `GET /v1/projects/{id}/quota` (includes ResourceQuota hard/used maps and load balancer caps).
- Keys/values are stored as JSON—quotes and surrounding whitespace are normalised automatically, so send valid JSON string values (for example `"1Gi"`).

Suspend/Unsuspend

- Suspend sets `pods: 0` to block new workloads; existing pods may continue running.
- Unsuspend reapplies defaults plus any overrides.

