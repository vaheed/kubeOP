# KubeOP

KubeOP is an out-of-cluster control plane that lets operators manage multiple Kubernetes clusters from a single API. It focuses on secure multi-tenancy, predictable automation, and observability without running controllers inside the target clusters.

## Key capabilities

- **Multi-cluster management** – ingest kubeconfigs (base64 encoded) and orchestrate user, project, and application lifecycles across clusters.
- **Tenant automation** – bootstrap namespaces, NetworkPolicies, quotas, and credentials with one call while keeping projects scoped to the user namespace by default.
- **Application delivery** – deploy container images, raw manifests, or Helm charts, with CI webhook triggers and attachment endpoints for configs and secrets.
- **Kubectl visibility** – workloads created directly with `kubectl` can surface in project timelines when they include kubeOP labels, while remaining unmanaged so namespaces stay free of surprise kubeOP apps.
- **Security & auditing** – JWT-secured admin APIs, Pod Security Admission profiles, environment-driven hardening, and structured audit logs with redaction of sensitive fields.
- **Operational insight** – JSON logs, per-project/app log streams on disk with download APIs (`/v1/projects/{id}/logs`, `/v1/projects/{id}/apps/{appId}/logs`), `/metrics` for Prometheus, and health/readiness endpoints designed for fast smoke tests. Cluster health scheduler ticks now emit cluster identifiers, warn when dependencies are misconfigured, and expose structured summaries via `TickWithSummary` so operators can feed metrics pipelines without scraping logs.
- **Event visibility** – Normalised project event feeds stored in PostgreSQL and `${LOGS_ROOT}/projects/<project_id>/events.jsonl`, filterable via the `/v1/projects/{id}/events` API and appendable for custom signals.
- **Watcher bridge** – Optional out-of-cluster watcher that streams Kubernetes changes to `/v1/events/ingest`, filters namespaces by prefix (`WATCH_NAMESPACE_PREFIXES`), and forwards events for workloads that already carry kubeOP labels while buffering batches with durable retry queues. Watchers now bootstrap once via `/v1/watchers/register`, persist short-lived JWTs + refresh tokens in their BoltDB state file, automatically rotate credentials on 401 responses, and schedule proactive access-token refreshes to tolerate clock skew between the kubeOP control plane and remote clusters. When the API responds with a transient 401, the sink now forces a fresh watcher registration before retrying the batch so kubectl-driven changes stay visible without manual intervention, falling back to a token refresh only if bootstrap re-registration fails. This forced re-registration runs synchronously before the retry to eliminate the persistent 401 loop observed when credential rotation raced queued deliveries. Watcher pods keep the hardened `restricted` PodSecurity defaults (non-root, drop all Linux capabilities, `allowPrivilegeEscalation=false`, seccomp `RuntimeDefault`) with UID/GID/FSGroup `65532` overrides when required. Upgraded control planes automatically backfill the `cluster_id` claim for legacy watcher tokens during ingest so older deployments continue streaming without manual credential rotation. After a successful handshake, any unauthorized batch flush now triggers an immediate credential refresh signal and a short re-handshake delay so newly issued tokens are exercised before the next delivery attempt.

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

See [`docs/architecture.md`](docs/architecture.md) for the full component walkthrough and sequence diagrams.

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
   Logs stream to `./logs`; the API listens on `http://localhost:8080` (Docker Compose maps container port `8080` to the host so the REST API is reachable from your workstation).
   The Compose file now sets both `image: ghcr.io/vaheed/kubeop-api:latest` and `target: api` so local builds stay on the API binary while always pulling the latest published API image. If you built the repository before the package split, remove any locally tagged `ghcr.io/vaheed/kubeop` image before `docker compose up` so Docker does not reuse the legacy watcher artifact that exits early with `config error: CLUSTER_ID is required (this container runs the watcher agent; use the :latest tag for the API)`.

> **Using published images**
>
> If you prefer to skip the local build, point Compose to the published image and force a pull so Docker does not reuse a previously built layer:
>
> ```yaml
> services:
>   api:
>     image: ghcr.io/vaheed/kubeop-api:latest
>     pull_policy: always
> ```
>
> Seeing `config error: CLUSTER_ID is required (this container runs the watcher agent; use the :latest tag for the API)` means the watcher image is running—replace the tag with `:latest` from `ghcr.io/vaheed/kubeop-api`, rebuild with `target: api`, and remove any stale local watcher images tagged `ghcr.io/vaheed/kubeop` or `ghcr.io/vaheed/kubeop-watcher` before re-running Compose.
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
> **Watcher rollout happens asynchronously.** kubeOP persists the cluster immediately and then schedules the watcher
> deployment in the background. Watch for `queueing watcher deployment ensure`, `starting watcher ensure`, and
> `watcher ensure complete` logs to confirm rollout. Failures stay logged as `watcher ensure failed`; rerun the watcher ensure
> job after fixing the underlying issue. The API response no longer waits for the watcher Deployment to become ready, so
> cluster registration returns in a few seconds even if the watcher takes longer to settle, and rollout errors never block the
> cluster from being registered.
6. **Bootstrap a user and project namespace**
   ```bash
   curl -s $AUTH_H -H 'Content-Type: application/json' \
     -d '{"name":"Alice","email":"alice@example.com","clusterId":"<cluster-id>"}' \
     http://localhost:8080/v1/users/bootstrap | jq
   ```
   kubeOP now provisions the managed `tenant-quota` ResourceQuota with Kubernetes `count/<resource>` identifiers so clusters
   running newer API validations accept workload quotas for Deployments, Jobs, StatefulSets, and Ingresses. When these object
   counts are configured, kubeOP automatically drops incompatible quota scopes such as `NotBestEffort` to avoid the server-side
   `unsupported scope applied to resource` error while still respecting CPU and memory requests/limits scopes where they apply.
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

   # Namespace-scoped kubeconfigs can manage workloads and configs in their namespace only
   kubectl --kubeconfig kubeconfig.yaml auth can-i create deployments -n user-<userId>
   kubectl --kubeconfig kubeconfig.yaml auth can-i get secrets -n user-<userId>
   kubectl --kubeconfig kubeconfig.yaml -n user-<userId> scale deployment web-02 --replicas=2
   ```

Additional walkthroughs live in [`docs/getting-started.md`](docs/getting-started.md) and the guides under [`docs/guides/`](docs/guides/tenants-projects-apps.md).

## Documentation

The public documentation is published automatically to GitHub Pages at
[`https://vaheed.github.io/kubeOP/`](https://vaheed.github.io/kubeOP/). The
VitePress site is built from the contents of `docs/` with the repository base
path configured for GitHub Pages hosting. Key operator guides now include:

- **Zero to Production walkthrough** – [`docs/zero-to-prod.md`](docs/zero-to-prod.md) stitches cloning, configuration, watcher rollout, tenancy, app delivery, DNS, and TLS into a single copy-pasteable runbook.
- **REST API catalogue** – [`docs/api/ENDPOINTS.md`](docs/api/ENDPOINTS.md) lists every public endpoint with schemas, examples, and failure cases.
- **kubectl verification cheatsheet** – [`docs/reference/kubectl.md`](docs/reference/kubectl.md) maps each API mutation to the corresponding cluster validation command.
- **Security policy** – [`docs/security.md`](docs/security.md) covers vulnerability reporting, supported versions, and hardening expectations.

Use the local commands below to work on documentation before pushing changes:

```bash
npm install
npm run docs:dev   # local preview
npm run docs:build # production build
```

The generated site lives under `docs/.vitepress/dist/`.

## Running locally with Go

1. Start PostgreSQL (the bundled Compose stack reads credentials from `.env`).
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

All configuration happens through environment variables. Copy
`.env.example` to `.env` to see the fully documented list used by both the API
and the docker-compose stack. Core values include:

| Variable | Default | Description |
| --- | --- | --- |
| `DATABASE_URL` | `postgres://postgres:postgres@postgres:5432/kubeop?sslmode=disable` | PostgreSQL connection string. |
| `ADMIN_JWT_SECRET` | _none_ | HMAC secret used to validate admin tokens. |
| `KUBEOP_BASE_URL` | _empty_ | External HTTPS endpoint for kubeOP. Powers watcher handshake + event ingest. When set, watcher auto deployment turns on automatically unless overridden. |
| `ALLOW_INSECURE_HTTP` | `false` | Permit `http://` base URLs for development-only scenarios. |
| `LOG_LEVEL` | `info` | Minimum structured log level (`debug`, `info`, `warn`, `error`). |
| `LOGS_ROOT` | `/var/log/kubeop` | Root for project/app log directories. Identifiers must match `[A-Za-z0-9._-]+`. |
| `AUDIT_ENABLED` | `true` | Emit audit events for mutating requests. |
| `EVENTS_DB_ENABLED` | `true` | Persist project events to PostgreSQL in addition to disk-backed JSONL logs. Disable to operate in file-only mode. |
| `K8S_EVENTS_BRIDGE` | `false` | Enable ingestion of Kubernetes core/v1 Events into the project event stream when the bridge is deployed. |
| `PROJECTS_IN_USER_NAMESPACE` | `true` | Scope projects to the owning user’s namespace by default. |
| `DISABLE_AUTH` | `false` | Bypass admin auth for development/testing only. |

A complete list and tuning guidance is available in [`docs/configuration.md`](docs/configuration.md).

The local development workflow expects the PostgreSQL container to inherit
`POSTGRES_USER`, `POSTGRES_PASSWORD`, and `POSTGRES_DB` from the same `.env`
file, while the API process uses the matching `PG*` variables to construct the
runtime connection string. Adjust them in one place and both services stay in
sync.

### Namespace limit policy defaults

Every managed namespace receives a `tenant-quota` `ResourceQuota` and a
`tenant-limits` `LimitRange` annotated with `managed-by=kubeop-operator`. These
objects enforce a balanced slice of cluster capacity for each tenant. The
defaults are driven by the `KUBEOP_DEFAULT_*` environment variables, covering
namespace-wide quotas (CPU, memory, ephemeral storage, object counts) and
per-container/pod LimitRange settings. Key variables include:

| Variable | Description |
| --- | --- |
| `KUBEOP_DEFAULT_REQUESTS_CPU`, `KUBEOP_DEFAULT_LIMITS_CPU` | Namespace CPU request/limit caps. |
| `KUBEOP_DEFAULT_REQUESTS_MEMORY`, `KUBEOP_DEFAULT_LIMITS_MEMORY` | Namespace memory request/limit caps. |
| `KUBEOP_DEFAULT_PODS`, `KUBEOP_DEFAULT_SERVICES`, `KUBEOP_DEFAULT_SERVICES_LOADBALANCERS` | Core object quotas. |
| `KUBEOP_DEFAULT_REQUESTS_STORAGE` | Total PVC storage requests. |
| `KUBEOP_DEFAULT_SCOPES`, `KUBEOP_DEFAULT_PRIORITY_CLASSES` | ResourceQuota scope filters (e.g. `NotBestEffort`, allowed priority classes). Scopes incompatible with configured quotas are dropped automatically. |
| `KUBEOP_DEFAULT_LR_CONTAINER_*` | Container LimitRange max/min/default/defaultRequest values for CPU, memory, and ephemeral storage. |
| `KUBEOP_DEFAULT_LR_EXT_*` | Optional extended resource limits (default empty so pods don't request GPUs unless configured). |
| `PROJECT_LR_REQUEST_CPU`, `PROJECT_LR_REQUEST_MEMORY`, `PROJECT_LR_LIMIT_CPU`, `PROJECT_LR_LIMIT_MEMORY` | Project-scoped LimitRange defaults (100m/128Mi requests, 1 CPU/1Gi limits by default). |

See `.env.example` and [`docs/configuration.md`](docs/configuration.md) for the full
list and tuning guidance. kubeOP reapplies this namespace limit policy whenever
it provisions a tenant namespace, updates quota overrides, or toggles project
suspension, so manual drift from the defaults is corrected automatically.

## Automatic domains, DNS, and TLS

Set `PAAS_DOMAIN` and pick a DNS provider via `DNS_PROVIDER` (`http`, `cloudflare`, or `powerdns`) to enable fully automated ingress provisioning. kubeOP derives a stable FQDN for every app using `<app-full>.<project>.<cluster>.<PAAS_DOMAIN>`, where `<app-full>` combines the slugified app name with a deterministic short hash of the app ID (for example, `web-02-f7f88c5b4-4ldbq`). It then creates the Ingress with cert-manager annotations for the `letsencrypt-prod` ClusterIssuer and waits for the Service load balancer to publish IPv4 and IPv6 addresses. Once the addresses are available kubeOP calls the DNS API to upsert matching `A`/`AAAA` records with `DNS_RECORD_TTL`. The HTTP provider uses `DNS_API_URL` + `DNS_API_KEY`; the Cloudflare driver reads `CLOUDFLARE_ZONE_ID` (and `CLOUDFLARE_API_TOKEN` when the shared `DNS_API_KEY` is unset); the PowerDNS driver patches `PDNS_API_URL`/`PDNS_API_KEY` for `PDNS_SERVER_ID` and `PDNS_ZONE` (defaults to `PAAS_DOMAIN`). Domain assignments are persisted in PostgreSQL and surfaced through the app status endpoints under `domains[]`, including the latest certificate status (`pending`, `issued`, or error details) pulled from the associated cert-manager `Certificate`. When an app is removed, kubeOP deletes the Ingress, TLS secret, certificate, DNS records, and domain rows automatically.

## API essentials

- Base URL: `http://localhost:8080`
- Health probes: `/healthz`, `/readyz`
- Version metadata: `/v1/version`
- Core workflows:
  - `POST /v1/clusters` – register a cluster (requires `kubeconfig_b64`).
  - `POST /v1/users/bootstrap` – create user, namespace, default quotas, kubeconfig.
  - `POST /v1/projects` – create project scoped to a cluster/user namespace.
  - `GET /v1/projects/{id}/quota` – inspect defaults, overrides, and current usage (including load balancer caps).
  - `POST /v1/kubeconfigs` – ensure or mint a namespace-scoped kubeconfig (user or project); rotate via `POST /v1/kubeconfigs/rotate` and revoke with `DELETE /v1/kubeconfigs/{id}`.
  - App deployments via `/v1/apps` (image, manifests, Helm) with optional CI webhooks.
  - Project event history via `GET /v1/projects/{id}/events` with filters for kind, severity, actor, search terms, cursor pagination, and custom append via `POST /v1/projects/{id}/events`.

Refer to [`docs/openapi.yaml`](docs/openapi.yaml) or the VitePress API pages under [`docs/api/`](docs/api/README.md) for schemas, request/response examples, and authentication details.

## Observability & logs

- Structured JSON logs go to stdout and `${LOGS_ROOT}/app.log` with `X-Request-Id` correlation.
- Audit events write to `${LOGS_ROOT}/audit.log` when enabled; sensitive fields are redacted.
- Project/app logs live under `${LOGS_ROOT}/projects/<project_id>/apps/<app_id>/` with safe identifier enforcement, and event streams replicate to `${LOGS_ROOT}/projects/<project_id>/events.jsonl` alongside the `/v1/projects/{id}/events` API.
- `GET /v1/projects/{id}/logs` accepts `tail` to return the most recent lines with a safeguard of 5,000 lines to prevent excessive memory usage when streaming from disk.
- Prometheus metrics served at `/metrics`, including the `readyz_failures_total` counter for alerting on repeated readiness probe issues.
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
`kubeop.project-id`/`kubeop.app-id`/`kubeop.tenant.id` labels are
forwarded (the bridge also accepts the historic `kubeop.<name>.id`
variants), keeping tenant traffic scoped.

> Enable ingestion by setting `K8S_EVENTS_BRIDGE=true` alongside
> `EVENTS_DB_ENABLED=true` on the control plane. When disabled, the API
> still acknowledges watcher batches with `202 Accepted` but drops the
> payloads so watchers do not thrash retries while operators prepare the
> database or logging destination.

### Connectivity expectations

- **API exposure** – kubeOP always listens on container port `8080`. Docker
  Compose binds that port to `${PORT:-8080}` on the host. Production
  deployments should publish the API through an ingress or load balancer so
  it is reachable at the `KUBEOP_BASE_URL` you configure.
- **Watcher access** – the watcher performs an authenticated handshake against
  `${KUBEOP_BASE_URL}/v1/watchers/handshake` (posting `{"cluster_id": "<CLUSTER_ID>"}` so
  older watcher tokens without the `cluster_id` claim continue to validate, and
  cluster-scoped bootstrap tokens without an explicit `watcher_id` can be
  resolved server-side) and
  streams batches to `${KUBEOP_BASE_URL}/v1/events/ingest`. Ensure those URLs resolve from the
  managed cluster (or wherever the watcher runs) and that firewalls allow
  TCP/443 (or the custom port in the URL).
  > kubeOP 0.14.8 also rehydrates the missing `cluster_id` during ingest based
  > on the persisted watcher record, keeping bridges registered before 0.14.8
  > online until you rotate their credentials.
- **Watcher diagnostics** – the watcher container exposes `:8081` for
  `/healthz`, `/readyz`, and `/metrics`. The Docker image now declares that
  port so Kubernetes Services or port-forwards can publish it when needed.

### Automatic deployment

Point kubeOP at its external HTTPS endpoint (for example,
`KUBEOP_BASE_URL=https://kubeop.example.com` or `baseURL` in a config file).
Without this value, watcher auto-deployment stays disabled so local development
or air-gapped installs do not fail health checks when the ingest endpoint is
unreachable. Once the base URL is configured, watcher auto deployment is
enabled by default unless you explicitly set `WATCHER_AUTO_DEPLOY=false` (env)
or `watcherAutoDeploy: false` (config file): every new cluster registration
provisions the ServiceAccount, RBAC, Secret, storage, and Deployment inside the
managed cluster, waiting for readiness before the API call returns. kubeOP now
performs an explicit `/v1/watchers/handshake` before reporting readiness and
caches batches to the local BoltDB queue whenever the API is unavailable,
flushing them automatically once the handshake succeeds again.

On startup kubeOP now logs the watcher auto-deploy status together with the
detected reason (`base-url`, config, or environment override). Each cluster
registration also records whether the watcher deployment ran or was skipped so
operators can confirm auto deployment decisions straight from the API logs.

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
  kubeOP records accepted batches (`accepted`/`dropped` counters) in the
  JSON response and structured logs so operators can trace ingestion
  health in addition to watcher metrics.
- **State** – Resume tokens (resource versions) are persisted with BoltDB
  at `${STORE_PATH:-/var/lib/kubeop-watcher/state.db}` so restarts resume
  from the last bookmark.
- **Logs** – Structured watcher logs now land under
  `${LOGS_ROOT:-/var/lib/kubeop-watcher/logs}`. The auto-deployer points this
  at the same writable volume as the state database so restricted
  deployments no longer attempt to write to `/var/log`.
- **Batch tuning** – Configure with `BATCH_MAX` and `BATCH_WINDOW_MS`.
- **Heartbeat** – Optional `HEARTBEAT_MINUTES` emits a periodic
  synthetic watcher event so kubeOP can alert on stale bridges.
- **Resilience** – The watcher automatically reinitialises the informer
  manager with exponential backoff when startup fails so `/readyz`
  reflects the true state of the bridge.
- **Readiness** – `/readyz` returns `{"status":"ready"}` once the state DB
  opens, informer caches sync, a recent handshake succeeds, and queued batches
  flush without errors. When kubeOP is unreachable or ingest rejects events the
  probe still responds with HTTP 200 but marks the status as `"degraded"` and
  includes diagnostic fields while the watcher buffers events on disk for later
  replay. Example probe responses:

  ```bash
  $ curl -sS http://localhost:8081/readyz | jq
  {
    "status": "degraded",
    "diagnostics": {
      "handshake": {
        "detail": "dial tcp 10.0.0.5:7780: connect: connection refused",
        "fresh": false,
        "ready": false
      }
    }
  }

  $ curl -sS http://localhost:8081/readyz | jq
  {
    "status": "degraded",
    "diagnostics": {
      "delivery": {
        "detail": "deliver queued events: aborted after 1 attempt(s): unexpected status 401",
        "healthy": false
      }
    }
  }
  ```

Build the watcher binary locally with `make build-watcher` or obtain the
container image via `docker build --target watcher .`. Runtime
environment:

```bash
export CLUSTER_ID="cluster-uuid"
export KUBEOP_EVENTS_URL="https://kubeop.example.com/v1/events/ingest"
export KUBEOP_BOOTSTRAP_TOKEN="<bootstrap token issued by kubeOP>"
export KUBECONFIG="/etc/kubeconfig"
export LOGS_ROOT="/var/lib/kubeop-watcher/logs"

./bin/kubeop-watcher \
  -- or --
docker run --rm \
  -v $KUBECONFIG:/kube/config:ro \
  -v watcher-data:/var/lib/kubeop-watcher \
  -e KUBECONFIG=/kube/config \
  -e CLUSTER_ID \
  -e KUBEOP_EVENTS_URL \
  -e KUBEOP_BOOTSTRAP_TOKEN \
  -e LOGS_ROOT=$LOGS_ROOT \
  -p 8081:8081 \
  ghcr.io/vaheed/kubeop-watcher:latest
```

The named volume `watcher-data` (or a host bind mount) gives the non-root
watcher process a writable home for both the BoltDB state file and structured
logs. On first boot the watcher calls `/v1/watchers/register` with
`KUBEOP_BOOTSTRAP_TOKEN`, persists the issued watcher ID plus short-lived JWT
and refresh token under `STORE_PATH`, and automatically rotates credentials when
`/v1/events/ingest` returns 401/403 so batches resume without manual
intervention.

When developing against a non-TLS control plane, set `ALLOW_INSECURE_HTTP=true`
alongside `KUBEOP_BASE_URL=http://...` so both the watcher handshake and the
event sink allow HTTP targets. Production deployments must continue using HTTPS.

See [`docs/guides/watcher-sync.md`](docs/guides/watcher-sync.md) for deployment, RBAC, and
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

CI pipelines (see `.github/workflows/ci.yml`) install dependencies, lint with `go vet`, run tests, build the binary artifact, and build the VitePress documentation site (`npm run docs:build`).

## Repository layout

```
cmd/api/                  # Application entrypoint and HTTP wiring
internal/                 # Domain logic, services, logging, crypto, data access
internal/store/migrations # PostgreSQL schema migrations (golang-migrate format)
docs/                     # VitePress site (content, API reference, changelog)
samples/                  # Example manifests and payloads
testcase/                 # Go test suites (package-aligned)
```

Consult the VitePress docs under `docs/` (e.g., `docs/operations.md`, `docs/guides/`, `docs/api/`) for deeper dives.

## Contributing

1. Review [`docs/contributing.md`](docs/contributing.md) and repository rules in `AGENTS.md`.
2. Keep documentation up to date alongside code changes.
3. Run `go vet`, `go test ./...`, and ensure the Docker Compose stack still boots.
4. Include tests for new or changed functionality under `testcase/`.

## License

KubeOP is released under the [MIT License](LICENSE).
