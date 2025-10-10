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

- Example:
- `curl -X POST http://localhost:8080/v1/clusters -H "Authorization: Bearer <token>" -H "Content-Type: application/json" -d '{"name":"talos-stage","kubeconfig":"<paste kubeconfig contents>"}'`
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

- See `ARCHITECTURE.md`, `ENVIRONMENT.md`, `API_REFERENCE.md`, `SECURITY.md`, `OPERATIONS.md`, and `ROADMAP.md` for details.

