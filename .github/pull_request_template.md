## Summary

<!-- Briefly describe the changes introduced by this PR. -->

## Testing

- [ ] `go vet ./...`
- [ ] `go build ./...`
- [ ] `go test ./...`
- [ ] `go test -count=1 ./testcase`
- [ ] `cd kubeop-operator && go vet ./...`
- [ ] `cd kubeop-operator && go test ./...`
- [ ] `cd kubeop-operator && go build ./cmd/manager`
- [ ] `go mod tidy` (no changes)
- [ ] Additional commands (list):
- [ ] Verified project/app logs under `${LOGS_ROOT}` (if applicable)

## Documentation & Release

- [ ] README updated (if behavior or usage changed)
- [ ] Relevant docs in `docs/` updated
- [ ] Architecture diagrams refreshed (`docs/ARCHITECTURE.md`, `docs/media/*.mmd`/`.svg`)
- [ ] Quickstart/Install guides reflect workflow changes (`docs/QUICKSTART.md`, `docs/INSTALL.md`)
- [ ] Configuration references updated when environment variables change (`docs/ENVIRONMENT.md`)
- [ ] API reference updated when handlers change (`docs/API.md`, `docs/openapi.yaml`)
- [ ] CLI/Operations/Security docs align with behaviour (`docs/CLI.md`, `docs/OPERATIONS.md`, `docs/SECURITY.md`)
- [ ] Troubleshooting/FAQ refreshed if new scenarios documented (`docs/TROUBLESHOOTING.md`, `docs/FAQ.md`)
- [ ] Roadmap adjusted when scope shifts (`docs/ROADMAP.md`)
- [ ] Snippets/examples regenerated when sample commands change (`docs/_snippets/`, `docs/examples/`)
- [ ] `CHANGELOG.md` updated under `[Unreleased]` when applicable
- [ ] Version bumped (SemVer) when applicable
- [ ] Version compatibility docs updated when metadata changes (`docs/reference/versioning.md`, `/v1/version` examples)
- [ ] Database migrations tested/validated (no dirty state; document recovery steps if touched)
- [ ] Migration numbering remains contiguous (`TestMigrationVersionsAreSequential`)
- [ ] Roadmap / ADRs updated if new work identified or decisions made (`docs/adr.md`, `docs/ROADMAP.md`)
- [ ] `AGENTS.md` reviewed/updated if workflow expectations changed

## Checklist

- [ ] Lint, tests, and build succeed locally
- [ ] Minimal logging/error handling added where needed
- [ ] Startup/logging changes documented when behaviour shifts
- [ ] External HTTP requests validate user-controlled input (request forgery mitigations reviewed)
- [ ] No secrets committed; runtime secrets pulled from GitHub Actions or env vars
- [ ] PR linked to related issues/discussions when available
- [ ] Added/updated tests cover new behaviour and guard against regressions
