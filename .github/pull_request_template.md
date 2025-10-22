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
- [ ] Architecture pages (`docs/architecture.md`, diagrams) reflect new flows
- [ ] Guides updated when workflows change (`docs/guides/tenants-projects-apps.md`, kubeconfig, deployments, quotas)
- [ ] Delivery guides updated when behaviour changes (`docs/apps/*`)
- [ ] Tutorials updated when new end-to-end workflows are added (`docs/TUTORIALS/*`)
- [ ] Samples library scripts/docs updated when `samples/` changes
- [ ] Cluster inventory docs updated when metadata or health endpoints change (`docs/api/clusters.md`, `docs/TUTORIALS/cluster-inventory-service.md`)
- [ ] Configuration or operations changes documented (`docs/configuration.md`, `docs/operations.md`)
- [ ] Operator automation docs updated when rollout behaviour changes (`docs/CRD-GUIDE.md`, `docs/operations.md`, `docs/configuration.md`)
- [ ] Domain/DNS automation updates captured (`README.md`, `docs/configuration.md`, `docs/api/projects.md`)
- [ ] API reference updated when handlers change (`docs/api/*` and `docs/openapi.yaml`)
- [ ] API contract docs updated when versioning or deprecation policies change (`docs/reference/api-contract.md`)
- [ ] `docs/changelog.md` updated under `[Unreleased]` when applicable
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
