Operations

Running

- Docker: `docker compose up -d --build` starts API + Postgres.
- Local: set env (see `.env.example`) and run `go run ./cmd/api`.

Logs

- Structured JSON via zap: stdout plus rotating files under `LOG_DIR` (`/var/log/kubeop` by default). Rotation is handled by lumberjack (`LOG_MAX_*`, `LOG_COMPRESS`). Project-level logs live under `${LOGS_ROOT}/projects/<project_id>/` with per-app `app.log`/`app.err.log`.
- Access logs (`msg="http_request"`) include `request_id`, method/path, status, latency (ms), remote IP, and byte counters. The response header `X-Request-Id` mirrors the logged value for correlating downstream systems.
- Audit logs (`audit.log`) capture mutating verbs, resources, tenant/user hints, client IP, and HTTP status. Secrets/tokens/passwords are automatically redacted in both paths and error reasons.
- Send `SIGHUP` to the process (or `docker compose kill -s HUP kubeop-api`) to reopen log files after external rotation or permission changes.
- Scheduler ticks emit `msg="cluster health"`/`"cluster health tick complete"` with healthy/unhealthy counters; investigate WARN entries immediately preceding summaries for failing clusters.

Migrations

- SQL files in `internal/store/migrations` are embedded and run automatically on startup using golang-migrate with `iofs`.
- Use golang-migrate naming: `NNNN_description.up.sql` and `NNNN_description.down.sql`.
- If a deployment fails mid-migration and leaves Postgres "dirty", run `migrate force <version>` (matching the version in the error) or reset the database volume before restarting the API. Version 0.3.7 and later log this hint automatically.

Backups

- Use standard Postgres tooling (e.g., `pg_dump` for logical backups). Ensure backups are encrypted and access-controlled.

Scaling

- The API is stateless; scale horizontally behind a load balancer. Use a managed Postgres with sufficient connections and CPU.
- Add a reverse proxy (e.g., NGINX/Envoy) for TLS and rate limiting.

Health & Readiness

- `GET /healthz` returns basic liveness.
- `GET /readyz` verifies DB connectivity; returns 503 with `{"status":"not_ready","error":"service unavailable"}` until the service layer and database respond. Each probe emits structured logs (`readyz status=...`) for dashboards.

Cluster Health Scheduler

- Interval controlled by `CLUSTER_HEALTH_INTERVAL_SECONDS` (default 60s). Each tick is bounded to 20s per cluster.
- Logs include per-cluster entries (`level=INFO|WARN msg="cluster health" ...`) followed by an aggregate summary. Use these to detect long-running probes or failing clusters.
- If ticks overrun the interval, reduce target cluster count per instance or increase interval; roadmap includes Prometheus metrics for automation.
- Troubleshooting checklist:
  1. Confirm scheduler tick logs continue at the configured interval.
  2. For repeated WARN entries, fetch `/v1/clusters/{id}/health` and audit cluster credentials.
  3. If ticks stall entirely, restart the pod/binary after verifying database connectivity; scheduler honours context cancellation on shutdown.

Configuration

- All runtime config is via environment variables. Optionally provide a YAML file at `CONFIG_FILE` for defaults (env wins).
 - Compose loads `.env` by default (see `docker-compose.yml` with `env_file: .env`).

Documentation Site (Docsify + GitHub Pages)

- Structure:
  - Site entry at `docs/index.html` (Docsify).
  - All Markdown content lives in `docs/` (repo rule); index and nav files are alongside.
- Automated publish (recommended):
  - A GitHub Action at `.github/workflows/docs-publish.yml` publishes the contents of `docs/` to the `gh-pages` branch on push to `main`.
  - In repository Settings → Pages, set Source to branch `gh-pages` and select `/ (root)`.
  - Your site will be available at `https://<org-or-user>.github.io/<repo>/`.
  - No Jekyll: the action sets `enable_jekyll: false` so `_sidebar.md` and `_navbar.md` are served.
- Manual publish (alternative):
  - In Settings → Pages, set Source: branch `main`, folder `/docs`.
  - Ensure `docs/.nojekyll` exists so GitHub Pages does not drop `_sidebar.md` and `_navbar.md`.
  - Remove or disable the custom “Publish Docs (GitHub Pages)” action to avoid confusion.

- Local preview:
  - Docker: `docker run -it --rm -p 3000:3000 -v "$PWD":/site -w /site/docs node:20 npx docsify serve .`
  - Node: `cd docs && npx docsify serve .` then open `http://localhost:3000`.

Permissions

- KubeOP performs reconciliation via Kubernetes Server-Side Apply (SSA). Ensure the API service identity (or your kubeconfig when running locally) has RBAC to create/patch the following resources in target namespaces:
  - `namespaces`, `resourcequotas`, `limitranges`, `serviceaccounts`, `roles`, `rolebindings`, `networkpolicies`.
  - TokenRequest (`create` on subresource `serviceaccounts/token`).

Observability (Future)

- Add structured audit events, Prometheus metrics, tracing, and request ID propagation.
