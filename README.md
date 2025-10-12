KubeOP — Out-of-Cluster Control Plane (Go)

Overview

- Production-ready starter for an out-of-cluster control plane in Go.
- Manages multiple Kubernetes clusters via uploaded kubeconfigs.
- Exposes a REST API on port 8080.
- Persists state in PostgreSQL (users, clusters, projects).
- Secured with an admin JWT and at-rest encryption for kubeconfigs.
- Supports app deployments (image/manifests/helm), flavors, CI webhooks, logs streaming, Prometheus metrics, config/secret attachment endpoints, and ENV-driven ingress/LB (MetalLB default).

Before you begin

1. Install Docker and Docker Compose (or run everything locally with Go + Postgres).
2. Clone the repository and copy `.env.example` to `.env` if you need to override defaults.
3. Generate an admin JWT signed with `ADMIN_JWT_SECRET` and claim `{ "role": "admin" }` for API requests.
4. Export helper variables for curl commands:
   ```bash
   export TOKEN="<admin-jwt>"
   export AUTH_H="-H 'Authorization: Bearer $TOKEN'"
   ```

Quickstart (5-step path)

1. **Start the stack**
   ```bash
   docker compose up -d --build
   ```
2. **Check health**
   ```bash
   curl http://localhost:8080/healthz
   curl http://localhost:8080/readyz
   curl $AUTH_H http://localhost:8080/v1/version
   ```
3. **Register a cluster (base64 kubeconfig required)**
   ```bash
   B64=$(base64 -w0 < kubeconfig)                     # macOS/Linux
   # Windows PowerShell: $B64 = [Convert]::ToBase64String([IO.File]::ReadAllBytes('kubeconfig'))
   curl -s $AUTH_H -H 'Content-Type: application/json' \
     -d "$(jq -n --arg name 'talos-stage' --arg b64 "$B64" '{name:$name,kubeconfig_b64:$b64}')" \
     http://localhost:8080/v1/clusters
   ```
4. **Bootstrap a user namespace (shared mode default)**
   ```bash
   curl -s $AUTH_H -H 'Content-Type: application/json' \
     -d '{"name":"Alice","email":"alice@example.com","clusterId":"<cluster-id>"}' \
     http://localhost:8080/v1/users/bootstrap
   ```
   *Save `user.id`, `namespace`, and decode `kubeconfig_b64` to `user.kubeconfig` for kubectl access.*
5. **Create a project and deploy an app**
   ```bash
   curl -s $AUTH_H -H 'Content-Type: application/json' \
     -d '{"userId":"<user-id>","clusterId":"<cluster-id>","name":"demo"}' \
     http://localhost:8080/v1/projects

   curl -s $AUTH_H -H 'Content-Type: application/json' \
     -d '{"name":"web","image":"nginx:1.27","ports":[{"containerPort":80,"servicePort":80,"serviceType":"LoadBalancer"}]}' \
     http://localhost:8080/v1/projects/<project-id>/apps
   ```
   *Access via wildcard ingress (`http://web.<namespace>.<PAAS_DOMAIN>`) or run `KUBECONFIG=./user.kubeconfig kubectl -n <namespace> get svc web -o wide` to find the external IP.*
API walk-through

- Follow `docs/QUICKSTART_API.md` for a scripted flow that covers creating/deleting users, projects, and apps with copy-ready commands.
- `docs/QUICKSTART_APPS.md` focuses on app deployments (image/helm/git) and includes log and access examples.

Config & Secret attachments (step-by-step)

1. **Create a ConfigMap or Secret** in the project namespace via kubectl or the `/v1/projects/{id}/configs|secrets` APIs.
2. **Attach all keys**
   ```bash
   curl -s $AUTH_H -H 'Content-Type: application/json' \
     -d '{"name":"app-config"}' \
     http://localhost:8080/v1/projects/<project-id>/apps/<app-id>/configs/attach
   ```
3. **Attach specific keys with an optional prefix**
   ```bash
   curl -s $AUTH_H -H 'Content-Type: application/json' \
     -d '{"name":"app-config","keys":["LOG_LEVEL"],"prefix":"APP_"}' \
     http://localhost:8080/v1/projects/<project-id>/apps/<app-id>/configs/attach
   ```
4. **Attach secrets the same way** using `/secrets/attach`.
5. **Detach when finished**; this removes `envFrom` and keyed env vars so pods restart cleanly.
   ```bash
   curl -s $AUTH_H -H 'Content-Type: application/json' \
     -d '{"name":"app-config"}' \
     http://localhost:8080/v1/projects/<project-id>/apps/<app-id>/configs/detach
   ```
   *Secrets detach via `/secrets/detach`.*

Auth essentials

1. Set `ADMIN_JWT_SECRET` in the environment for both the API and any tooling generating admin tokens.
2. Sign tokens with `HS256` and include the claim `{ "role": "admin" }`.
3. For development-only testing, export `DISABLE_AUTH=true` to skip auth entirely.
Tenancy cheat sheet

- **Shared user namespace (default, `PROJECTS_IN_USER_NAMESPACE=true`)**
  1. Register cluster → `clusterId`
  2. Bootstrap user → decode `kubeconfig_b64` to `user.kubeconfig`
  3. Create projects with `{ userId, clusterId, name }` → reuse the user kubeconfig for kubectl
  4. Manage quotas at the namespace level

- **Per-project namespaces (`PROJECTS_IN_USER_NAMESPACE=false`)**
  1. Register cluster → `clusterId`
  2. Create project with user reference → response includes project-scoped `kubeconfig_b64`
  3. Use `/quota`, `/suspend`, `/unsuspend` to control each namespace independently

Everyday curl references

- List users: `curl -s $AUTH_H http://localhost:8080/v1/users | jq`
- List clusters: `curl -s $AUTH_H http://localhost:8080/v1/clusters | jq`
- List projects: `curl -s $AUTH_H http://localhost:8080/v1/projects | jq`
- List a user’s projects: `curl -s $AUTH_H http://localhost:8080/v1/users/<user-id>/projects | jq`

Local development (Go without Docker)

1. Start Postgres (see `docker-compose.yml` for default credentials) or point `DATABASE_URL` to a running instance.
2. Export env vars or load `.env`.
3. Install dependencies and run the API:
   ```bash
   go mod download
   go run ./cmd/api
   ```

Operational notes

- Talos support: any CNCF-compliant cluster works via kubeconfig upload; Talos is tested today.
- Configuration: all settings are environment-driven; optionally point `CONFIG_FILE` at a YAML overlay.
- Migrations: embedded migrations run automatically on startup.
Documentation map

- docs/ARCHITECTURE.md — System diagram, package layout, and data flow.
- docs/API_REFERENCE.md — REST endpoints with numbered walkthroughs and curl snippets.
- docs/QUICKSTART_API.md — Copy-ready flow: register cluster → bootstrap user → create project/app → clean up.
- docs/QUICKSTART_APPS.md — App-centric quickstart (image, Helm, Git) plus attachment walkthrough.
- docs/APPS.md — Deep dive into deployment options, app management, and config/secret handling.
- docs/ENVIRONMENT.md — Environment variables with defaults and suggested values.
- docs/OPERATIONS.md — Running locally, via Docker Compose, maintenance, migrations, backups, scaling, and health checks.
- docs/SECURITY.md — JWT model, encryption-at-rest, rotation guidance, and hardening tips.
- docs/ROADMAP.md — Ordered phases with explicit deliverables.
- docs/KUBECONFIG.md — How namespace-scoped kubeconfigs are minted and returned base64.
- docs/TENANCY.md — User → Project → Namespace lifecycle with env knobs.
- docs/ISOLATION.md — NetworkPolicy defaults and PSA expectations.
- docs/QUOTAS.md — Default quotas and override workflow.
- docs/FLAVORS.md — Built-in flavors and override guidance.
- docs/INGRESS_LB.md — Wildcard ingress, MetalLB settings, and DNS automation.
- docs/CI_WEBHOOKS.md — Git webhook configuration and payload schema.
- docs/METRICS.md — `/metrics` output and scraping tips.
- docs/CHANGELOG.md — Release history (Keep a Changelog).
- docs/openapi.yaml — OpenAPI spec (view via `docs/openapi.html` or import to an API client).

Project rules

- Review AGENTS.md for repository-wide coding, docs, and testing requirements before submitting changes.

Testing

- Unit tests live under `testcase/`.
- Run locally: `go test ./...`
- CI (`.github/workflows/ci.yml`) runs `go vet`, `go build`, `go test ./...`, and uploads the compiled API binary on every push and PR.

License

- MIT License — see `LICENSE` for the full text.

Kubeconfig base64 helpers

- The API only accepts `kubeconfig_b64`.
- macOS/Linux: `base64 -w0 < kubeconfig`
- Windows PowerShell: `[Convert]::ToBase64String([IO.File]::ReadAllBytes('kubeconfig'))`
