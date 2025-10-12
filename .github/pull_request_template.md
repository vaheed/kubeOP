## Summary

<!-- Briefly describe the changes introduced by this PR. -->

## Testing

- [ ] `go vet ./...`
- [ ] `go test ./...`
- [ ] `go test -count=1 ./testcase`
- [ ] Additional commands (list):

## Documentation & Release

- [ ] README updated (if behavior or usage changed)
- [ ] Relevant docs in `docs/` updated
- [ ] `docs/CHANGELOG.md` updated under `[Unreleased]` when applicable
- [ ] Version bumped (SemVer) when applicable
- [ ] Documentation plan (`docs/DOCUMENTATION_PLAN.md`) reviewed/updated when scope shifts
- [ ] Roadmap (`docs/ROADMAP.md`) updated if new follow-up work was identified

## Checklist

- [ ] Lint, tests, and build succeed locally
- [ ] Minimal logging/error handling added where needed
- [ ] External HTTP requests validate user-controlled input (request forgery mitigations reviewed)
- [ ] No secrets committed; runtime secrets pulled from GitHub Actions or env vars
- [ ] PR linked to related issues/discussions when available
- [ ] Added/updated tests cover new behaviour and guard against regressions

