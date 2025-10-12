# Contributing Guide

Thank you for investing time in KubeOP! This guide outlines how to set up your environment, run checks, and send high-quality contributions.

## Getting Started

- **Clone and fork**: Fork the repository on GitHub, then clone your fork locally.
- **Install tooling**:
  - Go `>= 1.22`
  - Docker & Docker Compose (for local stack)
  - GNU Make
- **Copy environment defaults**: `cp .env.example .env` and adjust values for your local database, JWT secret, and encryption keys.
- **Install dependencies**: `go mod download`

## Local Development Workflow

1. Start Postgres via Docker Compose or point `DATABASE_URL` at an existing instance.
2. Run migrations automatically by starting the API or manually via `go run ./cmd/api --migrate`.
3. Launch the API: `go run ./cmd/api`.
4. Export an admin JWT signed with `ADMIN_JWT_SECRET` and claim `{ "role": "admin" }` to call admin endpoints.
5. Use the curl snippets in `docs/QUICKSTART_API.md` and `docs/QUICKSTART_APPS.md` to exercise the API locally.

## Required Checks

Run these commands before opening a pull request:

```bash
# Format
make fmt

# Lint
go vet ./...

# Build
go build ./...

# Unit tests
go test ./...

# Focused testcase suite (mocks external dependencies)
go test -count=1 ./testcase
```

CI mirrors these steps and will fail if any command fails locally.

## Code Style & Expectations

- Follow the structure and conventions described in `AGENTS.md` and `docs/ARCHITECTURE.md`.
- Use `log/slog` via `internal/logging` for structured logs.
- Wrap errors with context using `fmt.Errorf("context: %w", err)` or helpers in `internal/errors`.
- Keep API request/response payloads consistent with `docs/openapi.yaml`.
- Place new documentation in `docs/` (README is the only markdown file allowed at the root).
- Update or add tests under `testcase/` whenever you modify code.

## Branching & Pull Requests

1. Create a feature branch off `main` (or the default branch): `git checkout -b feat/<short-description>`.
2. Commit logically grouped changes with descriptive messages.
3. Ensure the following before submitting the PR:
   - [ ] Tests, lint, build commands above all pass locally.
   - [ ] README and relevant docs updated when behavior changes.
   - [ ] CHANGELOG entry added under `## [Unreleased]` when applicable.
   - [ ] New configuration or API contracts documented under `docs/ENVIRONMENT.md` or `docs/API_REFERENCE.md`.
4. Push your branch and open a pull request using the provided template.
5. Be responsive to review feedback; follow up with additional commits rather than force-pushes when collaboration is active.

## Reporting Issues & Requesting Features

- **Bug reports**: Include environment details, reproduction steps, and expected vs. actual behavior. Use the `Bug report` template for structure.
- **Feature requests**: Outline the problem, proposed solution, alternatives considered, and any additional context in the `Feature request` template.

## Community Expectations

All contributors and maintainers are expected to uphold the [Code of Conduct](./CODE_OF_CONDUCT.md). Violations can be reported confidentially at [maintainers@kubeop.dev](mailto:maintainers@kubeop.dev).

We appreciate your contributions and look forward to building a reliable control plane together!

