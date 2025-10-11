Environment Variables

- APP_ENV: application environment (default `development`).
- PORT: HTTP port (default `8080`).
- LOG_LEVEL: `debug|info|warn|error` (default `info`).
- DISABLE_AUTH: bypass admin JWT middleware (default `false`).
- DATABASE_URL: Postgres DSN, e.g., `postgres://user:pass@host:5432/kubeop?sslmode=disable`.
- ADMIN_JWT_SECRET: HMAC secret for admin JWTs (required unless `DISABLE_AUTH=true`).
- KCFG_ENCRYPTION_KEY: key for AES-GCM at-rest encryption. Accepts Base64 or hex; otherwise SHA-256 of literal string is used.
- CONFIG_FILE: optional path to YAML file with defaults. Values from env override file.

Examples

- Local DSN: `postgres://postgres:postgres@localhost:5432/kubeop?sslmode=disable`
- Docker Compose DSN: `postgres://postgres:postgres@postgres:5432/kubeop?sslmode=disable`

Notes for Future Phases

- Domain/ingress and SSO variables will be introduced when the UI and public endpoints are added.

