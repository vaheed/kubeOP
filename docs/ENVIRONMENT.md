Environment Variables

- APP_ENV: application environment (default `development`).
- PORT: HTTP port (default `8080`).
- LOG_LEVEL: `debug|info|warn|error` (default `info`). Controls zap logging level for stdout/app.log.
- PUBLIC_URL: External HTTPS base URL for kubeOP (default
  `https://localhost:8443`). Used to derive the watcher ingest endpoint when
  explicit overrides are not provided.
- LOGS_ROOT: root directory for project/app logs (default `/var/log/kubeop`). The API creates `projects/<project_id>/...` under this path after trimming whitespace and requiring identifiers to match `[A-Za-z0-9._-]+`. Other characters are rejected before touching disk and the resolved path must already be absolute/clean (relative or normalising variants fail fast).
- LOG_DIR: directory containing application/audit logs (defaults to `LOGS_ROOT`). Ensure the process can create it when overridden.
- LOG_MAX_SIZE_MB: rotate log files after this many megabytes (default `50`).
- LOG_MAX_BACKUPS: number of rotated files to keep (default `7`).
- LOG_MAX_AGE_DAYS: days to retain rotated logs (default `14`).
- LOG_COMPRESS: gzip rotated logs when `true` (default `true`).
- AUDIT_ENABLED: enable JSON audit events for mutating requests (default `true`).
- CLUSTER_ID: optional metadata added to every log line; useful when running per-cluster control planes.
- DISABLE_AUTH: bypass admin JWT middleware (default `false`).
  - When disabled (auth enabled), pass an Authorization header like: `AUTH_H="-H 'Authorization: Bearer $TOKEN'"` and include `$AUTH_H` in curl commands.
- DATABASE_URL: Postgres DSN, e.g., `postgres://user:pass@host:5432/kubeop?sslmode=disable`.
- EVENTS_DB_ENABLED: When `true` (default), project events are written to PostgreSQL in addition to `${LOGS_ROOT}/projects/<project_id>/events.jsonl`. Set to `false` for file-only sinks.
- K8S_EVENTS_BRIDGE: Enables ingestion of Kubernetes core/v1 Events into the project event stream when a bridge component is deployed (default `false`).
- ADMIN_JWT_SECRET: HMAC secret for admin JWTs (required unless `DISABLE_AUTH=true`).
- KCFG_ENCRYPTION_KEY: key for AES-GCM at-rest encryption. Accepts Base64 or hex; otherwise SHA-256 of literal string is used.
- CONFIG_FILE: optional path to YAML file with defaults. Values from env override file.

Tenancy / Projects

- PROJECTS_IN_USER_NAMESPACE: if `true`, projects live in a user namespace (shared mode). Default is `true` (one user, many projects). If `false`, each project gets its own namespace and receives a kubeconfig.
  - `true` (shared mode): get kubeconfig from `POST /v1/users/bootstrap`, reuse for all projects; project responses omit kubeconfig.
  - `false` (per-project): `POST /v1/projects` returns a project-scoped kubeconfig; use project quota/suspend endpoints.
- POD_SECURITY_LEVEL: Pod Security Admission level label for namespaces (default `baseline`; set to `restricted` to enforce non-
  root containers).
- DNS_NS_LABEL_KEY / DNS_NS_LABEL_VALUE: label selector for the DNS namespace (defaults `kubernetes.io/metadata.name=kube-system`).
- DNS_POD_LABEL_KEY / DNS_POD_LABEL_VALUE: label selector for DNS pods (defaults `k8s-app=kube-dns`).
- INGRESS_NS_LABEL_KEY / INGRESS_NS_LABEL_VALUE: label selector to allow ingress traffic from selected namespaces (default `kubeop.io/ingress=true`).

Quotas and Limits

- NamespaceLimitPolicy defaults:
  - `KUBEOP_DEFAULT_REQUESTS_CPU`, `KUBEOP_DEFAULT_LIMITS_CPU` – namespace CPU request/limit caps.
  - `KUBEOP_DEFAULT_REQUESTS_MEMORY`, `KUBEOP_DEFAULT_LIMITS_MEMORY` – namespace memory request/limit caps.
  - `KUBEOP_DEFAULT_REQUESTS_EPHEMERAL`, `KUBEOP_DEFAULT_LIMITS_EPHEMERAL` – ephemeral storage request/limit caps.
  - `KUBEOP_DEFAULT_PODS`, `KUBEOP_DEFAULT_SERVICES`, `KUBEOP_DEFAULT_SERVICES_LOADBALANCERS`, `KUBEOP_DEFAULT_CONFIGMAPS`, `KUBEOP_DEFAULT_SECRETS`, `KUBEOP_DEFAULT_PVCS`, `KUBEOP_DEFAULT_REQUESTS_STORAGE`, `KUBEOP_DEFAULT_DEPLOYMENTS_APPS`, `KUBEOP_DEFAULT_REPLICASETS_APPS`, `KUBEOP_DEFAULT_STATEFULSETS_APPS`, `KUBEOP_DEFAULT_JOBS_BATCH`, `KUBEOP_DEFAULT_CRONJOBS_BATCH`, `KUBEOP_DEFAULT_INGRESSES_NETWORKING_K8S_IO` – resource/object quotas enforced per namespace.
  - `KUBEOP_DEFAULT_SCOPES` – comma-separated ResourceQuota scopes (e.g. `NotBestEffort`).
  - `KUBEOP_DEFAULT_PRIORITY_CLASSES` – optional allow-list for priority classes applied via `ScopeSelector`.
- LimitRange defaults (per container/pod):
  - `KUBEOP_DEFAULT_LR_CONTAINER_MAX_CPU`, `KUBEOP_DEFAULT_LR_CONTAINER_MIN_CPU`, `KUBEOP_DEFAULT_LR_CONTAINER_DEFAULT_CPU`, `KUBEOP_DEFAULT_LR_CONTAINER_DEFAULTREQUEST_CPU` – container CPU limits.
  - `KUBEOP_DEFAULT_LR_CONTAINER_MAX_MEMORY`, `KUBEOP_DEFAULT_LR_CONTAINER_MIN_MEMORY`, `KUBEOP_DEFAULT_LR_CONTAINER_DEFAULT_MEMORY`, `KUBEOP_DEFAULT_LR_CONTAINER_DEFAULTREQUEST_MEMORY` – container memory limits.
  - `KUBEOP_DEFAULT_LR_CONTAINER_MAX_EPHEMERAL`, `KUBEOP_DEFAULT_LR_CONTAINER_MIN_EPHEMERAL`, `KUBEOP_DEFAULT_LR_CONTAINER_DEFAULT_EPHEMERAL`, `KUBEOP_DEFAULT_LR_CONTAINER_DEFAULTREQUEST_EPHEMERAL` – container ephemeral storage limits.
  - `KUBEOP_DEFAULT_LR_EXT_MAX`, `KUBEOP_DEFAULT_LR_EXT_MIN`, `KUBEOP_DEFAULT_LR_EXT_DEFAULT`, `KUBEOP_DEFAULT_LR_EXT_DEFAULTREQUEST` – extended resource limits (comma-separated `resource=value`).
- PROJECT_LR_REQUEST_CPU, PROJECT_LR_REQUEST_MEMORY, PROJECT_LR_LIMIT_CPU, PROJECT_LR_LIMIT_MEMORY: per-project LimitRange defaults (should be <= namespace defaults). Defaults are `100m`, `128Mi`, `1`, and `1Gi` respectively.
- kubeOP reapplies the managed `tenant-quota` and `tenant-limits` objects whenever it provisions namespaces, updates quota overrides, or toggles project suspension so drift from the configured defaults is corrected automatically.

Scheduler

- CLUSTER_HEALTH_INTERVAL_SECONDS: interval in seconds for logging cluster health in the background (default `60`).
- Scheduler helper enforces a 20 second timeout per cluster probe today; future releases may expose `CLUSTER_HEALTH_TIMEOUT_SECONDS` once tuning data is available.

Ingress & Load Balancers

- PAAS_DOMAIN: Base domain for generated Ingress hosts (e.g., `apps.example.com`).
- PAAS_WILDCARD_ENABLED: When true, KubeOP generates `{app}.{namespace}.{PAAS_DOMAIN}` if `domain` isn’t provided on app deploy.
- LB_DRIVER: Load balancer driver name. Default `metallb`.
- LB_METALLB_POOL: Optional MetalLB address-pool annotation applied to Services.
- MAX_LOADBALANCERS_PER_PROJECT: Default cap for `LoadBalancer` Services per project. Can be overridden using project quota key `services.loadbalancers`.
- ENABLE_CERT_MANAGER: When `true`, create a cert-manager `Certificate` and set Ingress TLS for app hosts. Requires cert-manager installed in the cluster.

CI Webhooks

- GIT_WEBHOOK_SECRET: If set, `/v1/webhooks/git` verifies `X-Hub-Signature-256` (HMAC-SHA256) for incoming payloads.

External DNS (optional)

- EXTERNAL_DNS_PROVIDER: `cloudflare|powerdns|""` — enables DNS automation to upsert A records for app hosts.
- EXTERNAL_DNS_TTL: TTL for created records (default `300`).
- Cloudflare:
  - CF_API_TOKEN: token with DNS edit permissions for the zone.
  - CF_ZONE_ID: Cloudflare Zone ID (UUID) where records are created (note: this is not the human-readable zone name).
  - KubeOP polls asynchronously for the Service load balancer IP before creating or updating the Cloudflare record, logging `dns_wait_for_load_balancer_ip` while waiting. DNS automation now logs through the primary service logger with project/app/cluster metadata and surfaces Cloudflare API response bodies on errors for faster debugging.
- PowerDNS:
  - PDNS_API_URL: e.g., `http://pdns:8081`.
  - PDNS_API_KEY: X-API-Key value.
  - PDNS_SERVER_ID: server identifier (default `localhost`).
- PDNS_ZONE: Zone name; defaults to `PAAS_DOMAIN` if empty.

Watcher Bridge (`cmd/kubeop-watcher`)

- CLUSTER_ID: required cluster UUID (propagated to every event payload).
- WATCHER_URL: Base URL for kubeOP’s public API (http or https). Events are
  posted to `${WATCHER_URL}/v1/events` and health syncs poll
  `${WATCHER_URL}/v1/health`. Legacy `KUBEOP_EVENTS_URL` continues to work but
  will be removed in a future release.
- KUBEOP_TOKEN: Bearer token accepted by the kubeOP API (required).
- KUBECONFIG: optional path to a kubeconfig on disk. When unset, the
  watcher uses in-cluster service account credentials.
- WATCH_KINDS: comma-separated list of kinds to watch (defaults to all
  supported kinds).
- LABEL_SELECTOR: defaults to
  `kubeop.project.id,kubeop.app.id,kubeop.tenant.id`; existence-only
  keys double as the watcher’s guard rails.
- BATCH_MAX / BATCH_WINDOW_MS: batching controls (defaults 200 events,
  1000 ms).
- STORE_PATH: BoltDB location for resource version checkpoints
  (`/var/lib/kubeop-watcher/state.db`).
- HEARTBEAT_MINUTES: optional synthetic heartbeat interval (`0` to
  disable).
- WATCHER_LISTEN_ADDR: HTTP bind address for `/healthz`, `/readyz`, and
  `/metrics` (default `:8081`).
- HTTP_TIMEOUT_SECONDS: timeout for POSTs to kubeOP (default `15`).
- LOG_* variables: follow the API server behaviour for log storage.

Watcher auto-deployment (API server)
------------------------------------

- WATCHER_AUTO_DEPLOY: when `true`, kubeOP will deploy/manage the watcher after
  cluster registration. Defaults to `true` only when `PUBLIC_URL` is set; remains
  `false` for local/offline installs unless overridden.
- WATCHER_URL: Base URL for the kubeOP API the watcher should contact. Defaults
  to `PUBLIC_URL` when set and accepts either HTTP or HTTPS with custom ports.
- WATCHER_TOKEN: Optional override for the watcher bearer token. When omitted
  kubeOP signs a per-cluster JWT using `ADMIN_JWT_SECRET` and stores only a
  SHA-256 fingerprint in the Secret metadata.
- WATCHER_NAMESPACE: namespace where the watcher resources are created
  (default `kubeop-system`).
- WATCHER_NAMESPACE_CREATE: set `true` to create the namespace automatically if
  it does not already exist (default `true`).
- WATCHER_DEPLOYMENT_NAME / WATCHER_SERVICE_ACCOUNT / WATCHER_SECRET_NAME /
  WATCHER_PVC_NAME: override the default resource names (`kubeop-watcher`,
  `kubeop-watcher-state`).
- WATCHER_PVC_STORAGE_CLASS / WATCHER_PVC_SIZE: configure persistent storage
  (leave size empty to fall back to `emptyDir`).
- WATCHER_IMAGE: watcher container image (default
  `ghcr.io/vaheed/kubeop:watcher`).
- WATCHER_BATCH_MAX / WATCHER_BATCH_WINDOW_MS / WATCHER_STORE_PATH /
  WATCHER_HEARTBEAT_MINUTES: propagate batching and heartbeat tuning to the
  deployed pod.
- kubeOP persists watcher status transitions (`Pending`, `Deploying`, `Ready`,
  `Failed`) per cluster and flags clusters as unhealthy when the watcher misses
  a three minute readiness deadline while continuing to retry the rollout in
  the background.
- WATCHER_WAIT_FOR_READY: when `true` (default), kubeOP waits for the watcher
  Deployment to report at least one available replica before returning.
- WATCHER_READY_TIMEOUT_SECONDS: readiness deadline for the deployment check
  (default `180`).

Examples

- Local DSN: `postgres://postgres:postgres@localhost:5432/kubeop?sslmode=disable`
- Docker Compose DSN: `postgres://postgres:postgres@postgres:5432/kubeop?sslmode=disable`

Notes for Future Phases

- Domain/ingress and SSO variables will be introduced when the UI and public endpoints are added.
