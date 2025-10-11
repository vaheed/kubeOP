KubeOP — Out-of-Cluster Control Plane (Go)

Overview

- Production-ready starter for an out-of-cluster control plane in Go.
- Manages multiple Kubernetes clusters via uploaded kubeconfigs.
- Exposes a REST API on port 8080.
- Persists state in PostgreSQL (users, clusters, etc.).
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

Users

- Create: `curl -X POST http://localhost:8080/v1/users -H "Authorization: Bearer <token>" -H "Content-Type: application/json" -d '{"name":"Alice","email":"alice@example.com"}'`
- List: `curl -H "Authorization: Bearer <token>" http://localhost:8080/v1/users`

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

- See below for a brief of each document. Files are under `documents/` unless noted.

Documents Summary

- documents/ARCHITECTURE.md:1 — High-level design, package layout, data flow, and an embedded Mermaid diagram of the system.
- documents/API_REFERENCE.md:1 — REST API endpoints, auth requirements, detailed curl examples (with and without auth), and how to register clusters using `kubeconfig_b64`.
- documents/ENVIRONMENT.md:1 — Environment variables, defaults, and example DSNs for local and Docker setups.
- documents/OPERATIONS.md:1 — How to run locally and with Docker Compose, migrations, logs, backups, scaling, health/readiness, and config.
- documents/SECURITY.md:1 — Admin JWT model, encryption-at-rest details, secret rotation guidance, transport and hardening notes.
- documents/ROADMAP.md:1 — Phased plan for upcoming features and improvements.
- AGENTS.md:1 — Repository rules for docs/tests layout, migrations naming, CI requirements, coding standards, and agent workflow.

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

