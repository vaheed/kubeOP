# KubeOP

KubeOP is an out-of-cluster control plane that lets operators manage multiple Kubernetes clusters from a single API. It focuses on secure multi-tenancy, predictable automation, and observability without running controllers inside the target clusters.

## Key capabilities

- **Multi-cluster management** – ingest kubeconfigs (base64 encoded) and orchestrate user, project, and application lifecycles across clusters.
- **Tenant automation** – bootstrap namespaces, NetworkPolicies, quotas, and credentials with one call while keeping projects scoped to the user namespace by default.
- **Application delivery** – deploy container images, raw manifests, or Helm charts, with CI webhook triggers and attachment endpoints for configs and secrets.
- **OCI manifest bundles** – ship Kubernetes manifests packaged as OCI artifacts with digest tracking, credential reuse, and validation before apply.
- **Deployment preflight** – dry-run app specs with `/v1/apps/validate` to confirm quotas, rendering, and generated manifests before touching Kubernetes.
- **Kubectl visibility** – workloads created directly with `kubectl` can surface in project timelines when they include kubeOP labels, while remaining unmanaged so namespaces stay free of surprise kubeOP apps.
- **Security & auditing** – JWT-secured admin APIs, Pod Security Admission profiles, environment-driven hardening, and structured audit logs with redaction of sensitive fields.
- **Operational insight** – JSON logs, per-project/app log streams on disk with download APIs (`/v1/projects/{id}/logs`, `/v1/projects/{id}/apps/{appId}/logs`), `/metrics` for Prometheus, and health/readiness endpoints designed for fast smoke tests. Cluster health scheduler ticks now emit cluster identifiers, warn when dependencies are misconfigured, and expose structured summaries via `TickWithSummary` so operators can feed metrics pipelines without scraping logs.
- **Event visibility** – Normalised project event feeds stored in PostgreSQL and `${LOGS_ROOT}/projects/<project_id>/events.jsonl`, filterable via the `/v1/projects/{id}/events` API and appendable for custom signals.
- **Credential vault** – encrypted Git and container registry credential stores with `/v1/credentials/*` endpoints so delivery engines fetch sources without embedding secrets in app specs.
- **Maintenance guardrails** – toggle `/v1/admin/maintenance` to pause mutating API flows during upgrades and surface clear
  503 responses to automation until maintenance completes.

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
   The Compose file builds the API binary locally while also referencing the latest published image. Remove any legacy images tagged `ghcr.io/vaheed/kubeop` before running `docker compose up` so Docker does not reuse outdated layers.

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
     -d "$(jq -n --arg name 'talos-stage' --arg b64 "$B64" '{name:$name,kubeconfig_b64:$b64,"owner":"platform","environment":"staging","region":"eu-west","tags":["platform","staging"]}')" \
     http://localhost:8080/v1/clusters | jq
   ```
   Cluster registration now accepts optional metadata so operators can track ownership, deployment environment, region, API endpoint, and arbitrary tags. Metadata flows into the cluster registry, inventory docs, and health dashboards exposed via `/v1/clusters`, `/v1/clusters/{id}`, and `/v1/clusters/{id}/status`.
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
7. **Store Git or registry credentials (optional)**
   ```bash
   curl -s $AUTH_H -H 'Content-Type: application/json' \
     -d '{"name":"git-main","scope":{"type":"user","id":"<user-id>"},"auth":{"type":"token","token":"ghp_example"}}' \
     http://localhost:8080/v1/credentials/git | jq

   curl -s $AUTH_H -H 'Content-Type: application/json' \
     -d '{"name":"dockerhub","registry":"https://index.docker.io/v1/","scope":{"type":"project","id":"<project-id>"},"auth":{"type":"basic","username":"repo","password":"s3cret"}}' \
     http://localhost:8080/v1/credentials/registries | jq
   ```

8. **Dry-run an app deployment**
   ```bash
   curl -s $AUTH_H -H 'Content-Type: application/json' \
     -d '{"projectId":"<project-id>","name":"web","image":"ghcr.io/example/web:1.2.3","ports":[{"containerPort":80,"servicePort":80,"serviceType":"LoadBalancer"}]}' \
     http://localhost:8080/v1/apps/validate | jq
   ```
   The response echoes the computed Kubernetes resource names, effective replicas/resources, load balancer quota usage, and a manifest summary without applying anything to the cluster.

> **Deploy Helm charts from OCI registries**
>
> kubeOP now understands `helm.oci` payloads so operators can fetch charts directly from registries such as GHCR, Harbor, or ECR. Reference an optional `registryCredentialId` created via `/v1/credentials/registries` when private authentication is required.
>
> ```bash
> curl -s $AUTH_H -H 'Content-Type: application/json' \
>   -d '{
>         "name": "grafana",
>         "helm": {
>           "oci": {
>             "ref": "oci://ghcr.io/example/charts/grafana:11.0.0",
>             "registryCredentialId": "<registry-credential-id>"
>           },
>           "values": {
>             "service": {"type": "ClusterIP"}
>           }
>         }
>       }' \
>   http://localhost:8080/v1/projects/<project-id>/apps | jq
> ```
> kubeOP resolves the registry host with the same egress safeguards as HTTPS chart downloads, logs into the registry when credentials are supplied, and renders the chart with Helm before applying the manifests. Set `"insecure": true` inside `helm.oci` only for trusted on-prem registries served over plain HTTP during development.

> **Deploy OCI manifest bundles**
>
> kubeOP can pull raw manifest bundles published as OCI artifacts and apply them directly to the project namespace.
> ```bash
> curl -s $AUTH_H -H 'Content-Type: application/json' \
>   -d '{
>         "name": "bundle-app",
>         "ociBundle": {
>           "ref": "oci://ghcr.io/example/bundles/web:1.2.0",
>           "credentialId": "<registry-credential-id>"
>         }
>       }' \
>   http://localhost:8080/v1/projects/<project-id>/apps | jq
> ```
> Validation responses echo `ociBundleRef` and `ociBundleDigest` so you can double-check which artifact will be deployed. Archives are validated for safe paths and size, and registry hosts must resolve to globally routable addresses. Set `"insecure": true` only when working with trusted HTTP registries during local development.

9. **Mint or rotate kubeconfigs on demand**

## Samples library

The `samples/` directory ships with reusable automation scaffolding so teams can
bootstrap kubeOP flows without writing bespoke scripts. Each sample sources
`./samples/.env.samples` and `./samples/lib/common.sh` to provide timestamped logs,
command validation, and safe defaults. Full documentation now lives under
[`docs/samples/`](docs/samples/index.md) so the repository keeps Markdown content
centralised.

```bash
cd samples/00-bootstrap
cp .env.example .env
# Populate AUTH_TOKEN, PROJECT_ID, USER_EMAIL, and CLUSTER_ID
./curl.sh    # preview bootstrap payloads
./verify.sh  # check /healthz and /readyz
./cleanup.sh # remove temp files
```

See [`docs/TUTORIALS/samples-scaffolding.md`](docs/TUTORIALS/samples-scaffolding.md) for a full walkthrough
of the scaffolding, expected output, and customisation tips.


    ```bash
    API_ORIGIN=${API_ORIGIN:-http://127.0.0.1:8080}

    # Ensure or fetch an existing binding (user or project scope)
    curl -s $AUTH_H -H 'Content-Type: application/json' \
      -d '{"userId":"<user-id>","clusterId":"<cluster-id>"}' \
      "${API_ORIGIN}/v1/kubeconfigs" | jq

    # Rotate a binding by ID (returns a fresh token kubeconfig)
    curl -s $AUTH_H -H 'Content-Type: application/json' \
      -d '{"id":"<binding-id>"}' \
      "${API_ORIGIN}/v1/kubeconfigs/rotate" | jq

    # Namespace-scoped kubeconfigs can manage workloads and configs in their namespace only
    kubectl --kubeconfig kubeconfig.yaml auth can-i create deployments -n user-<userId>
    kubectl --kubeconfig kubeconfig.yaml auth can-i get secrets -n user-<userId>
    kubectl --kubeconfig kubeconfig.yaml -n user-<userId> scale deployment web-02 --replicas=2
    ```

10. **Promote reusable application templates**
    ```bash
    TEMPLATE_ID=$(curl -s $AUTH_H -H 'Content-Type: application/json' \
      -d '{
            "name": "nginx-template",
            "kind": "helm",
            "description": "Baseline nginx deployment",
            "schema": {"type":"object","properties":{"name":{"type":"string"}},"required":["name"]},
            "defaults": {"name": "web"},
            "deliveryTemplate": "{\\n  \\\"name\\\": \\\"{{ .values.name }}\\\",\\n  \\\"image\\\": \\\"ghcr.io/library/nginx:1.27\\\"\\n}"
        }' \
      http://localhost:8080/v1/templates | jq -r '.id')

    curl -s $AUTH_H -H 'Content-Type: application/json' \
      -d '{"values":{"name":"web-blue"}}' \
      http://localhost:8080/v1/templates/${TEMPLATE_ID}/render | jq

    curl -s $AUTH_H -H 'Content-Type: application/json' \
      -d '{"values":{"name":"web-blue"}}' \
      http://localhost:8080/v1/projects/<project-id>/templates/${TEMPLATE_ID}/deploy | jq
    ```
    Rendering merges JSON Schema–validated defaults with per-deployment overrides so teams can preview specs before shipping
    them with `/deploy`.

11. **Deploy manifests straight from Git**
    ```bash
    curl -s $AUTH_H -H 'Content-Type: application/json' \
      -d '{
            "name": "git-app",
            "git": {
              "url": "https://github.com/example/platform-configs.git",
              "ref": "refs/heads/main",
              "path": "apps/web/overlays/prod",
              "mode": "kustomize"
            }
          }' \
      http://localhost:8080/v1/projects/<project-id>/apps | jq
    ```
    kubeOP clones the repository (with optional Git credentials from `/v1/credentials/git`), renders manifests using either raw YAML or Kustomize overlays, and stores the commit hash in the release record. Local testing against `file://` repositories requires `ALLOW_GIT_FILE_PROTOCOL=true` in `.env`; keep it `false` in production environments.

12. **Inspect release history**
    ```bash
    curl -s $AUTH_H "http://localhost:8080/v1/projects/<project-id>/apps/<app-id>/releases?limit=5" | jq
    ```
    The response lists each rollout with spec digests, rendered object summaries, load balancer usage, and warnings so you can
    audit what changed between deployments. Use the `nextCursor` value (a release ID) with the same `projectId`/`appId` pair to
    keep paging through older entries.

13. **Pause mutating APIs during maintenance**
    ```bash
    # Enable maintenance with a descriptive message
    curl -s $AUTH_H -H 'Content-Type: application/json' \
      -d '{"enabled":true,"message":"Control plane upgrade"}' \
      http://localhost:8080/v1/admin/maintenance | jq

    # Disable maintenance when finished
    curl -s $AUTH_H -H 'Content-Type: application/json' -d '{"enabled":false}' \
      http://localhost:8080/v1/admin/maintenance | jq
    ```
    While maintenance is enabled, kubeOP responds to project/app/cluster mutations with HTTP `503` and the message you provide,
    signalling CI/CD pipelines to pause until the platform is healthy again.

Additional walkthroughs live in [`docs/getting-started.md`](docs/getting-started.md) and the guides under [`docs/guides/`](docs/guides/tenants-projects-apps.md).

## Documentation

The public documentation is published automatically to GitHub Pages at
 [`docs/`](./docs/index.md). The
VitePress site is built from the contents of `docs/` with the repository base
path configured for GitHub Pages hosting. Key operator guides now include:

- **Zero to Production walkthrough** – [`docs/zero-to-prod.md`](docs/zero-to-prod.md) covers cloning, configuration, tenancy setup, app delivery, DNS, and TLS in a single copy-pasteable runbook.
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

## Repository hygiene and cleanup report

The `repo-sanity` GitHub Actions workflow and [`hack/repo_sanity.py`](hack/repo_sanity.py)
script keep duplicate or orphaned files out of the tree and ensure Markdown links
stay valid. Run the helper locally before pushing large cleanups:

```bash
python3 hack/repo_sanity.py
```

Historical cleanup notes live in [`docs/reports/cleanup-report.md`](docs/reports/cleanup-report.md).

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
| `KUBEOP_BASE_URL` | _empty_ | External HTTPS endpoint for kubeOP. Used to populate URLs in responses and callbacks. |
| `ALLOW_INSECURE_HTTP` | `false` | Permit `http://` base URLs for development-only scenarios. |
| `LOG_LEVEL` | `info` | Minimum structured log level (`debug`, `info`, `warn`, `error`). |
| `LOGS_ROOT` | `/var/log/kubeop` | Root for project/app log directories. Identifiers must match `[A-Za-z0-9._-]+`. |
| `AUDIT_ENABLED` | `true` | Emit audit events for mutating requests. |
| `EVENTS_DB_ENABLED` | `true` | Persist project events to PostgreSQL in addition to disk-backed JSONL logs. Disable to operate in file-only mode. |
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

## Versioning & compatibility

- kubeOP follows Semantic Versioning. The `/v1/version` endpoint now returns a compatibility matrix so API clients can refuse to run with unsupported binaries.
- `compatibility.minClientVersion` is the minimum kubeOP CLI/automation version validated against the server. Older tooling should upgrade before proceeding.
- `compatibility.minApiVersion` and `compatibility.maxApiVersion` advertise the supported REST surface (`/v1` today) for forward planning.
- When release managers set `deprecation.deadline`, the API logs a warning once the deadline passes so operators know to upgrade.

```bash
curl http://localhost:8080/v1/version | jq
# {
#   "version": "0.8.21",
#   "commit": "<sha>",
#   "date": "2025-10-29T10:00:00Z",
#   "compatibility": {
#     "minClientVersion": "0.8.16",
#     "minApiVersion": "v1",
#     "maxApiVersion": "v1"
#   }
# }
```

See [`docs/reference/versioning.md`](docs/reference/versioning.md) for detailed compatibility policy and release cadence notes.

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
