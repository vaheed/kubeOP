# kubeOP

[![GitHub Actions build status for kubeOP](https://github.com/vaheed/kubeOP/actions/workflows/ci.yml/badge.svg)](https://github.com/vaheed/kubeOP/actions/workflows/ci.yml "View the latest CI workflow run")
[![Hosted documentation status for kubeOP](https://img.shields.io/badge/docs-vitepress-blue.svg)](https://vaheed.github.io/kubeOP "Open the published documentation site")

kubeOP is an out-of-cluster control plane for managing fleets of Kubernetes clusters. It exposes a single REST API for tenant onboarding, application delivery, and lifecycle automation while a lightweight operator reconciles workloads inside each managed cluster.

## Highlights

- **Unified API** – register clusters, bootstrap tenants, and deploy applications without installing per-cluster control planes.
- **Deterministic automation** – every deployment emits canonical `kubeop.*` labels (such as `kubeop.app.id`) and persists release metadata so rollouts stay auditable.
- **Safe multi-cluster operations** – encrypted kubeconfigs, health scheduling, and structured project logging provide a clear operational picture.
- **Batteries included** – optional DNS automation, ingress guardrails, and opinionated resource quotas are available through configuration.

## Architecture

![kubeOP control plane architecture diagram showing API, scheduler, PostgreSQL, and in-cluster operator components](docs/media/architecture.svg)

The API binary (`cmd/api`) handles authentication, validation, and audit logging. Business logic in `internal/service` coordinates PostgreSQL persistence (`internal/store`), Kubernetes interactions (`internal/kube`), and delivery engines (`internal/delivery`). A background scheduler records cluster health summaries, and the `kubeop-operator` (bundled in this repo) reconciles rendered manifests inside each managed cluster. See [`docs/ARCHITECTURE.md`](docs/ARCHITECTURE.md) for the end-to-end flow.

## Quickstart (10 minutes)

You need Docker, Docker Compose, and `jq`.

1. **Clone and prepare local overrides**

   ```bash
   git clone https://github.com/vaheed/kubeOP.git
   cd kubeOP
   cp docs/examples/docker-compose.env .env
   mkdir -p logs
   ```

2. **Launch the stack**

   ```bash
   docker compose up -d --build
   ```

   The API listens on `http://localhost:8080`. Logs stream to `./logs`.

3. **Check health**

   ```bash
   curl http://localhost:8080/healthz
   curl http://localhost:8080/readyz
   curl http://localhost:8080/v1/version | jq
   ```

   `/v1/version` returns immutable build metadata (version, commit, date) for kubectl-style upgrade checks.

4. **Authenticate (once)**

   ```bash
   export KUBEOP_TOKEN="<admin-jwt>"
   export KUBEOP_AUTH_HEADER="-H 'Authorization: Bearer ${KUBEOP_TOKEN}'"
   ```

5. **Register a cluster**

   ```bash
   B64=$(base64 -w0 < /path/to/kubeconfig)
   curl -s ${KUBEOP_AUTH_HEADER} -H 'Content-Type: application/json' \
     -d "$(jq -n --arg name 'edge-cluster' --arg b64 "$B64" '{name:$name,kubeconfig_b64:$b64,"owner":"platform","environment":"staging","region":"eu-west"}')" \
     http://localhost:8080/v1/clusters | jq '.id'
   ```

6. **Bootstrap a tenant project**

   ```bash
   curl -s ${KUBEOP_AUTH_HEADER} -H 'Content-Type: application/json' \
     -d '{"name":"Alice","email":"alice@example.com","clusterId":"<cluster-id>"}' \
     http://localhost:8080/v1/users/bootstrap | jq
   ```

7. **Dry-run an application deployment**

   ```bash
   curl -s ${KUBEOP_AUTH_HEADER} -H 'Content-Type: application/json' \
     -d '{"projectId":"<project-id>","name":"web","image":"ghcr.io/example/web:1.2.3","ports":[{"containerPort":80,"servicePort":80,"serviceType":"LoadBalancer"}]}' \
     http://localhost:8080/v1/apps/validate | jq '.summary'
   ```

## Documentation

The full documentation set lives in [`docs/`](docs/) and is published as a VitePress site.

- [`docs/QUICKSTART.md`](docs/QUICKSTART.md) – step-by-step local bootstrap with curl samples.
- [`docs/INSTALL.md`](docs/INSTALL.md) – deployment guidance for Docker Compose and Kubernetes.
- [`docs/ENVIRONMENT.md`](docs/ENVIRONMENT.md) – exhaustive configuration reference.
- [`docs/ARCHITECTURE.md`](docs/ARCHITECTURE.md) – detailed component and data flow diagrams.
- [`docs/API.md`](docs/API.md) – endpoint overview and representative payloads.
- [`docs/STYLEGUIDE.md`](docs/STYLEGUIDE.md) – conventions for contributing documentation.
- [`docs/ROADMAP.md`](docs/ROADMAP.md) – current focus areas and upcoming milestones.

## Contributing and support

- Run `go fmt ./...`, `go vet ./...`, `go test ./...`, and `go test -count=1 ./testcase` before opening a PR. The CI workflow mirrors these checks.
- Update documentation, CHANGELOG entries, and tests whenever behaviour changes.
- Read [`CONTRIBUTING.md`](CONTRIBUTING.md), [`CODE_OF_CONDUCT.md`](CODE_OF_CONDUCT.md), and [`SUPPORT.md`](SUPPORT.md) for project policies.

kubeOP is licensed under the [Apache License 2.0](LICENSE).
