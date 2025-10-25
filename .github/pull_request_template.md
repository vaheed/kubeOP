## Summary

<!-- Briefly describe the changes introduced by this PR. -->

## Testing

- [ ] `go vet ./...`
- [ ] `go build ./...`
- [ ] `go test ./...`
- [ ] `go test -count=1 ./testcase`
- [ ] `cd kubeop-operator && go vet ./...`
- [ ] `cd kubeop-operator && go test ./...`
- [ ] `cd kubeop-operator && go test ./internal/webhooks -count=1`
- [ ] `cd kubeop-operator && go build ./cmd/manager`
- [ ] `cd kubeop-operator && make tools`
- [ ] `cd kubeop-operator && make crds` (no diff)
- [ ] `cd kubeop-operator && make validate`
- [ ] `cd kubeop-operator && go build ./cmd/bootstrap`
- [ ] Applied/Appended CRD updates when touching `kubeop-operator/config/crd` (`kubectl apply -f kubeop-operator/config/crd/bases/kubeop.io_apps.yaml`)
- [ ] `npm run docs:lint` (Vale)
- [ ] `npm run docs:build`
- [ ] `go mod tidy` (no changes)
- [ ] Additional commands (list):
- [ ] `codeql database analyze` (or equivalent CodeQL scan)
- [ ] Verified project/app logs under `${LOGS_ROOT}` (if applicable)

## Documentation & Release

- [ ] README updated (if behavior or usage changed)
- [ ] Relevant docs in `docs/` updated
- [ ] Mermaid diagrams refreshed (`docs/ARCHITECTURE.md`, `docs/_snippets/diagram-*.md`)
- [ ] Quickstart/Install guides reflect workflow changes (`docs/QUICKSTART.md`, `docs/INSTALL.md`)
- [ ] Configuration references updated when environment variables change (`docs/ENVIRONMENT.md`)
- [ ] API reference updated when handlers change (`docs/API.md`, `docs/openapi.yaml`)
- [ ] CLI/Operations/Security docs align with behaviour (`docs/CLI.md`, `docs/OPERATIONS.md`, `docs/SECURITY.md`)
- [ ] App delivery security doc updated when registry policies or release controls change (`docs/apps/security.md`)
- [ ] Troubleshooting/FAQ refreshed if new scenarios documented (`docs/TROUBLESHOOTING.md`, `docs/FAQ.md`)
- [ ] Roadmap adjusted when scope shifts (`docs/ROADMAP.md`)
- [ ] CRD reference updated (`docs/CRDs.md`) when schemas or behaviour change
- [ ] Snippets/examples regenerated when sample commands change (`docs/_snippets/`, `docs/examples/`)
- [ ] `CHANGELOG.md` updated under `[Unreleased]` when applicable
- [ ] Version bumped (SemVer) when applicable
- [ ] `CODEQL.md` updated when security mitigations change
- [ ] Version compatibility docs updated when metadata changes (`README.md`, `docs/API.md`, `/v1/version` examples)
- [ ] Database migrations tested/validated (no dirty state; document recovery steps if touched)
- [ ] Migration numbering remains contiguous (`TestMigrationVersionsAreSequential`)
- [ ] Roadmap updated if new work identified or decisions made (`docs/ROADMAP.md`)
- [ ] `AGENTS.md` reviewed/updated if workflow expectations changed

## Checklist

- [ ] Lint, tests, and build succeed locally
- [ ] Minimal logging/error handling added where needed
- [ ] Startup/logging changes documented when behaviour shifts
- [ ] External HTTP requests validate user-controlled input (request forgery mitigations reviewed)
- [ ] No secrets committed; runtime secrets pulled from GitHub Actions or env vars
- [ ] PR linked to related issues/discussions when available
- [ ] Added/updated tests cover new behaviour and guard against regressions
