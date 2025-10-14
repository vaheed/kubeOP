# KubeOP

KubeOP is an out-of-cluster control plane that lets operators manage multiple Kubernetes clusters from a single API. It focuses on secure multi-tenancy, predictable automation, and observability without running controllers inside the target clusters.

## Key capabilities

- **Multi-cluster management** – ingest kubeconfigs (base64 encoded) and orchestrate user, project, and application lifecycles across clusters.
- **Tenant automation** – bootstrap namespaces, NetworkPolicies, quotas, and credentials with one call while keeping projects scoped to the user namespace by default.
- **Application delivery** – deploy container images, raw manifests, or Helm charts, with CI webhook triggers and attachment endpoints for configs and secrets.
- **Security & auditing** – JWT-secured admin APIs, Pod Security Admission profiles, environment-driven hardening, and structured audit logs with redaction of sensitive fields.
- **Operational insight** – JSON logs, per-project/app log streams on disk with download APIs (`/v1/projects/{id}/logs`, `/v1/projects/{id}/apps/{appId}/logs`), `/metrics` for Prometheus, and health/readiness endpoints designed for fast smoke tests.
- **Event visibility** – Normalised project event feeds stored in PostgreSQL and `${LOGS_ROOT}/projects/<project_id>/events.jsonl`, filterable via the `/v1/projects/{id}/events` API and appendable for custom signals.
- **Watcher bridge** – Optional out-of-cluster watcher that streams filtered Kubernetes resource changes to `/v1/events/ingest`, keeping kubeOP project timelines aligned with cluster activity in near real-time.

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
7. **Mint or rotate kubeconfigs on demand**
   ```bash
   # Ensure or fetch an existing binding (user or project scope)
   curl -s $AUTH_H -H 'Content-Type: application/json' \
     -d '{"userId":"<user-id>","clusterId":"<cluster-id>"}' \
     http://localhost:8080/v1/kubeconfigs | jq

   # Rotate a binding by ID (returns a fresh token kubeconfig)
   curl -s $AUTH_H -H 'Content-Type: application/json' \
     -d '{"id":"<binding-id>"}' \
     http://localhost:8080/v1/kubeconfigs/rotate | jq
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
| `PUBLIC_URL` | _empty_ | External HTTPS endpoint for kubeOP. When set, watcher auto deployment turns on automatically unless overridden. |
| `LOG_LEVEL` | `info` | Minimum structured log level (`debug`, `info`, `warn`, `error`). |
| `LOGS_ROOT` | `/var/log/kubeop` | Root for project/app log directories. Identifiers must match `[A-Za-z0-9._-]+`. |
| `AUDIT_ENABLED` | `true` | Emit audit events for mutating requests. |
| `EVENTS_DB_ENABLED` | `true` | Persist project events to PostgreSQL in addition to disk-backed JSONL logs. Disable to operate in file-only mode. |
| `K8S_EVENTS_BRIDGE` | `false` | Enable ingestion of Kubernetes core/v1 Events into the project event stream when the bridge is deployed. |
| `PROJECTS_IN_USER_NAMESPACE` | `true` | Scope projects to the owning user’s namespace by default. |
| `DISABLE_AUTH` | `false` | Bypass admin auth for development/testing only. |

A complete list and tuning guidance is available in [`docs/ENVIRONMENT.md`](docs/ENVIRONMENT.md).

## External DNS automation

When `EXTERNAL_DNS_PROVIDER` is set (Cloudflare or PowerDNS), KubeOP watches for the published load balancer IP of each app Service before upserting the corresponding A record. Cloudflare automation polls asynchronously until an address is assigned, ensuring subdomain records are created even when the IP is provisioned after the initial deployment. Structured service logs (`dns_wait_for_load_balancer_ip`, `dns_record_upserted`) now include project, app, cluster, and host context for each step, and Cloudflare API error responses are surfaced verbatim so operators can triage DNS failures without reproducing requests manually.

## API essentials

- Base URL: `http://localhost:8080`
- Health probes: `/healthz`, `/readyz`
- Version metadata: `/v1/version`
- Core workflows:
  - `POST /v1/clusters` – register a cluster (requires `kubeconfig_b64`).
  - `POST /v1/users/bootstrap` – create user, namespace, default quotas, kubeconfig.
  - `POST /v1/projects` – create project scoped to a cluster/user namespace.
  - `POST /v1/kubeconfigs` – ensure or mint a namespace-scoped kubeconfig (user or project); rotate via `POST /v1/kubeconfigs/rotate` and revoke with `DELETE /v1/kubeconfigs/{id}`.
  - App deployments via `/v1/apps` (image, manifests, Helm) with optional CI webhooks.
  - Project event history via `GET /v1/projects/{id}/events` with filters for kind, severity, actor, search terms, cursor pagination, and custom append via `POST /v1/projects/{id}/events`.

Refer to [`docs/openapi.yaml`](docs/openapi.yaml) or [`docs/API_REFERENCE.md`](docs/API_REFERENCE.md) for schemas, request/response examples, and authentication details.

## Observability & logs

- Structured JSON logs go to stdout and `${LOGS_ROOT}/app.log` with `X-Request-Id` correlation.
- Audit events write to `${LOGS_ROOT}/audit.log` when enabled; sensitive fields are redacted.
- Project/app logs live under `${LOGS_ROOT}/projects/<project_id>/apps/<app_id>/` with safe identifier enforcement, and event streams replicate to `${LOGS_ROOT}/projects/<project_id>/events.jsonl` alongside the `/v1/projects/{id}/events` API.
- `GET /v1/projects/{id}/logs` accepts `tail` to return the most recent lines with a safeguard of 5,000 lines to prevent excessive memory usage when streaming from disk.
- Prometheus metrics served at `/metrics`, including the `readyz_failures_total` counter for alerting on repeated readiness probe issues. Import the sample Grafana board at [`docs/dashboards/readyz-grafana.json`](docs/dashboards/readyz-grafana.json) to visualize failures by reason and total volume.
- Send `SIGHUP` to the process to rotate file handles after external changes.
- Startup fails fast if logging cannot be initialised or database
  migrations do not complete, preventing the API from running in a
  partially configured state.

## kubeOP Watcher Bridge

The kubeOP watcher is a small Go binary/ container that tails Kubernetes
resources (Pods, Deployments, Services, Ingresses, Jobs, CronJobs, HPAs,
PVCs, ConfigMaps, Secrets, core/v1 Events, and cert-manager Certificates)
using shared informers and posts normalised events to
`/v1/events/ingest`. Only objects carrying the
`kubeop.project.id`/`kubeop.app.id`/`kubeop.tenant.id` labels are
forwarded, keeping tenant traffic scoped.

### Automatic deployment

Point kubeOP at its external HTTPS endpoint (for example,
`PUBLIC_URL=https://kubeop.example.com`). Without this value, watcher
auto-deployment stays disabled so local development or air-gapped installs do
not fail health checks when the ingest endpoint is unreachable. Once the public
URL is configured, watcher auto deployment is enabled by default: every new
cluster registration provisions the ServiceAccount, RBAC, Secret, storage, and
Deployment inside the managed cluster, waiting for readiness before the API
call returns. kubeOP signs a unique per-cluster bearer token using the admin JWT
secret and stores only a SHA-256 fingerprint alongside the secret data so
credentials never appear in logs.

Optional knobs (`WATCHER_NAMESPACE`, `WATCHER_IMAGE`, `WATCHER_PVC_SIZE`,
`WATCHER_BATCH_MAX`, `WATCHER_TOKEN` to force a static credential, etc.) mirror
the values documented in [`docs/ENVIRONMENT.md`](docs/ENVIRONMENT.md).

Health can be checked with:

```
kubectl -n ${WATCHER_NAMESPACE:-kubeop-system} get deploy kubeop-watcher
kubectl -n ${WATCHER_NAMESPACE:-kubeop-system} get pods -l app=kubeop-watcher
kubectl -n ${WATCHER_NAMESPACE:-kubeop-system} port-forward deploy/kubeop-watcher 8081:8081 &
curl http://localhost:8081/readyz
```

Disable auto deployment (or override the generated resources) by setting
`WATCHER_AUTO_DEPLOY=false`.

- **Endpoints** – `/healthz`, `/readyz`, and `/metrics` (Prometheus).
- **Delivery** – Batches up to 200 events or 1s are POSTed with bearer
  auth, gzip-compressed when the payload exceeds 8 KiB, and retried with
  exponential backoff. Deduplication is handled per `uid#resourceVersion`.
- **State** – Resume tokens (resource versions) are persisted with BoltDB
  at `${STORE_PATH:-/var/lib/kubeop-watcher/state.db}` so restarts resume
  from the last bookmark.
- **Batch tuning** – Configure with `BATCH_MAX` and `BATCH_WINDOW_MS`.
- **Heartbeat** – Optional `HEARTBEAT_MINUTES` emits a periodic
  synthetic watcher event so kubeOP can alert on stale bridges.
- **Resilience** – The watcher automatically reinitialises the informer
  manager with exponential backoff when startup fails so `/readyz`
  reflects the true state of the bridge.

Build the watcher binary locally with `make build-watcher` or obtain the
container image via `docker build --target watcher .`. Runtime
environment:

```bash
export CLUSTER_ID="cluster-uuid"
export KUBEOP_EVENTS_URL="https://kubeop.example.com/v1/events/ingest"
export KUBEOP_TOKEN="<bearer token issued by kubeOP>"
export KUBECONFIG="/etc/kubeconfig"

./bin/kubeop-watcher \
  -- or --
docker run --rm -v $KUBECONFIG:/kube/config:ro \
  -e KUBECONFIG=/kube/config \
  -e CLUSTER_ID \
  -e KUBEOP_EVENTS_URL \
  -e KUBEOP_TOKEN \
  -p 8081:8081 \
  ghcr.io/vaheed/kubeop:watcher
```

See [`docs/WATCHER.md`](docs/WATCHER.md) for deployment, RBAC, and
configuration details.

## Development workflow

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
- `testcase/migrations_sql_test.go` now enforces sequential, contiguous
  migration numbering and checks for matching down files to keep the
  database history reliable.

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
