# FAQ

**How do I generate an admin token?**  Use any JWT library to sign `{"role":"admin"}` with `ADMIN_JWT_SECRET`. Include `sub` or `email` to improve audit logs.


**Does kubeOP manage CRDs?**  Raw manifests allow any resource type. kubeOP applies them with server-side apply, so CRDs can be managed if the kubeconfig has permission.

**How do I rotate secrets without downtime?**  Call the rotation endpoint (`/v1/kubeconfigs/rotate` or `/v1/projects/{id}/kubeconfig/renew`) and update workloads to use the new kubeconfig or Secret. kubeOP does not automatically restart pods when Secrets change.

**What databases are supported?**  PostgreSQL 14+ is required. `DATABASE_URL` can target managed services (RDS, CloudSQL) as long as TLS settings are configured externally.

**Where are logs stored?**  Structured application and audit logs go to stdout and `${LOGS_ROOT}` (default `/var/log/kubeop`). Each project has its own directory for app logs and events.


**How do I add new quota defaults?**  Update `.env` with `KUBEOP_DEFAULT_*` variables and restart the API. Existing namespaces require manual reconciliation.


**Is there an OpenAPI spec?**  `docs/openapi.yaml` tracks the REST surface manually. Use it with tools like `redoc-cli` to generate HTML if needed.
