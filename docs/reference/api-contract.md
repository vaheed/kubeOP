# API contract & release policy

kubeOP maintains a stable, well-documented contract for its public API so
operators and automation can upgrade predictably. This reference outlines the
versioning promises, deprecation workflow, and release cadence that apply to
the control-plane binaries, REST endpoints, and generated documentation.

## Versioning promises

- kubeOP follows [Semantic Versioning](https://semver.org/). Breaking changes
  to the REST API or CLI require a new **major** version.
- The primary REST surface is `/v1`. Minor and patch releases may add new
  fields, optional query parameters, or additional endpoints under `/v1` but
  will not remove existing fields or alter response shapes in a
  backward-incompatible way.
- When a future API version (for example `/v2`) is introduced, the prior major
  version remains available for at least two minor release trains. Metadata
  returned by `/v1/version` advertises the supported range via
  `minApiVersion`/`maxApiVersion`.
- Generated schemas in [`docs/openapi.yaml`](../openapi.yaml) reflect the
  current contract. Any change to handlers under `cmd/api` or `internal/api`
  must update the OpenAPI document in the same pull request.

## Deprecation workflow

1. Document the upcoming change in [`docs/changelog.md`](../changelog.md)
   under the **Changed** or **Removed** section for the `[Unreleased]`
   heading.
2. Update [`docs/reference/versioning.md`](./versioning.md) with the new
   compatibility metadata and, when applicable, set
   `rawDeprecationDeadline`/`rawDeprecationNote` in
   `internal/version/version.go` so `/v1/version` broadcasts the timeline.
3. Add migration or upgrade guidance to the relevant docs under `docs/`
   (tutorials, guides, or reference pages) and confirm examples in the
   `README.md` remain accurate.
4. Announce the change at least one minor release before removal. During this
   window, the API continues to accept deprecated fields but logs warnings
   through `internal/logging`.

## Release cadence & support window

- The team targets a **monthly minor release** that bundles new features and
  operational improvements. Patch releases ship as needed to address bugs or
  security fixes.
- kubeOP officially supports the **current minor release and the two most
  recent minors**. Automation should upgrade before falling outside this
  three-release window.
- Each release updates the Keep a Changelog entry in
  [`docs/changelog.md`](../changelog.md) and tags a SemVer in
  `internal/version/version.go`. The changelog must link to any new or
  modified documentation sections so operators can find migration guidance.

## Pull request expectations

Pull requests that affect behaviour, API handlers, or client-visible
interfaces must:

- Tick the API contract checkbox in `.github/pull_request_template.md` when the
  contract changes.
- Include corresponding tests under `testcase/` when code paths change.
- Provide logging and error-handling updates when introducing new flows.

These expectations keep the contract enforceable and ensure documentation stays
synchronised with released binaries.
