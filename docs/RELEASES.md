# Release & Versioning Policy

kubeOP follows Semantic Versioning (SemVer). The authoritative version metadata lives in
`internal/version/version.go`; releases are published when that file, `CHANGELOG.md`, and
the Docker/Operator artifacts in CI agree on the target tag.

## Version semantics

- **MAJOR** increments for backwards-incompatible API, schema, or behaviour changes.
  - Requires migration guidance in `docs/OPERATIONS.md` and highlighted breaking notes
    in `CHANGELOG.md`.
  - Update compatibility fields returned by `/v1/version` (see `internal/version`).
- **MINOR** increments for new functionality delivered under the existing API surface.
  - Update relevant docs (`README.md`, `docs/API.md`, `docs/QUICKSTART.md`).
  - Ensure new env vars or flags are documented in `docs/ENVIRONMENT.md`.
- **PATCH** increments for bug fixes or security updates without behavioural change.
  - Add regression tests and note the fix in `CHANGELOG.md`.

## Release checklist

1. Run the full CI suite locally:
   ```bash
   go test ./...
   go test -count=1 ./testcase
   npm run docs:lint
   npm run docs:build
   ```
2. Update `internal/version/version.go` with the new semantic version and timestamp.
3. Populate the `[Unreleased]` section of `CHANGELOG.md`, then split it into a dated
   entry for the new version.
4. Update roadmap statuses in `docs/ROADMAP.md` and link relevant GitHub issues.
5. Confirm documentation references (README badges, `docs/index.md`, `docs/RELEASES.md`)
   mention the new version.
6. Commit with a message like `chore: release v0.x.y`.
7. Tag the release locally (`git tag v0.x.y`) and push (`git push origin v0.x.y`).
8. Let GitHub Actions produce Docker images (`kubeop-api`, `kubeop-operator`) and upload
   build artifacts.
9. Draft GitHub release notes summarising highlights and linking to the CHANGELOG entry.

## Branch strategy

- `main` contains the latest development snapshot. All pull requests merge here.
- Release branches (e.g., `release/v0.12.x`) are created only when a patch train is
  needed after a minor release. They receive cherry-picked fixes and patch tags.
- Hotfixes should start from the latest tagged release, receive targeted changes, and
  bump the PATCH number.

## Backporting policy

- Security fixes affecting supported minors must be backported to the two previous
  minor versions (e.g., when releasing `0.12.3`, maintain `0.11.x` and `0.10.x`).
- Record backport status in the GitHub issue linked from the roadmap item.
- Update `docs/RELEASES.md` with any exceptions to the policy.

## Release communication

- Announce releases via CHANGELOG, GitHub Releases, and—if applicable—the documentation
  site homepage.
- Update `docs/ROADMAP.md` to reflect completed items and move follow-up work to the
  appropriate phase.
- Provide upgrade guidance (commands, rollback steps) in `docs/OPERATIONS.md` for any
  change touching migrations, secrets, or operator deployment.
