# Release and versioning policy

kubeOP follows Semantic Versioning and publishes Git tags plus container images
from the `main` branch. This document explains how release metadata maps to the
codebase and which quality gates run before cutting a tag.

## Version sources

- `internal/version/version.go` holds the build metadata surfaced by `/v1/version`
  and injected into logs on startup.
- `Makefile` and CI set `VERSION`, `COMMIT`, and `DATE` linker flags during
  builds, ensuring binaries and Docker images report consistent metadata.
- `CHANGELOG.md` tracks user-facing changes following
  [Keep a Changelog](https://keepachangelog.com/en/1.1.0/) conventions.

## Branching and tagging

1. All work lands on `main` via pull requests with passing CI
   (`go vet ./...`, `go test ./...`, `go test -count=1 ./testcase`, `npm run docs:lint`,
   `npm run docs:build`).
2. Release candidates are validated directly on `main`; no long-lived release
   branches exist today.
3. Create an annotated tag `vX.Y.Z` when the changelog and roadmap both reflect
   the shipped scope. CI automatically builds and publishes multi-arch images on
   tag pushes (see `.github/workflows/ci.yml`).

## Supported cadence

- **Patch releases**: as-needed for security fixes or regressions. Expect a
  turnaround measured in days once a bug is confirmed.
- **Minor releases**: roughly every 6–8 weeks, aligned with the roadmap phase
  boundaries documented in `docs/ROADMAP.md`.
- **Major releases**: only when a breaking API or schema change ships. These
  require migration guides and maintenance mode expectations.

## Pre-release checklist

Before tagging a release:

- [ ] Ensure all roadmap items targeted for the version are in `Done` state with
      merged pull requests.
- [ ] Update `CHANGELOG.md` `[Unreleased]` → `[X.Y.Z] - YYYY-MM-DD` and note any
      deprecations or breaking changes.
- [ ] Bump `internal/version/version.go` default version string.
- [ ] Run `go mod tidy` and confirm no diffs.
- [ ] Execute the full CI command suite locally or in a dry-run pipeline:
      `go vet ./...`, `go test ./...`, `go test -count=1 ./testcase`,
      `cd kubeop-operator && go test ./...`, `npm run docs:lint`, `npm run docs:build`.
- [ ] Verify Docker images build (`docker build -t kubeop-api:rc .`).
- [ ] Sanity-test `/v1/version`, `/healthz`, `/readyz`, and a sample app deploy
      using the Docker Compose stack (`docs/examples/docker-compose.yaml`).

## Post-release steps

- [ ] Push the `vX.Y.Z` tag and confirm GitHub Actions published images and
      artifacts.
- [ ] Create a GitHub release that links to the changelog section and roadmap
      items delivered.
- [ ] Update `docs/ROADMAP.md` to move completed items into "Done" or mark
      follow-up work.
- [ ] Notify operators via README badge/docs updates if compatibility notes
      changed.
- [ ] Backfill any documentation screenshots or examples that reference the new
      version.

## Deprecation policy

- Breaking API changes require at least one minor release of deprecation notices
  surfaced via `/v1/version` and the API documentation.
- Database migrations must be reversible; document rollback steps in
  `docs/OPERATIONS.md` when introducing non-trivial schema changes.
- Operator compatibility follows the same semantic version: the API and
  `kubeop-operator` with matching minor versions are expected to work together.
