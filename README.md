KubeOP — Out-of-Cluster Control Plane (Go)

Overview

- Production-ready starter for an out-of-cluster control plane in Go.
- Manages multiple Kubernetes clusters via uploaded kubeconfigs.
- Exposes a REST API on port 8080.
- Persists state in PostgreSQL (users, clusters, projects).
- Secured with an admin JWT and at-rest encryption for kubeconfigs.

Quickstart

- Prereqs: Docker and Docker Compose.
- Clone this repo, then run:
- `docker compose up -d --build`
- Health: `curl http://localhost:8080/healthz` and `curl http://localhost:8080/readyz`.
- Version: `curl http://localhost:8080/v1/version`.

Auth

- All `/v1/*` endpoints require an admin JWT (`Authorization: Bearer <token>`).
- Sign tokens with `HS256` and include claim `{"role":"admin"}`.
- Set `ADMIN_JWT_SECRET` in environment. For development, you can set `DISABLE_AUTH=true` to disable.

Register a Cluster

- The API requires the kubeconfig to be provided as base64 in the field `kubeconfig_b64`.
- Create base64 and register:
  - Linux/macOS: `B64=$(base64 -w0 < kubeconfig)`
  - Windows (PowerShell): `$B64 = [Convert]::ToBase64String([IO.File]::ReadAllBytes('kubeconfig'))`
  - `curl -X POST http://localhost:8080/v1/clusters -H "Authorization: Bearer <token>" -H "Content-Type: application/json" -d "$(jq -n --arg name 'talos-stage' --arg b64 "$B64" '{name:$name,kubeconfig_b64:$b64}')"`
- List:
- `curl -H "Authorization: Bearer <token>" http://localhost:8080/v1/clusters`

Users & Projects (default: shared user namespace)

- Tenancy modes overview:
  - Shared user namespace (default): one K8s namespace per user; all that user’s projects live inside it. Bootstrap once per cluster per user. Project responses do not include kubeconfig; reuse the user kubeconfig.
  - Per-project namespaces (optional): one K8s namespace per project; each project response includes a project-scoped kubeconfig.
- Bootstrap user namespace and get kubeconfig (shared mode):
  - `curl -s -X POST http://localhost:8080/v1/users/bootstrap -H "Authorization: Bearer <token>" -H "Content-Type: application/json" -d '{"userId":"<user-uuid>","clusterId":"<cluster-uuid>"}'`
  - Or create/reuse by email: `curl -s -X POST http://localhost:8080/v1/users/bootstrap -H "Authorization: Bearer <token>" -H "Content-Type: application/json" -d '{"name":"Alice","email":"alice@example.com","clusterId":"<cluster-uuid>"}'`
  - Set `PROJECTS_IN_USER_NAMESPACE=true` (default) to place multiple projects into the user namespace. Reuse the user kubeconfig for all projects.
- Create project in user namespace (shared mode):
  - `curl -s -X POST http://localhost:8080/v1/projects -H "Authorization: Bearer <token>" -H "Content-Type: application/json" -d '{"userId":"<user-uuid>","clusterId":"<cluster-uuid>","name":"demo"}'`
  - Response omits kubeconfig in shared mode.

Per-project namespaces (optional)

- Set `PROJECTS_IN_USER_NAMESPACE=false` to create a dedicated namespace and receive a project-scoped kubeconfig on `POST /v1/projects`.

Tenancy modes: end-to-end flows

- Shared user namespace (default):
  - 1) Register cluster → get `clusterId`.
  - 2) Bootstrap user: `POST /v1/users/bootstrap` with either `{userId, clusterId}` or `{name, email, clusterId}` → response returns `user.id`, `namespace`, and `kubeconfig_b64` for the user namespace.
  - 3) Create projects: `POST /v1/projects` with `{userId, clusterId, name}` → response does not include kubeconfig; keep using the user kubeconfig.
  - 4) Manage quotas at the user namespace level (project-level suspend/quota endpoints are not applicable in shared mode).
- Per-project namespaces:
  - 1) Set `PROJECTS_IN_USER_NAMESPACE=false` in env.
  - 2) Register cluster → get `clusterId`.
  - 3) Create project: `POST /v1/projects` with either `{userId, clusterId, name}` or `{userEmail, userName, clusterId, name}` → response includes `kubeconfig_b64` for that project namespace.
  - 4) Manage per-project quotas and use suspend/unsuspend when needed.

Users (Shared Namespace Mode)

- Bootstrap user namespace and get kubeconfig:
  - `curl -s -X POST http://localhost:8080/v1/users/bootstrap -H "Authorization: Bearer <token>" -H "Content-Type: application/json" -d '{"userId":"<user-uuid>","clusterId":"<cluster-uuid>"}'`
  - Or create/reuse by email: `curl -s -X POST http://localhost:8080/v1/users/bootstrap -H "Authorization: Bearer <token>" -H "Content-Type: application/json" -d '{"name":"Alice","email":"alice@example.com","clusterId":"<cluster-uuid>"}'`
  - Set `PROJECTS_IN_USER_NAMESPACE=true` to place multiple projects into that user namespace. In this mode, project responses omit kubeconfig; reuse the user kubeconfig.
- Status: `curl -s -H "Authorization: Bearer <token>" http://localhost:8080/v1/projects/<project-id>`
- Quota (per-project mode): `curl -s -X PATCH -H "Authorization: Bearer <token>" -H "Content-Type: application/json" -d '{"overrides":{"pods":"100"}}' http://localhost:8080/v1/projects/<project-id>/quota`

Local Development (without Docker)

- Copy `.env.example` to `.env` and adjust values, or export env vars.
- Start Postgres locally (see `docker-compose.yml` for defaults) or use `DATABASE_URL`.
- Build and run:
- `go mod download && go run ./cmd/api`

Notes

- Talos support: Any CNCF-compliant cluster works via kubeconfig upload. Talos kubeconfigs work today; CloudStack K8s is planned next.
- Config: All settings are environment-driven; optional `CONFIG_FILE` can point to a YAML file to overlay defaults.
- Migrations: Automatically run on startup (embedded via Go `embed`).

Docs

- See below for a brief of each document. Files are under `docs/` unless noted.
- Prefer a website? GitHub Actions auto-publishes `docs/` to GitHub Pages (gh-pages branch) using Docsify. See docs/OPERATIONS.md:1 for setup.

Documents Summary

- docs/ARCHITECTURE.md:1 — High-level design, package layout, data flow, and an embedded Mermaid diagram of the system.
- docs/API_REFERENCE.md:1 — REST API endpoints, auth requirements, detailed curl examples (with and without auth), and how to register clusters using `kubeconfig_b64`.
- docs/ENVIRONMENT.md:1 — Environment variables, defaults, and example DSNs for local and Docker setups.
- docs/OPERATIONS.md:1 — How to run locally and with Docker Compose, migrations, logs, backups, scaling, health/readiness, and config.
- docs/SECURITY.md:1 — Admin JWT model, encryption-at-rest details, secret rotation guidance, transport and hardening notes.
- docs/ROADMAP.md:1 — Phased plan for upcoming features and improvements.
- AGENTS.md:1 — Repository rules for docs/tests layout, migrations naming, CI requirements, coding standards, and agent workflow.
- docs/KUBECONFIG.md:1 — How namespace-scoped kubeconfigs are minted and returned base64.
- docs/TENANCY.md:1 — User→Project→Namespace model, lifecycle (create/suspend/unsuspend/quota/update/delete), and ENV knobs.
- docs/ISOLATION.md:1 — NetworkPolicy and Pod Security Admission strategy with configurable label selectors.
- docs/QUOTAS.md:1 — Default quotas/limits and how to override via API.
- docs/KUBECONFIG.md:1 — How kubeconfigs are minted per project and returned base64.
- docs/openapi.yaml:1 — OpenAPI 3 specification for the API. View it at `docs/openapi.html` (ReDoc) or import the YAML into your API client.

Project Rules

- See AGENTS.md:1 for repository-wide rules on docs, tests, migrations, CI, and agent workflow.

Tests

- Unit tests live under `testcase/` and cover config, auth middleware, router basics, and crypto utils.
- Run locally: `go test ./...`
- CI: `.github/workflows/ci.yml` runs vet, build, and `go test ./...` on every push and PR before building/pushing images.

License

- This project is licensed under the MIT License. See LICENSE:1.

Kubeconfig Base64 Notes

- The API requires `kubeconfig_b64` (base64) when registering clusters. Plaintext `kubeconfig` is not accepted by project policy.
- Linux/macOS: `base64 -w0 < kubeconfig`
- Windows (PowerShell): `[Convert]::ToBase64String([IO.File]::ReadAllBytes('kubeconfig'))`
