Contributing
============

Getting Started
---------------
- Read `AGENTS.md` for repository rules (docs location, testing, OpenAPI updates).
- Install Go 1.22+, Docker (optional for Postgres), and make sure `go` is on your PATH.
- Run `go mod download` to fetch dependencies.

Branching & Commits
-------------------
- Fork and branch from `main` (e.g., `feature/<short-description>`).
- Keep commits focused; prefer descriptive messages (imperative mood).
- Rebase before submitting a PR to keep history clean.

Testing & Linting
-----------------
- Required commands before opening a PR:
  - `gofmt` (CI enforces formatting).
  - `go vet ./...`
  - `go test ./...`
  - Update/add tests under `testcase/` that cover behaviour changes.
- If you add tools (e.g., `staticcheck`, `golangci-lint`), document usage in this file and ensure CI runs them.

Documentation Expectations
--------------------------
- Update `README.md` plus relevant `docs/*.md` files when behaviour, configuration, or usage changes.
- For API changes, update `docs/openapi.yaml` and `docs/API_REFERENCE.md` in the same PR.
- Keep the documentation plan (`docs/DOCUMENTATION_PLAN.md`) accurate when adding/removing docs.

PR Checklist
------------
Tick each item before requesting review:

- [ ] Tests: `go test ./...` and targeted cases under `testcase/` updated.
- [ ] Linting: `gofmt` (no diff) and `go vet ./...` (optionally `staticcheck`).
- [ ] Docs: README and affected `docs/` files updated; roadmap/documentation plan touched if scope changed.
- [ ] CHANGELOG: add entry under `[Unreleased]` or new version and bump `internal/version/version.go` when releasing.
- [ ] CI: ensure `.github/workflows/ci.yml` still covers download → format → vet → build → test → artifact upload.
- [ ] Secrets: no secrets or credentials checked into the repo; rely on GitHub Actions secrets.

Communication
-------------
- Use Discussions/Issues for proposals; link them in PR descriptions when relevant.
- Document follow-up items in `docs/ROADMAP.md` Open Questions or next steps sections.
