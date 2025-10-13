# KubeOP

KubeOP is an out-of-cluster control plane that lets operators manage multiple Kubernetes clusters from a single API. It focuses on secure multi-tenancy, predictable automation, and observability without running controllers inside the target clusters.

- Production-ready starter for an out-of-cluster control plane in Go.
- Manages multiple Kubernetes clusters via uploaded kubeconfigs.
- Exposes a REST API on port 8080.
- Persists state in PostgreSQL (users, clusters, projects).
- Secured with an admin JWT and at-rest encryption for kubeconfigs.
- Supports app deployments (image/manifests/helm), flavors, CI webhooks, logs streaming, Prometheus metrics, config/secret attachment endpoints, and ENV-driven ingress/LB (MetalLB default).
- 0.3.16 hardens log directory creation with `filepath.Rel` checks so CodeQL recognises that every file touched stays rooted under `${LOGS_ROOT}`.
- 0.3.13 enforces ASCII-safe (`[A-Za-z0-9._-]`) project and app identifiers for disk-backed logs so paths stay under `${LOGS_ROOT}` while keeping log metadata intact.
- 0.3.8 switches the default Pod Security Admission level to `baseline`, keeping privilege escalation disabled while letting common images (e.g., `nginx:1.27`) run without custom manifests.
- 0.3.7 fixes soft-delete migrations for fresh installs, adds dirty-database recovery guidance, and surfaces clearer migration error logging.
- 0.3.1 hardens readiness reporting when dependencies are unavailable, deduplicates kubeconfig parsing helpers, and refreshes documentation/roadmap guidance for production onboarding.

What's new in 0.3.16

- File-manager directory creation now re-validates every parent with `filepath.Rel` so CodeQL sees writes anchored to `${LOGS_ROOT}` and rejects traversal attempts.
- Test helpers cover traversal edge cases by exercising `TouchLogFileForTest` with invalid identifiers, preventing regressions from bypassing sanitisation logic.
- Documentation and release metadata reflect the stricter guards so operators understand that behaviour is unchanged while safety improves.

What's new in 0.3.15

- File-manager helpers now normalise log file creation through `${LOGS_ROOT}` joins, closing remaining path traversal alerts detected by CodeQL.
- Test-only log file helpers accept a root plus segments to mirror production usage, ensuring absolute paths are derived from sanitised identifiers before touching disk.
- Documentation, changelog, and version metadata note the tightened helpers so operators know the log layout remains unchanged while validation improves.

What's new in 0.3.13

## Architecture at a glance

KubeOP exposes a REST API (default `:8080`) built with Go and `chi`, backed by PostgreSQL via `pgx`. A background scheduler performs cluster health checks and asynchronous tasks. Logging uses `zap` with `lumberjack` rotation, and Helm interactions leverage `helm.sh/helm/v3`. All state and configuration is driven through environment variables so the control plane can run in Docker or as a standalone binary.

```
+-------------+        +--------------------+        +------------------+
| API client  |  --->  | KubeOP REST API    |  --->  | Target clusters  |
| (curl/CI)   |        | (auth, tenancy,    |        | (Talos/any K8s)  |
+-------------+        | deployments, logs) |        +------------------+
                         |       |
                         v       v
                     PostgreSQL  Logs & metrics
```

See [`docs/ARCHITECTURE.md`](docs/ARCHITECTURE.md) for the full component walkthrough and sequence diagrams.

## Prerequisites

- Docker and Docker Compose **or** Go 1.22+ with access to PostgreSQL 14+.
- `make`, `jq`, and `base64` utilities for the quickest workflows.
- An admin JWT signed with `ADMIN_JWT_SECRET` containing `{"role":"admin"}` when calling privileged endpoints.

## Quickstart: Docker Compose

1. **Clone and prepare**
   ```bash
   git clone https://github.com/vaheed/kubeOP.git
   cd kubeOP
   cp .env.example .env   # optional overrides
   mkdir -p logs
   ```
2. **Launch the stack**
   ```bash
   docker compose up -d --build
   ```
   Logs stream to `./logs`; the API listens on `http://localhost:8080`.
3. **Check health**
   ```bash
   curl http://localhost:8080/healthz
   curl http://localhost:8080/readyz
   ```
4. **Authenticate**
   ```bash
   export TOKEN="<admin-jwt>"
   export AUTH_H="-H 'Authorization: Bearer $TOKEN'"
   ```
5. **Register a cluster (base64 kubeconfig only)**
   ```bash
   B64=$(base64 -w0 < kubeconfig)
   curl -s $AUTH_H -H 'Content-Type: application/json' \
     -d "$(jq -n --arg name 'talos-stage' --arg b64 "$B64" '{name:$name,kubeconfig_b64:$b64}')" \
     http://localhost:8080/v1/clusters | jq
   ```
6. **Bootstrap a user and project namespace**
   ```bash
   curl -s $AUTH_H -H 'Content-Type: application/json' \
     -d '{"name":"Alice","email":"alice@example.com","clusterId":"<cluster-id>"}' \
     http://localhost:8080/v1/users/bootstrap | jq
   ```

Additional walkthroughs live in [`docs/QUICKSTART_API.md`](docs/QUICKSTART_API.md) and [`docs/QUICKSTART_APPS.md`](docs/QUICKSTART_APPS.md).

## Running locally with Go

1. Start PostgreSQL (credentials in `docker-compose.yml`).
2. Export environment variables or load `.env`.
3. Install dependencies and run the API:
   ```bash
   go mod download
   go run ./cmd/api
   ```
4. Optional: build a static binary with version metadata
   ```bash
   make build VERSION=$(git describe --tags --always)
   ```

## Configuration

All configuration happens through environment variables. Core values include:

| Variable | Default | Description |
| --- | --- | --- |
| `DATABASE_URL` | `postgres://kubeop:kubeop@postgres:5432/kubeop?sslmode=disable` | PostgreSQL connection string. |
| `ADMIN_JWT_SECRET` | _none_ | HMAC secret used to validate admin tokens. |
| `LOG_LEVEL` | `info` | Minimum structured log level (`debug`, `info`, `warn`, `error`). |
| `LOGS_ROOT` | `/var/log/kubeop` | Root for project/app log directories. Identifiers must match `[A-Za-z0-9._-]+`. |
| `AUDIT_ENABLED` | `true` | Emit audit events for mutating requests. |
| `PROJECTS_IN_USER_NAMESPACE` | `true` | Scope projects to the owning user’s namespace by default. |
| `DISABLE_AUTH` | `false` | Bypass admin auth for development/testing only. |

A complete list and tuning guidance is available in [`docs/ENVIRONMENT.md`](docs/ENVIRONMENT.md).

## API essentials

- Base URL: `http://localhost:8080`
- Health probes: `/healthz`, `/readyz`
- Version metadata: `/v1/version`
- Core workflows:
  - `POST /v1/clusters` – register a cluster (requires `kubeconfig_b64`).
  - `POST /v1/users/bootstrap` – create user, namespace, default quotas, kubeconfig.
  - `POST /v1/projects` – create project scoped to a cluster/user namespace.
  - App deployments via `/v1/apps` (image, manifests, Helm) with optional CI webhooks.

Refer to [`docs/openapi.yaml`](docs/openapi.yaml) or [`docs/API_REFERENCE.md`](docs/API_REFERENCE.md) for schemas, request/response examples, and authentication details.

## Observability & logs

- Structured JSON logs go to stdout and `${LOGS_ROOT}/app.log` with `X-Request-Id` correlation.
- Audit events write to `${LOGS_ROOT}/audit.log` when enabled; sensitive fields are redacted.
- Project/app logs live under `${LOGS_ROOT}/projects/<project_id>/apps/<app_id>/` with safe identifier enforcement.
- Prometheus metrics served at `/metrics`.
- Send `SIGHUP` to the process to rotate file handles after external changes.

## Development workflow

Operational notes

- Talos support: any CNCF-compliant cluster works via kubeconfig upload; Talos is tested today.

Logging & audit trail

- **Default location**: JSON logs stream to stdout and `/var/log/kubeop/app.log`; audit events land in `/var/log/kubeop/audit.log` when `AUDIT_ENABLED=true`. Per-project logs live under `${LOGS_ROOT}/projects/<project_id>/` where `<project_id>`/`<app_id>` are trimmed and must match `[A-Za-z0-9._-]+`.
- **Environment variables**

  | Variable | Default | Purpose |
  | --- | --- | --- |
  | `LOG_LEVEL` | `info` | Minimum level for application logs (`debug`, `info`, `warn`, `error`). |
  | `LOGS_ROOT` | `/var/log/kubeop` | Root directory for project/app logs (`project.log`, `events.jsonl`, per-app log/err files). Project/app IDs are trimmed and must match `[A-Za-z0-9._-]+`; all joins are normalised so traversal attempts fail before touching disk and relative/unclean paths are rejected. |
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
- Static analysis and builds:
  ```bash
  go vet ./...
  go build ./cmd/api
  ```
- Make targets:
  - `make build` – produce a trimmed static binary with version metadata.
  - `make run` – start the API locally.
  - `make test` – execute the full Go test suite.

CI pipelines (see `.github/workflows/ci.yml`) install dependencies, lint with `go vet`, run tests, build the binary artifact, and publish documentation via Docsify (`docs/`).

## Repository layout

```
cmd/api/                  # Application entrypoint and HTTP wiring
internal/                 # Domain logic, services, logging, crypto, data access
internal/store/migrations # PostgreSQL schema migrations (golang-migrate format)
docs/                     # Extended documentation, API reference, changelog
samples/                  # Example manifests and payloads
testcase/                 # Go test suites (package-aligned)
```

Consult the documentation map in the `docs/` directory (e.g., `docs/OPERATIONS.md`, `docs/SECURITY.md`, `docs/ROADMAP.md`) for deeper dives.

## Contributing

1. Review [`docs/CONTRIBUTING.md`](docs/CONTRIBUTING.md) and repository rules in `AGENTS.md`.
2. Keep documentation up to date alongside code changes.
3. Run `go vet`, `go test ./...`, and ensure the Docker Compose stack still boots.
4. Include tests for new or changed functionality under `testcase/`.

## License

KubeOP is released under the [MIT License](LICENSE).
