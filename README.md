# kubeOP

Operator‑powered, multi‑tenant platform for Kubernetes. kubeOP provides a small
set of custom resources and controllers to model tenants, projects, and apps; a
manager API for automation and billing; and an admission webhook that enforces
baseline security policy (image registry allow‑list, network egress baseline,
resource quotas). It’s designed to be simple, reproducible, and easy to test
locally and in CI.

• Docs live under `docs/` — start with: `docs/getting-started.md`, `docs/crds.md`, `docs/controllers.md`, `docs/config.md`, `docs/api.md`.

## Features
- Multi‑tenant model via CRDs: `Tenant`, `Project`, `App`, `DNSRecord`, `Certificate`.
- Kubernetes operator (controllers) to reconcile namespaces, network policies,
  deployments, DNS and certificates (via mocks for local). Exposes `/healthz`,
  `/readyz`, `/version` and `/metrics` in‑cluster.
- Admission webhook to enforce:
  - Image registry allow‑list (`KUBEOP_IMAGE_ALLOWLIST`).
  - Egress CIDR baseline (`KUBEOP_EGRESS_BASELINE`).
  - Optional ResourceQuota ceilings.
- Manager REST API with OpenAPI and metrics, covering:
  - Tenants, Projects, Apps CRUD.
  - Usage ingest + invoice snapshot (simple CPU/mem rates).
  - Cluster registration + readiness checks.
  - Per‑project kubeconfigs and short‑lived JWT minting.
  - CronJobs API for project‑scoped scheduled jobs (create/list/run/delete).
- End‑to‑end (Kind) bootstrap with mock DNS/ACME and CI automation.

## Architecture
- Manager (API): `cmd/manager`, HTTP server with `/healthz`, `/readyz`,
  `/version`, `/openapi.json`, `/metrics`.
- Operator (controllers): `cmd/operator` + `internal/operator/controllers` for
  CRDs in `deploy/k8s/crds`.
- Admission (webhook): `cmd/admission` enforced via Helm chart.
- Mocks (local only): `cmd/dnsmock`, `cmd/acmemock`.
- Aggregator: background usage rollups in the Manager.
- PostgreSQL database for metadata and usage.

Images published to GHCR:
- `ghcr.io/vaheed/kubeop/manager`
- `ghcr.io/vaheed/kubeop/operator`

## Tech Stack
- Go `1.24+`, Docker/Compose, Kind, `kubectl`, Helm, `jq`, `yq`.
- Docs (VitePress): Node.js `18+` under `docs/`.
- Database: PostgreSQL.

## Repository Layout
- API server: `internal/api`, `cmd/manager`
- Controllers: `internal/operator`, `cmd/operator`
- Admission: `cmd/admission`
- Kube helpers: `internal/kube`
- Models/DB: `internal/models`, `internal/db`
- CRDs & manifests: `deploy/k8s/`
- Helm charts: `charts/`
- E2E assets: `e2e/` and tests in `hack/e2e/`
- Docs: `docs/`

## Quickstart (Local Dev)
Prerequisites: Go 1.24+, Docker, Kind, kubectl, Helm, make.

1) Configure environment
- Copy env defaults: `cp env.example .env`
- Ensure PostgreSQL URL is reachable from host: `KUBEOP_DB_URL=postgres://kubeop:kubeop@localhost:5432/kubeop?sslmode=disable`

2) Create a Kind cluster and bootstrap kubeOP components
- `make kind-up` — create Kind (`e2e/kind-config.yaml`).
- `bash e2e/bootstrap.sh` — install CRDs, build/load local images, install Helm chart
  for operator + admission (with mocks for local).

3) Start the Manager
- Option A (compose): `docker compose up -d db && docker compose up -d manager`
- Option B (local binary):
  - `make build`
  - Run with dev‑insecure for local ease: `KUBEOP_DEV_INSECURE=true KUBEOP_REQUIRE_AUTH=false KUBEOP_HTTP_ADDR=:18080 KUBEOP_DB_URL=postgres://kubeop:kubeop@localhost:5432/kubeop?sslmode=disable ./bin/manager`

4) Check readiness
- `curl -sf http://localhost:18080/healthz` → `200`
- `curl -sf http://localhost:18080/readyz` → `200`
- OpenAPI: `http://localhost:18080/openapi.json`

## Makefile Targets
- `make right` — `fmt`, `vet`, `tidy`, build.
- `make build` — build `bin/manager`, `bin/operator`, `bin/admission`.
- `make kind-up` — create Kind cluster.
- `make platform-up` — apply namespace + CRDs; install chart (replicas=0).
- `make manager-up` — compose DB + manager.
- `make operator-up` — install operator via Helm.
- `make kind-load-operator` — build local operator image, load to Kind, install chart.
- `make test-e2e` — run all end‑to‑end tests in `hack/e2e`.
- `make down` — tear down Kind and compose.
- `make prod-install` — opinionated production install (cert‑manager, metrics‑server, ExternalDNS, kubeOP chart with prod values).
- `make sync-policy` — sync admission policy env to cluster.

## Configuration
Environment variables drive both Manager and tests. See `env.example` for the
complete list and defaults. Key variables:
- `KUBEOP_DB_URL` — PostgreSQL DSN (required).
- `KUBEOP_REQUIRE_AUTH` — enable JWT auth for API (default false for local).
- `KUBEOP_JWT_SIGNING_KEY` — base64 HS256 key (required if auth enabled).
- `KUBEOP_KMS_MASTER_KEY` — base64 32‑byte key for envelope encryption. For local
  dev, set `KUBEOP_DEV_INSECURE=true` to auto‑generate a temp key.
- `KUBEOP_HTTP_ADDR` — Manager listen address (default `:8080`).
- `KUBEOP_IMAGE_ALLOWLIST` — comma‑sep allowed image registries (admission).
- `KUBEOP_EGRESS_BASELINE` — comma‑sep allowed egress CIDRs baseline (admission).
- `KUBEOP_BOOTSTRAP_ON_START` — Manager auto‑applies CRDs/operator from local dirs
  (`KUBEOP_BOOTSTRAP_CRDS_DIR`, `KUBEOP_BOOTSTRAP_OPERATOR_DIR`) to `$KUBECONFIG`.

## API Overview
- Health and introspection: `/healthz`, `/readyz`, `/version`, `/metrics`, `/openapi.json`.
- Tenancy:
  - `POST /v1/tenants`, `GET|DELETE /v1/tenants/{id}`
  - `POST /v1/projects`, `GET|DELETE /v1/projects/{id}`
  - `POST /v1/apps`, `GET|DELETE /v1/apps/{id}`
- Usage and billing:
  - `POST /v1/usage/ingest` (hourly points), `GET /v1/usage/snapshot` (rollup)
  - `GET /v1/invoices/{tenantID}` (lines + subtotal)
- Clusters:
  - `POST /v1/clusters` (register kubeconfig; optional auto‑bootstrap)
  - `GET /v1/clusters/{id}/ready` (operator+admission ready and webhook CA set)
- Access:
  - `GET /v1/kubeconfigs/{namespace}` (project‑scoped kubeconfig)
  - `POST /v1/jwt/project` (mint short‑lived project‑scoped JWT)
- CronJobs (project‑scoped):
  - `POST /v1/cronjobs` (create), `GET /v1/cronjobs?projectID=...` (list)
  - `GET|DELETE /v1/cronjobs/{projectID}/{name}`
  - `POST /v1/cronjobs/{projectID}/{name}/run` (ad‑hoc job)

See `internal/api/openapi.json` and `api/openapi.yaml` for schema details.

## E2E Testing
- Kind‑based E2E assets in `e2e/` with bootstrap script `e2e/bootstrap.sh`.
- Run the full suite: `make test-e2e` (requires Docker, Kind, kubectl, Helm).
- Common tests (see `hack/e2e/`):
  - Admission denies disallowed image registry.
  - Operator in‑cluster endpoints (health/ready/version/metrics).
  - CronJobs create/list/run/delete within a project namespace.
  - Minimal end‑to‑end: CRDs applied, DNS/Cert mocks ready, app rollout.

Extended E2E:
- CI runs Kind E2E automatically.
- Real‑cluster E2E can be triggered via pipeline flag `E2E_REAL_CLUSTER=true`.
- All tests must pass on Kind and real clusters before tagging a release.

## Helm Charts
- Operator + Admission chart: `charts/kubeop-operator`.
- Values for Kind: `charts/kubeop-operator/values-kind.yaml`.
- Values for production: `charts/kubeop-operator/values-prod.yaml`.
- Example install (prod): `make prod-install` or run Helm directly.

## CI/CD
- Single workflow: `.github/workflows/ci.yaml` with stages: Test → Build → E2E → Push.
- Images pushed to GHCR.
- Tagging strategy:
  - Branch `main`: `latest`, `sha-<short>`, semantic tags (`ghcr.io/vaheed/kubeop/<pkg>`)
  - Branch `develop`: `dev`, `sha-<short>` (`ghcr.io/vaheed/kubeop/<pkg>-dev`)
- Branching model: feature branches → PRs into `develop`; never push to `main`.

## Testing Rules
- Unit tests next to packages as `*_test.go`. Use `tests/` only for black‑box tests.
- `go test ./... && make right` before committing.
- Postgres‑backed tests auto‑skip if DB unavailable; CI provides a Postgres service.
- Every new module/endpoint/logic should include tests.

## Coding & Style
- Follow Go idioms; keep commits focused and small.
- `make right` enforces formatting, vetting, tidy, and build checks.
- Avoid hard‑coded secrets; use environment variables (see `env.example`).
- Every service exposes `/healthz` and `/metrics`.
- Follow 12‑factor configuration principles.

## Production Notes
- Use `make prod-install` as a reference for installing cert‑manager,
  metrics‑server, ExternalDNS (PowerDNS), then kubeOP operator+admission.
- Set admission policy via env: `KUBEOP_IMAGE_ALLOWLIST`, `KUBEOP_EGRESS_BASELINE` and
  apply to cluster Deployment using `make sync-policy`.

## Troubleshooting
- Manager not ready: verify DB connectivity (`KUBEOP_DB_URL`) and KMS key presence
  (or set `KUBEOP_DEV_INSECURE=true` for local).
- Operator/admission not ready: run `bash e2e/bootstrap.sh`, then check
  `kubectl -n kubeop-system get deploy` and webhooks’ CABundle.
- CronJobs 404: ensure you are running the current Manager build (local binary or
  a fresh image), and the tenant is associated with a registered cluster.

## Contributing
- Use conventional commits: `feat:`, `fix:`, `chore:`, `docs:`, `refactor:`.
- Keep PRs small and single‑purpose. Always target `develop`.
- Update docs under `docs/` when behavior changes. Internal notes go to
  `docs/internal/WORKLOG.md`. For major features, update `docs/ROADMAP.md`,
  `CHANGELOG.md`, and bump `VERSION`.

## License
MIT — see `LICENSE`.

---

Helpful entry points:
- Getting started: `docs/getting-started.md`
- Config reference: `docs/config.md`
- API reference: `docs/api.md`
- CRDs: `docs/crds.md`
- Troubleshooting: `docs/troubleshooting.md`
