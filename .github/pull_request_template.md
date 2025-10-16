## Summary

<!-- Briefly describe the changes introduced by this PR. -->

## Testing

- [ ] `go vet ./...`
- [ ] `go build ./...`
- [ ] `go test ./...`
- [ ] `go test -count=1 ./testcase`
- [ ] `go mod tidy` (no changes)
- [ ] Additional commands (list):
- [ ] Verified project/app logs under `${LOGS_ROOT}` (if applicable)

## Documentation & Release

- [ ] README updated (if behavior or usage changed)
- [ ] Relevant docs in `docs/` updated
- [ ] Architecture pages (`docs/architecture.md`, diagrams) reflect new flows
- [ ] Guides updated when workflows change (`docs/guides/tenants-projects-apps.md`, kubeconfig, deployments, quotas, watcher)
- [ ] Configuration or operations changes documented (`docs/configuration.md`, `docs/operations.md`)
- [ ] Domain/DNS automation updates captured (`README.md`, `docs/configuration.md`, `docs/api/projects.md`)
- [ ] API reference updated when handlers change (`docs/api/*` and `docs/openapi.yaml`)
- [ ] `docs/changelog.md` updated under `[Unreleased]` when applicable
- [ ] Version bumped (SemVer) when applicable
- [ ] Watcher deployment changes validated against PodSecurity `restricted`
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
