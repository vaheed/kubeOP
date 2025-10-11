Operations

Running

- Docker: `docker compose up -d --build` starts API + Postgres.
- Local: set env (see `.env.example`) and run `go run ./cmd/api`.

Logs

- Structured JSON via Go slog. Control with `LOG_LEVEL` (debug|info|warn|error).

Migrations

- SQL files in `internal/store/migrations` are embedded and run automatically on startup using golang-migrate with `iofs`.
- Use golang-migrate naming: `NNNN_description.up.sql` and `NNNN_description.down.sql`.

Backups

- Use standard Postgres tooling (e.g., `pg_dump` for logical backups). Ensure backups are encrypted and access-controlled.

Scaling

- The API is stateless; scale horizontally behind a load balancer. Use a managed Postgres with sufficient connections and CPU.
- Add a reverse proxy (e.g., NGINX/Envoy) for TLS and rate limiting.

Health & Readiness

- `GET /healthz` returns basic liveness.
- `GET /readyz` verifies DB connectivity; returns 503 if not ready.

Configuration

- All runtime config is via environment variables. Optionally provide a YAML file at `CONFIG_FILE` for defaults (env wins).
 - Compose loads `.env` by default (see `docker-compose.yml` with `env_file: .env`).

Permissions

- KubeOP performs reconciliation via Kubernetes Server-Side Apply (SSA). Ensure the API service identity (or your kubeconfig when running locally) has RBAC to create/patch the following resources in target namespaces:
  - `namespaces`, `resourcequotas`, `limitranges`, `serviceaccounts`, `roles`, `rolebindings`, `networkpolicies`.
  - TokenRequest (`create` on subresource `serviceaccounts/token`).

Observability (Future)

- Add structured audit events, Prometheus metrics, tracing, and request ID propagation.
