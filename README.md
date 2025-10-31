# kubeOP

[![CI](https://github.com/vaheed/kubeOP/actions/workflows/ci.yml/badge.svg)](.github/workflows/ci.yml)

Multi-tenant application platform for Kubernetes combining a PostgreSQL-backed management API, controller-runtime operator, and admission webhooks. Automates tenant/project/app lifecycle, delivery, guardrails, and billing analytics.

## Architecture

- Manager API (PostgreSQL): REST API for tenants/projects/apps, usage ingestion, invoices.
- Operator (controller-runtime): Reconciles CRDs, delivers apps (Image/Git/Helm/Raw), orchestrates DNS/TLS/quotas/network policy.
- Admission: Validates/mutates resources for safety and multi-tenancy.
- Observability: `/healthz`, `/readyz`, `/version`, `/metrics` exposed by all services; Prometheus scrapes via ServiceMonitor.

## Installation (Kind + Compose)

Prereqs: Go 1.24+, Docker, Kind, kubectl, Helm, Node 18+

> **CI-first:** operational bootstrap is managed inside the CI pipeline. The snippets below are provided for maintainers mirroring CI locally; no new local tooling was added.

```bash
kind create cluster --config hack/e2e/kind-config.yaml
kubectl apply -f deploy/k8s/crds
kubectl apply -f deploy/k8s/operator
```

Helm (OCI) install on any cluster:

```bash
helm install kubeop-operator oci://ghcr.io/vaheed/kubeop/charts/operator \
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

## Testing & CI

- **Unit (`-tags short`)** – hermetic, table-driven suites executed with `-race`, split deterministically via `tools/sharder`. Coverage chunks land in `unit-*.cov` artifacts.
- **Integration (`-tags integration`)** – Postgres-backed store and migration checks using GitHub Actions services. Fixtures tear down cleanly and skip when DSN is absent.
- **End-to-End (`-tags e2e`)** – Kind-per-shard bootstrap from `hack/e2e/run.sh`, CRD install, operator deployment, tenant→project→app flows, RBAC/quotas, network policy, delivery mocks, and billing/metrics verification. Artifacts: JUnit XML, coverage, Kind dumps, controller logs, `/tmp/test-artifacts/**`.

The CI workflow (`.github/workflows/ci.yml`) exposes jobs `lint`, `unit`, `integration`, `e2e`, `coverage-merge`, and `summary`. Each job restores Go module caches, installs required CLIs (kubectl, kind, helm, gotestsum), emits deterministic artifacts, retries flaky shards once, and fails when total coverage drops below 80%.

Utilities used by the workflow live in `tools/`:

- `tools/sharder` – deterministic package splitting.
- `tools/covermerge` – merges per-job `.cov` files.
- `tools/junitmerge` – consolidates job-level JUnit XML into a single report.

## Docs

- Site: GitHub Pages → https://vaheed.github.io/kubeOP/
- Local: `cd docs && npm ci && npm run docs:dev`
- Edit links and version switcher are enabled; sitemap and Open Graph metadata configured.

See:

- docs/guide/ (install, upgrade, rollback, kubeconfig, outbox/offline-first, drift)
- docs/reference/ (API, CRDs, health/version/metrics)
- docs/ops/ (runbooks, monitoring, alerting)
- docs/security/ (RBAC, KMS, cert rotation)

## Troubleshooting

- Operator not ready >90s: inspect CI artifact `logs/kubeop-system/deployment-kubeop-operator.log`.
- Integration failures: confirm Postgres service exposure (`KUBEOP_DB_URL`) and rerun the `integration` job with a clean database.
- Metrics missing: ensure ServiceMonitor enabled and Prometheus namespace allowed in NetworkPolicy; CI artifacts include `cr-*.yaml` snapshots for diffing.

## License

MIT – see LICENSE.
