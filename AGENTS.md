Agent And Project Rules

Scope

- This file governs the entire repository. Any agent or contributor must follow these rules for all changes within this repo.

Directory Layout

- Documentation: all Markdown docs live under `docs/`. Keep only `README.md` at repo root.
  - Static docs site: `docs/index.html` (Docsify) renders the markdown for GitHub Pages. A GitHub Action publishes `docs/` to the `gh-pages` branch.
- Tests: place unit tests under `testcase/` using Go `*_test.go` files. Prefer package-level tests that import internal packages.
- Migrations: SQL migrations are embedded from `internal/store/migrations` and use golang-migrate naming `NNNN_name.up.sql` and `NNNN_name.down.sql`.
- Code: application code under `internal/` and entrypoint under `cmd/api`.

API And Behavior

- Auth: Admin endpoints require a Bearer token signed with `ADMIN_JWT_SECRET` and claim `{"role":"admin"}` unless `DISABLE_AUTH=true`.
- Clusters: POST `/v1/clusters` must receive `kubeconfig_b64` (base64). Plaintext `kubeconfig` is not allowed by policy.
- Users: Prefer `POST /v1/users/bootstrap` to provision the user namespace, quotas, and kubeconfig in one call. Projects default to the user namespace (`PROJECTS_IN_USER_NAMESPACE=true`).
- Health: `/healthz` and `/readyz` must remain stable and fast.
- Version: `/v1/version` returns build metadata.

Coding Standards

- Go version: match `go.mod`. Use stdlib first; avoid unnecessary dependencies.
- Error handling: wrap with context; return user-safe messages at API layer.
- Logging: use `log/slog` via `internal/logging`.
- Database: use `pgx` stdlib driver; keep queries simple and parameterized.
- Crypto: use `internal/crypto` for encryption; never reimplement primitives.

Documentation Rules

- Keep `README.md` current whenever features, setup, or commands change.
- Update `docs/API_REFERENCE.md` for any API additions/changes; include curl examples with and without auth when applicable.
- Update `docs/ARCHITECTURE.md` (including the Mermaid diagram) when structure or data flow changes.
- Document env vars in `docs/ENVIRONMENT.md` and operational notes in `docs/OPERATIONS.md`.

Testing Rules

- Every new or modified package/function should have or update tests in `testcase/`.
- Unit tests must not require external services; mock or limit scope. Integration tests can be added in a separate job if needed.
- Run `go vet` and `go test ./...` locally before opening PRs.

CI Rules

- CI must run `go vet`, `go build`, and `go test ./...` on every push and PR before image builds.
- Do not bypass the `test` job for Docker publishing.
- CI enforces AGENTS.md:
   - If any code changes under `internal/` or `cmd/`, at least one Markdown doc must be updated (under `docs/` or `README.md`) and at least one test file must be updated/added under `testcase/`.
   - CI fails if any `*.md` exists outside `docs/` (except `README.md` and `AGENTS.md`).

Migrations Rules

- Place new migrations in `internal/store/migrations` as up/down pairs with incremental numeric prefixes.
- Never edit applied migrations; add new ones to change schema.

Agent Workflow

- On every task, first scan the repo and this `AGENTS.md`.
- If the task changes API, config, or behavior:
  - Update code and corresponding tests under `testcase/`.
  - Update `README.md` and relevant docs under `docs/`.
- Keep changes minimal and focused on the task; avoid unrelated refactors.

Conventions

- Paths in communication should include file and start line (e.g., `docs/API_REFERENCE.md:1`).
- Avoid adding license headers unless specifically requested.
- Follow existing naming and structure; don’t rename files without need.
