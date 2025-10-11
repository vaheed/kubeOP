Environment Variables

- APP_ENV: application environment (default `development`).
- PORT: HTTP port (default `8080`).
- LOG_LEVEL: `debug|info|warn|error` (default `info`).
- DISABLE_AUTH: bypass admin JWT middleware (default `false`).
- DATABASE_URL: Postgres DSN, e.g., `postgres://user:pass@host:5432/kubeop?sslmode=disable`.
- ADMIN_JWT_SECRET: HMAC secret for admin JWTs (required unless `DISABLE_AUTH=true`).
- KCFG_ENCRYPTION_KEY: key for AES-GCM at-rest encryption. Accepts Base64 or hex; otherwise SHA-256 of literal string is used.
- CONFIG_FILE: optional path to YAML file with defaults. Values from env override file.

Tenancy / Projects

- PROJECTS_IN_USER_NAMESPACE: if `true`, projects live in a user namespace (shared mode). Default is `true` (one user, many projects). If `false`, each project gets its own namespace and receives a kubeconfig.
- POD_SECURITY_LEVEL: Pod Security Admission level label for namespaces (default `restricted`).
- DNS_NS_LABEL_KEY / DNS_NS_LABEL_VALUE: label selector for the DNS namespace (defaults `kubernetes.io/metadata.name=kube-system`).
- DNS_POD_LABEL_KEY / DNS_POD_LABEL_VALUE: label selector for DNS pods (defaults `k8s-app=kube-dns`).
- INGRESS_NS_LABEL_KEY / INGRESS_NS_LABEL_VALUE: label selector to allow ingress traffic from selected namespaces (default `kubeop.io/ingress=true`).
- SA_TOKEN_TTL_SECONDS: service account token TTL for generated kubeconfigs (default `3600`).

Quotas and Limits

- DEFAULT_QUOTA_LIMITS_MEMORY, DEFAULT_QUOTA_LIMITS_CPU, DEFAULT_QUOTA_EPHEMERAL_STORAGE, DEFAULT_QUOTA_PVC_STORAGE, DEFAULT_QUOTA_MAX_PODS: namespace-level ResourceQuota defaults.
- DEFAULT_LR_REQUEST_CPU, DEFAULT_LR_REQUEST_MEMORY, DEFAULT_LR_LIMIT_CPU, DEFAULT_LR_LIMIT_MEMORY: namespace-level LimitRange defaults.
- PROJECT_LR_REQUEST_CPU, PROJECT_LR_REQUEST_MEMORY, PROJECT_LR_LIMIT_CPU, PROJECT_LR_LIMIT_MEMORY: per-project LimitRange defaults (should be <= namespace defaults). If not set, they fall back to the namespace defaults above.

Scheduler

- CLUSTER_HEALTH_INTERVAL_SECONDS: interval in seconds for logging cluster health in the background (default `60`).

Examples

- Local DSN: `postgres://postgres:postgres@localhost:5432/kubeop?sslmode=disable`
- Docker Compose DSN: `postgres://postgres:postgres@postgres:5432/kubeop?sslmode=disable`

Notes for Future Phases

- Domain/ingress and SSO variables will be introduced when the UI and public endpoints are added.
