KubeOP — Out-of-Cluster Control Plane (Go)

Overview

- Production-ready starter for an out-of-cluster control plane in Go.
- Manages multiple Kubernetes clusters via uploaded kubeconfigs.
- Exposes a REST API on port 8080.
- Persists state in PostgreSQL (users, clusters, projects).
- Secured with an admin JWT and at-rest encryption for kubeconfigs.
- Supports app deployments (image/manifests/helm), flavors, CI webhooks, logs streaming, Prometheus metrics, config/secret attachment endpoints, and ENV-driven ingress/LB (MetalLB default).
- 0.3.11 hardens disk-backed project/app logs by sanitizing project/app identifiers so directories stay under `${LOGS_ROOT}`.
- 0.3.8 switches the default Pod Security Admission level to `baseline`, keeping privilege escalation disabled while letting common images (e.g., `nginx:1.27`) run without custom manifests.
- 0.3.7 fixes soft-delete migrations for fresh installs, adds dirty-database recovery guidance, and surfaces clearer migration error logging.
- 0.3.1 hardens readiness reporting when dependencies are unavailable, deduplicates kubeconfig parsing helpers, and refreshes documentation/roadmap guidance for production onboarding.

What's new in 0.3.11

- Project and app log directories now trim whitespace and reject path separators so log writers cannot escape `${LOGS_ROOT}`.
- Documentation and tests cover the sanitization behaviour; valid IDs continue to be emitted in log fields for correlation.

What's new in 0.3.10

- Projects now write aggregated (`project.log`) and event (`events.jsonl`) logs plus per-app `app.log`/`app.err.log` under `${LOGS_ROOT}/projects/<project_id>/apps/<app_id>/`.
- `docker-compose` mounts `./logs:/var/log/kubeop` and startup guarantees directories exist for every stored project/app so operators can tail logs immediately after `docker compose up`.
- Sensitive key/value pairs matching `password|token|secret|apikey|authorization` are redacted at write-time while preserving JSON structure; redaction applies to stdout, control-plane files, and project/app logs.

What's new in 0.3.9

- JSON logs now flow to both stdout and `/var/log/kubeop/app.log` with RFC3339Nano timestamps, service/version metadata, and request-scoped fields (`request_id`, latency, caller IP, bytes in/out).
- Mutating requests emit audit events (`/var/log/kubeop/audit.log`) capturing verb, resource, tenant/user hints, and redacted identifiers; secrets/tokens/passwords are automatically scrubbed.
- Compose mounts `./logs` from the host and exports `LOGS_ROOT=/var/log/kubeop`; SIGHUP triggers in-process log rotation rebuilds without downtime.
- Request IDs are returned via `X-Request-Id`; use them to join access, audit, and downstream logs.

What's new in 0.3.8

- Default Pod Security Admission level is now `baseline`, so popular Docker Hub images run without additional security overrides.
- Container security defaults follow the configured Pod Security level: `restricted` enforces non-root/read-only/drop-all, while
  other levels keep privilege escalation disabled and retain runtime/default seccomp without mutating user or filesystem.
- Quickstarts and app docs now outline when to pick `baseline` vs `restricted` with copy-ready curl examples for each mode.

What's new in 0.3.7

- Soft-delete migrations now apply cleanly on PostgreSQL 16+ by removing unsupported `ALTER TABLE IF NOT EXISTS` syntax.
- The API logs actionable instructions when a database is left dirty (e.g., `migrate force <version>` or reset) so operators can recover quickly.
- README and operations docs now document how to reset the database or force migrations after a failed deploy.
- Release metadata lives in `internal/version/version.go`; run `gofmt` after editing the struct-style block so CI accepts the change.

What's new in 0.3.1

- `/readyz` now fails fast with a 503 and explicit `service unavailable` message if the API is started without a service layer (or if dependencies are not wired yet), preventing nil dereferences and aiding smoke tests.
- Added structured readiness logging (`status=service_missing|health_check_failed|ready`) to make dashboards and CI diagnostics clearer.
- Consolidated kubeconfig YAML scalar parsing into a single helper with white-box tests to avoid drift between server/CA extraction logic.
- Expanded documentation plan, roadmap next steps, and README quickstart guidance for operators bootstrapping new environments.

What's new in 0.3.0

- Shared scheduler helper keeps cluster health checks bounded per tick, emits summaries, and is covered by targeted tests.
- Tenant NetworkPolicy/RBAC manifests now originate from shared builders, removing drift between user bootstrap and project creation flows.
- Docs refreshed for production readiness (README, Architecture, API, Environment, Operations, Security) and a documentation plan published at `docs/DOCUMENTATION_PLAN.md`.

Before you begin

1. Install Docker and Docker Compose (or run everything locally with Go + Postgres).
2. Clone the repository and copy `.env.example` to `.env` if you need to override defaults.
3. Generate an admin JWT signed with `ADMIN_JWT_SECRET` and claim `{ "role": "admin" }` for API requests.
4. Export helper variables for curl commands:
   ```bash
   export TOKEN="<admin-jwt>"
   export AUTH_H="-H 'Authorization: Bearer $TOKEN'"
   ```

Quickstart (5-step path)

1. **Start the stack**
   ```bash
   mkdir -p logs
   docker compose up -d --build
   ```
   *Compose now mounts `./logs:/var/log/kubeop`; after the API starts you should see `logs/projects/` seeded with existing projects/apps. IDs containing `/` or `\` are rejected when log directories are provisioned.*
2. **Check health**
   ```bash
   curl http://localhost:8080/healthz
   curl http://localhost:8080/readyz     # returns 503 with {"status":"not_ready"} until DB is reachable
   curl $AUTH_H http://localhost:8080/v1/version
   ```
3. **Register a cluster (base64 kubeconfig required)**
   ```bash
   B64=$(base64 -w0 < kubeconfig)                     # macOS/Linux
   # Windows PowerShell: $B64 = [Convert]::ToBase64String([IO.File]::ReadAllBytes('kubeconfig'))
   curl -s $AUTH_H -H 'Content-Type: application/json' \
     -d "$(jq -n --arg name 'talos-stage' --arg b64 "$B64" '{name:$name,kubeconfig_b64:$b64}')" \
     http://localhost:8080/v1/clusters
   ```
4. **Bootstrap a user namespace (shared mode default)**
   ```bash
   curl -s $AUTH_H -H 'Content-Type: application/json' \
     -d '{"name":"Alice","email":"alice@example.com","clusterId":"<cluster-id>"}' \
     http://localhost:8080/v1/users/bootstrap
   ```
   *Save `user.id`, `namespace`, and decode `kubeconfig_b64` to `user.kubeconfig` for kubectl access.*
5. **Create a project and deploy an app**
   ```bash
   curl -s $AUTH_H -H 'Content-Type: application/json' \
     -d '{"userId":"<user-id>","clusterId":"<cluster-id>","name":"demo"}' \
     http://localhost:8080/v1/projects

   curl -s $AUTH_H -H 'Content-Type: application/json' \
     -d '{"name":"web","image":"nginx:1.27","ports":[{"containerPort":80,"servicePort":80,"serviceType":"LoadBalancer"}]}' \
     http://localhost:8080/v1/projects/<project-id>/apps
   ```
   *Default installs use the `baseline` Pod Security level so common images run without tweaks. If you set `POD_SECURITY_LEVEL=restricted`, use an unprivileged image (e.g., `nginxinc/nginx-unprivileged`) and listen on a high container port.*
   *Access via wildcard ingress (`http://web.<namespace>.<PAAS_DOMAIN>`) or run `KUBECONFIG=./user.kubeconfig kubectl -n <namespace> get svc web -o wide` to find the external IP.*
API walk-through

- Follow `docs/QUICKSTART_API.md` for a scripted flow that covers creating/deleting users, projects, and apps with copy-ready commands.
- `docs/QUICKSTART_APPS.md` focuses on app deployments (image/helm/git) and includes log and access examples.

Config & Secret attachments (step-by-step)

1. **Create a ConfigMap or Secret** in the project namespace via kubectl or the `/v1/projects/{id}/configs|secrets` APIs.
2. **Attach all keys**
   ```bash
   curl -s $AUTH_H -H 'Content-Type: application/json' \
     -d '{"name":"app-config"}' \
     http://localhost:8080/v1/projects/<project-id>/apps/<app-id>/configs/attach
   ```
3. **Attach specific keys with an optional prefix**
   ```bash
   curl -s $AUTH_H -H 'Content-Type: application/json' \
     -d '{"name":"app-config","keys":["LOG_LEVEL"],"prefix":"APP_"}' \
     http://localhost:8080/v1/projects/<project-id>/apps/<app-id>/configs/attach
   ```
4. **Attach secrets the same way** using `/secrets/attach`.
5. **Detach when finished**; this removes `envFrom` and keyed env vars so pods restart cleanly.
   ```bash
   curl -s $AUTH_H -H 'Content-Type: application/json' \
     -d '{"name":"app-config"}' \
     http://localhost:8080/v1/projects/<project-id>/apps/<app-id>/configs/detach
   ```
   *Secrets detach via `/secrets/detach`.*

Auth essentials

1. Set `ADMIN_JWT_SECRET` in the environment for both the API and any tooling generating admin tokens.
2. Sign tokens with `HS256` and include the claim `{ "role": "admin" }`.
3. For development-only testing, export `DISABLE_AUTH=true` to skip auth entirely.
Tenancy cheat sheet

- **Shared user namespace (default, `PROJECTS_IN_USER_NAMESPACE=true`)**
  1. Register cluster → `clusterId`
  2. Bootstrap user → decode `kubeconfig_b64` to `user.kubeconfig`
  3. Create projects with `{ userId, clusterId, name }` → reuse the user kubeconfig for kubectl
  4. Manage quotas at the namespace level

- **Per-project namespaces (`PROJECTS_IN_USER_NAMESPACE=false`)**
  1. Register cluster → `clusterId`
  2. Create project with user reference → response includes project-scoped `kubeconfig_b64`
  3. Use `/quota`, `/suspend`, `/unsuspend` to control each namespace independently

*Kubeconfigs returned from bootstrap/renew flows now use a sanitized, human-readable user label derived from the display name or email and keep that label stable on renewals for a friendlier `kubectl config get-contexts` view.*

Everyday curl references

- List users: `curl -s $AUTH_H http://localhost:8080/v1/users | jq`
- List clusters: `curl -s $AUTH_H http://localhost:8080/v1/clusters | jq`
- List projects: `curl -s $AUTH_H http://localhost:8080/v1/projects | jq`
- List a user’s projects: `curl -s $AUTH_H http://localhost:8080/v1/users/<user-id>/projects | jq`

Local development (Go without Docker)

1. Start Postgres (see `docker-compose.yml` for default credentials) or point `DATABASE_URL` to a running instance.
2. Export env vars or load `.env`.
3. Install dependencies and run the API:
   ```bash
   go mod download
   go run ./cmd/api
   ```

Operational notes

- Talos support: any CNCF-compliant cluster works via kubeconfig upload; Talos is tested today.

Logging & audit trail

- **Default location**: JSON logs stream to stdout and `/var/log/kubeop/app.log`; audit events land in `/var/log/kubeop/audit.log` when `AUDIT_ENABLED=true`. Per-project logs live under `${LOGS_ROOT}/projects/<project_id>/` where `<project_id>`/`<app_id>` are trimmed and must not contain `/` or `\`.
- **Environment variables**

  | Variable | Default | Purpose |
  | --- | --- | --- |
  | `LOG_LEVEL` | `info` | Minimum level for application logs (`debug`, `info`, `warn`, `error`). |
  | `LOGS_ROOT` | `/var/log/kubeop` | Root directory for project/app logs (`project.log`, `events.jsonl`, per-app log/err files). Project/app IDs are sanitized and all log paths are normalised so traversal attempts fail before touching disk. |
  | `LOG_DIR` | `LOGS_ROOT` | Directory containing control-plane `app.log` and `audit.log` (falls back to `LOGS_ROOT`). |
  | `LOG_MAX_SIZE_MB` | `50` | Rotate after this many megabytes per file. |
  | `LOG_MAX_BACKUPS` | `7` | Number of old log files to retain. |
  | `LOG_MAX_AGE_DAYS` | `14` | Days to retain rotated files. |
  | `LOG_COMPRESS` | `true` | Gzip rotated files when `true`. |
  | `AUDIT_ENABLED` | `true` | Toggle security audit events for mutating requests. |

- **.env example**

  ```env
  LOG_LEVEL=info
  LOGS_ROOT=/var/log/kubeop
  LOG_DIR=/var/log/kubeop
  LOG_MAX_SIZE_MB=50
  LOG_MAX_BACKUPS=7
  LOG_MAX_AGE_DAYS=14
  LOG_COMPRESS=true
  AUDIT_ENABLED=true
  ```

- **Inspecting logs with jq**

  ```bash
  # Tail structured access logs, scoped to API requests
  jq 'select(.msg=="http_request") | {ts,request_id,path,status,latency_ms}' logs/app.log

  # List recent audit events for project mutations
  jq 'select(.msg=="audit" and .resource=="projects") | {when,request_id,resource_id,verb,status}' logs/audit.log
  ```

- **Project/app logs**: aggregated project activity lives in `logs/projects/<project_id>/project.log`; events emit to `events.jsonl`; app stdout/stderr go to `logs/projects/<project_id>/apps/<app_id>/app.log` and `app.err.log`.

- Send `SIGHUP` to the API container/process (`docker compose kill -s HUP kubeop-api`) to reopen log files after external rotation or permission changes.
- Configuration: all settings are environment-driven; optionally point `CONFIG_FILE` at a YAML overlay.
- Migrations: embedded migrations run automatically on startup.
- Cluster health scheduler logs start/stop events and honours shutdown signals to keep background checks predictable during deploys.
- Readiness endpoint emits structured logs (`status=service_missing|health_check_failed|ready`) so CI and dashboards can alert on degraded dependencies quickly.
- Helm chart downloads are restricted to `http(s)` URLs on public hosts, must resolve to globally routable addresses, are limited to default ports (`80`/`443`), and are fetched via a dedicated HTTP client that blocks cross-host redirects, enforces the validated host header, and only dials the pre-approved addresses to avoid SSRF or request forgery.
Documentation map

- docs/ARCHITECTURE.md — System diagram, package layout, and data flow.
- docs/DOCUMENTATION_PLAN.md — Living inventory of docs, audiences, gaps, and upcoming deliverables.
- docs/CONTRIBUTING.md — Local setup, checks, and pull request expectations.
- docs/API_REFERENCE.md — REST endpoints with numbered walkthroughs and curl snippets.
- docs/QUICKSTART_API.md — Copy-ready flow: register cluster → bootstrap user → create project/app → clean up.
- docs/QUICKSTART_APPS.md — App-centric quickstart (image, Helm, Git) plus attachment walkthrough.
- docs/CODE_OF_CONDUCT.md — Community expectations and enforcement guidelines.
- docs/APPS.md — Deep dive into deployment options, app management, and config/secret handling.
- docs/ENVIRONMENT.md — Environment variables with defaults and suggested values.
- docs/OPERATIONS.md — Running locally, via Docker Compose, maintenance, migrations, backups, scaling, and health checks.
- docs/SECURITY.md — JWT model, encryption-at-rest, rotation guidance, and hardening tips.
- docs/ROADMAP.md — Ordered phases with explicit deliverables.
- docs/KUBECONFIG.md — How namespace-scoped kubeconfigs are minted and returned base64.
- docs/TENANCY.md — User → Project → Namespace lifecycle with env knobs.
- docs/ISOLATION.md — NetworkPolicy defaults and PSA expectations.
- docs/QUOTAS.md — Default quotas and override workflow.
- docs/FLAVORS.md — Built-in flavors and override guidance.
- docs/INGRESS_LB.md — Wildcard ingress, MetalLB settings, and DNS automation.
- docs/CI_WEBHOOKS.md — Git webhook configuration and payload schema.
- docs/METRICS.md — `/metrics` output and scraping tips.
- docs/CHANGELOG.md — Release history (Keep a Changelog).
- docs/openapi.yaml — OpenAPI spec (view via `docs/openapi.html` or import to an API client).

Project rules

- Review AGENTS.md for repository-wide coding, docs, and testing requirements before submitting changes.

Testing

- Unit tests live under `testcase/`.
- Run locally: `go test ./...`
- CI (`.github/workflows/ci.yml`) runs `go vet`, `go build`, `go test ./...`, and uploads the compiled API binary on every push and PR.

License

- MIT License — see `LICENSE` for the full text.

Kubeconfig base64 helpers

- The API only accepts `kubeconfig_b64`.
- macOS/Linux: `base64 -w0 < kubeconfig`
- Windows PowerShell: `[Convert]::ToBase64String([IO.File]::ReadAllBytes('kubeconfig'))`
