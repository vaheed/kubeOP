# kubeOP

[![CI](https://github.com/vaheed/kubeOP/actions/workflows/ci.yaml/badge.svg)](.github/workflows/ci.yaml)

Multi-tenant application platform for Kubernetes combining a PostgreSQL-backed management API, controller-runtime operator, and admission webhooks. Automates tenant/project/app lifecycle, delivery, guardrails, and billing analytics.

## Architecture

- Manager API (PostgreSQL): REST API for tenants/projects/apps, usage ingestion, invoices.
- Operator (controller-runtime): Reconciles CRDs, delivers apps (Image/Git/Helm/Raw), orchestrates DNS/TLS/quotas/network policy.
- Admission: Validates/mutates resources for safety and multi-tenancy.
- Observability: `/healthz`, `/readyz`, `/version`, `/metrics` exposed by all services; Prometheus scrapes via ServiceMonitor.

## Installation (Kind + Compose)

Prereqs: Go 1.24+, Docker, Kind, kubectl, Helm, Node 18+

```bash
make kind-up          # Kind cluster
bash e2e/bootstrap.sh # Namespace + CRDs + operator
docker compose up -d db manager  # Manager + Postgres
```

Smoke test:

```bash
curl -sf localhost:18080/healthz && kubectl -n kubeop-system get deploy/kubeop-operator
```

Helm (OCI) install on any cluster:

```bash
helm install kubeop-operator oci://ghcr.io/vaheed/kubeop/charts/kubeop-operator \
  -n kubeop-system --create-namespace --version $(cat VERSION)
```

## Service Endpoints

All services expose:

- /healthz → 200 when internal probes pass
- /readyz → 200 when dependencies ready (DB/KMS/clients)
- /version → JSON: {service, version, gitCommit, buildDate}
- /metrics → Prometheus metrics (go/process + domain: request latencies, DB/webhook metrics, reconciliation durations, drift)

Operator chart annotates metrics for scrape via Service/ServiceMonitor; NetworkPolicy restricts access (configurable).

## Security

- KMS envelope encryption (Manager): set `KUBEOP_KMS_MASTER_KEY`
- Non-root, distroless images; tags pinned by digest in CI; Cosign signing; SBOMs attached
- Optional mTLS for scrapers and inter-service traffic (see docs)

## E2E & CI

- Kind E2E: `make test-e2e` runs cluster → operator → tenant/project/app → DNS/TLS → usage → invoice.
- Outage injection (Manager/DB) verifies offline-first recovery; backlog drains ≤ 2m.
- Staticcheck, govulncheck, Trivy pass with no high/critical issues.
- CI: lint → unit → e2e(kind) → images(buildx+cosign+sbom+trivy) → charts(OCI) → docs(VitePress) → pages.

Artifacts (logs, replay reports, DB snapshot, metrics) are uploaded and retained 30 days.

### Step-by-step E2E (Local)

1) Prepare environment

- Copy env file and adjust values as needed
  ```bash
  cp env.example .env
  ```

2) Start manager + database (Compose)

- Start Postgres and manager using the base compose + dev overrides
  ```bash
  docker compose --env-file .env -f docker-compose.yml -f docker-compose.dev.yml up -d db
  docker compose --env-file .env -f docker-compose.yml -f docker-compose.dev.yml up -d manager
  ```

- Smoke test the API
  ```bash
  curl -sSf http://localhost:${KUBEOP_MANAGER_PORT:-18080}/healthz
  curl -sSf http://localhost:${KUBEOP_MANAGER_PORT:-18080}/version | jq
  ```

3) Bring up Kind and operator

- Create Kind and install the chart with mocks enabled
  ```bash
  make kind-up
  bash e2e/bootstrap.sh
  ```

4) Full E2E tests

- Run all E2E suites (smoke + end-to-end + negative admission)
  ```bash
  KUBEOP_E2E=1 go test ./hack/e2e -v -timeout=25m
  ```

5) Cleanup

  ```bash
  make down
  ```

## Docs

- Site: GitHub Pages → https://vaheed.github.io/kubeOP/
- Local: `cd docs && npm ci && npm run docs:dev`
- Edit links and version switcher are enabled; sitemap and Open Graph metadata configured.

See:

- docs/guide/ (install, upgrade, rollback, kubeconfig, outbox/offline-first, drift)
- docs/guide/kind-metrics-server.md – enable metrics-server on Kind to use HPA in dev
- docs/guide/production.md – production install with cert-manager (ACME) and ExternalDNS (PowerDNS)
- docs/reference/ (API, CRDs, health/version/metrics)
- docs/reference/policy.md – Manager-driven policy (allowlist/egress) API
- docs/ops/ (runbooks, monitoring, alerting)
- docs/security/ (RBAC, KMS, cert rotation)

## Troubleshooting

- Operator not ready >90s: check `kubectl -n kubeop-system logs deploy/kubeop-operator`
- Manager DB errors: verify `docker compose ps` and `KUBEOP_DB_URL`
- Metrics missing: ensure ServiceMonitor enabled and Prometheus namespace allowed in NetworkPolicy

## License

MIT – see LICENSE.
