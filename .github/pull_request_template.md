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
- [ ] Kubeconfig lifecycle docs (`docs/KUBECONFIG.md`, kubeconfig API references) updated when behavior changes
- [ ] Grafana/alert assets updated (`docs/dashboards`, alert rules) when metrics or SLOs change
- [ ] Logging path constraints (`[A-Za-z0-9._-]` IDs, LOGS_ROOT usage) documented when touching disk logging
- [ ] `docs/CHANGELOG.md` updated under `[Unreleased]` when applicable
- [ ] Version bumped (SemVer) when applicable
- [ ] Database migrations tested/validated (no dirty state; document recovery steps if touched)
- [ ] Migration numbering remains contiguous (`TestMigrationVersionsAreSequential`)
- [ ] Documentation plan (`docs/DOCUMENTATION_PLAN.md`) reviewed/updated when scope shifts
- [ ] Roadmap (`docs/ROADMAP.md`) updated if new follow-up work was identified
- [ ] `AGENTS.md` reviewed/updated if workflow expectations changed

## Checklist

- [ ] Lint, tests, and build succeed locally
- [ ] Minimal logging/error handling added where needed
- [ ] Startup/logging changes documented when behaviour shifts
- [ ] External HTTP requests validate user-controlled input (request forgery mitigations reviewed)
- [ ] No secrets committed; runtime secrets pulled from GitHub Actions or env vars
- [ ] PR linked to related issues/discussions when available
- [ ] Added/updated tests cover new behaviour and guard against regressions

