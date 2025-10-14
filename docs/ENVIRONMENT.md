Environment Variables

- APP_ENV: application environment (default `development`).
- PORT: HTTP port (default `8080`).
- LOG_LEVEL: `debug|info|warn|error` (default `info`). Controls zap logging level for stdout/app.log.
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
- SA_TOKEN_TTL_SECONDS: deprecated; kubeconfigs now use non-expiring ServiceAccount token Secrets. The variable is accepted for backward compatibility but ignored.

Quotas and Limits

- DEFAULT_QUOTA_LIMITS_MEMORY, DEFAULT_QUOTA_LIMITS_CPU, DEFAULT_QUOTA_EPHEMERAL_STORAGE, DEFAULT_QUOTA_PVC_STORAGE, DEFAULT_QUOTA_MAX_PODS: namespace-level ResourceQuota defaults.
- DEFAULT_LR_REQUEST_CPU, DEFAULT_LR_REQUEST_MEMORY, DEFAULT_LR_LIMIT_CPU, DEFAULT_LR_LIMIT_MEMORY: namespace-level LimitRange defaults.
- PROJECT_LR_REQUEST_CPU, PROJECT_LR_REQUEST_MEMORY, PROJECT_LR_LIMIT_CPU, PROJECT_LR_LIMIT_MEMORY: per-project LimitRange defaults (should be <= namespace defaults). If not set, they fall back to the namespace defaults above.

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
  - KubeOP polls asynchronously for the Service load balancer IP before creating or updating the Cloudflare record, logging `dns_wait_for_load_balancer_ip` while waiting.
- PowerDNS:
  - PDNS_API_URL: e.g., `http://pdns:8081`.
  - PDNS_API_KEY: X-API-Key value.
  - PDNS_SERVER_ID: server identifier (default `localhost`).
  - PDNS_ZONE: Zone name; defaults to `PAAS_DOMAIN` if empty.

Examples

- Local DSN: `postgres://postgres:postgres@localhost:5432/kubeop?sslmode=disable`
- Docker Compose DSN: `postgres://postgres:postgres@postgres:5432/kubeop?sslmode=disable`

Notes for Future Phases

- Domain/ingress and SSO variables will be introduced when the UI and public endpoints are added.
