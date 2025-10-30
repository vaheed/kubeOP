# kubeOP

kubeOP is a multi-tenant application platform for Kubernetes that combines a PostgreSQL-backed management API, controller-runtime operator, and admission webhooks. It enables platform teams to onboard clusters, manage tenants/projects/apps, deliver workloads from multiple sources, and generate usage/invoice analytics from a single control plane.

## Why kubeOP?

- **End-to-end automation** – Register a cluster and kubeOP bootstraps namespaces, CRDs, operator, admission, policies, and quotas via server-side apply.
- **Multi-source delivery** – Support image, Git, Helm, and raw manifest rollouts with revision tracking, hooks, and DNS/TLS orchestration.
- **Guardrails by default** – Enforce namespace ownership, quota inheritance, registry allowlists, and network egress baselines with HA admission webhooks.
- **Operational insight** – `/v1/usage/snapshot`, `/v1/invoices/{tenant}`, and `/v1/analytics/summary` expose consumption, billing, and delivery mix data for every tenant.

## Quickstart

1. Clone and install dependencies:
   ```bash
   git clone https://github.com/vaheed/kubeOP.git
   cd kubeOP
   npm install
   ```
2. Spin up the local platform (Kind + Docker Compose + operator):
   ```bash
   make kind-up
   make platform-up
   make manager-up
   make operator-up
   ```
   The manager automatically bootstraps every registered cluster with the kubeOP namespace, CRDs, RBAC, admission stack, and the
   kubeop-operator Deployment using the GitHub Container Registry image (`ghcr.io/vaheed/kubeop/operator:latest`) with
   `imagePullPolicy: Always`.
3. Run the end-to-end scenario:
   ```bash
   make test-e2e
   ```
   Artifacts (`artifacts/usage.json`, `artifacts/invoice.json`, `artifacts/analytics.json`) capture the usage, invoice, and analytics exports produced during the run.
4. Tear down:
   ```bash
  make down
  ```

## Service Health, Readiness, Metrics

Every service exposes common endpoints:

- Manager: `/healthz`, `/readyz`, `/version`, `/metrics` on `http://localhost:18080`
- Operator: `/healthz`, `/readyz`, `/version`, `/metrics` on its Pod port `8082` (Service `kubeop-operator-metrics` exposes metrics)

`/version` returns `{service, version, gitCommit}`. `/metrics` includes Go/process and kubeOP domain metrics (HTTP latencies, DB/webhook, business counters).

## E2E Resilience

The E2E tests inject outages (manager down, DB down) and assert recovery without drift. Artifacts (logs, events, resources) are saved under `artifacts/` and uploaded by CI.

## Documentation

## Documentation

The full documentation lives under [`docs/`](./docs/) and is published via VitePress:

- [Roadmap](./docs/roadmap.md) – Phase A milestones with code, test, and manifest links.
- [Getting Started](./docs/getting-started.md) – Local and production deployment.
- [Bootstrap Guide](./docs/bootstrap-guide.md) – Copy-paste cluster → tenant → project → app → DNS/TLS → invoice flows.
- [Operations](./docs/operations.md) – Upgrades, backups, RBAC, and day-2 procedures.
- [Delivery Workflows](./docs/delivery.md) – Delivery kinds, hooks, revisions, rollback.
- [API Reference](./docs/api-reference.md) – REST endpoints and CRDs.
- [Production Hardening](./docs/production-hardening.md) – HA, sizing, probes, metrics, SLOs.

Launch the docs locally:
```bash
npm run docs:dev
```

The dev server serves the site at [http://localhost:5173/kubeOP/](http://localhost:5173/kubeOP/) to mirror the GitHub Pages base
path. Set `DOCS_BASE=/` before running the command if you prefer to mount the docs at the root during local development.

The published documentation is available at https://vaheed.github.io/kubeOP/ with the same scoped base path, ensuring CSS and
assets resolve correctly on GitHub Pages.

## Production Docker Compose

The repository ships a production-oriented [`docker-compose.yml`](./docker-compose.yml) that provisions PostgreSQL alongside the
kubeOP manager image from GHCR. Provide secrets via environment variables and bring the stack online:

```bash
export KUBEOP_KMS_MASTER_KEY="$(openssl rand -base64 32)"
export KUBEOP_JWT_SIGNING_KEY="$(openssl rand -base64 32)"
docker compose up -d
```

`KUBEOP_OPERATOR_IMAGE` and `KUBEOP_OPERATOR_IMAGE_PULL_POLICY` default to `ghcr.io/vaheed/kubeop/operator:latest` and
`Always`, ensuring every cluster bootstrap pulls the latest operator image. Override the values if you need to pin to a
particular release tag, including the development builds published at `ghcr.io/vaheed/kubeop/operator-dev:dev`. The compose
stack builds the manager locally but also honours the published development image `ghcr.io/vaheed/kubeop/manager-dev:dev` for
consistent testing against `develop` snapshots.

## Development

- Environment variables live in [`.env.example`](./.env.example). Copy to `.env` as needed.
- Run formatting, vetting, and build checks:
  ```bash
  make right
  ```
- Run Go tests (Postgres-backed suites expect `KUBEOP_DB_URL`):
  ```bash
  go test ./...
  ```
- Package the operator Helm chart without installing the Helm CLI:
  ```bash
  make helm-package
  # or go run ./tools/helmchart --chart charts/kubeop-operator --destination dist/charts
  ```
  The command lints the chart and writes `kubeop-operator-<version>.tgz` to `dist/charts/`.
- Regenerate documentation diagrams manually when `.mmd` sources change. The
  repository still ships the Mermaid renderer (`npm run diagrams:build`) for
  maintainers, but CI no longer enforces or re-renders the SVG outputs. Install
  dependencies with `npm ci` and run the generator locally if you need to
  update the assets:
  ```bash
  npm run diagrams:build
  ```
  Puppeteer requires a Chromium runtime. Install the packages that match your
  distribution before running the generator (for Debian/Ubuntu the list
  includes `libatk1.0-0{,t64}`, `libatk-bridge2.0-0{,t64}`, `libatspi2.0-0{,t64}`,
  `libgtk-3-0{,t64}`, `libpango-1.0-0`, `libpangocairo-1.0-0`, `libcairo2`,
  `libnss3`, `libnspr4`, `libcups2{,t64}`, `libxss1`, `libxcomposite1`,
  `libxcursor1`, `libxdamage1`, `libxrandr2`, `libxkbcommon-x11-0`,
  `libxinerama1`, `libxfixes3`, `libdrm2`, `libdrm-amdgpu1`, `libdrm-intel1`,
  `libgbm1`, and `libasound2{,t64}`).

## Testing & CI

The GitHub Actions pipeline (`.github/workflows/ci.yaml`) installs dependencies, runs lint/tests, executes `make test-e2e`, and uploads artifacts/logs. A dedicated discovery step scans `cmd/<service>/main.go` entrypoints and ensures matching Dockerfiles exist under `deploy/`. Every detected binary is built in a matrix, publishing:

- `main` → `ghcr.io/vaheed/kubeop/<service>` with `latest`, semantic, and `sha-<short>` tags for `linux/amd64` and `linux/arm64`.
- `develop` → `ghcr.io/vaheed/kubeop/<service>-dev` with `dev` and `sha-<short>` tags for `linux/amd64`.
- feature branches → `ghcr.io/vaheed/kubeop/<service>-dev:pr-<short>` for preview builds.

List the services that will be built by running:

```bash
go run ./tools/buildmeta --format=names
```

Local workflows should mirror CI before opening pull requests.

## Licensing

kubeOP is released under the [MIT License](./LICENSE).
