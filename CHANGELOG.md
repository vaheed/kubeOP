# Changelog

All notable changes to this project are documented here. The format follows the guidance at <https://keepachangelog.com/en/1.1.0/>,
and the project adheres to Semantic Versioning (<https://semver.org/>).

## [Unreleased]

### Added

- Nothing yet.

## [0.18.1] - 2026-02-16

### Fixed

- The tenant controller now watches namespace label changes with controller-runtime's typed builder,
  ensuring RoleBindings follow newly created namespaces and allowing `go vet` to pass without build
  errors in CI.

### Changed

- Documented the namespace watch behaviour in the README and tenancy security guide so platform
  teams know RoleBindings refresh automatically when namespaces are labelled.

## [0.18.0] - 2026-02-14

### Added

- Added tenant admission webhooks that enforce mandatory `paas.kubeop.io/{tenant,project,app}` labels, validate cross-object
  references, and reject privileged Jobs without the `paas.kubeop.io/run-as-root-justification` annotation. The behaviour is
  documented in the new [docs/security/tenancy.md](docs/security/tenancy.md) guide along with actionable `kubectl` snippets.
- Introduced unit tests for the App and Job webhooks to keep tenant isolation guardrails from regressing, plus envtest coverage
  to exercise the admission stack end-to-end.
- Enforced service exposure policies via `NetworkPolicyProfile` service policy definitions, delivered tenant-scoped RBAC
  automation with the new `TenantReconciler`, and shipped default `tenant-{owner,developer,viewer}` ClusterRoles so tenant
  kubeconfigs stay confined to labelled namespaces.

### Changed

- Added a `fmt` Makefile target so contributors can run `gofmt` across tracked Go sources with a single command, matching the
  CI formatting checks.

### Security

- Hardened Helm chart downloads with HTTPS-only requests, allow-listed hosts, redirect validation, private network blocking,
  and response size limits, resolving CodeQL SSRF alerts.
- Introduced shared `pkg/security` helpers and rewired Git delivery to normalise paths, evaluate symlinks, and prevent
  repository escapes, closing CodeQL path traversal findings.

## [0.16.2] - 2026-01-24

### Fixed

- `make -C kubeop-operator validate` now installs `kubeconform`, checks for `kubectl`, and logs each phase so validation fails
  fast with actionable feedback instead of cryptic missing-binary errors.
- The default overlay references the base kustomization file directly and the stray placeholder CRD stub was removed, fixing
  `kubectl kustomize` failures during manifest validation.

### Changed

- Bumped the platform version to `v0.16.2` and documented the new `make -C kubeop-operator tools` helper in the README so
  contributors install validation prerequisites explicitly.

## [0.16.1] - 2026-01-20

### Fixed

- Restored the bootstrap CLI compilation by wiring it to the embedded CRD installer, correcting default project environment
  handling, and ensuring the `init` command can run without explicit environment flags.

### Changed

- Documented the operator Makefile targets (`make crds`, `make validate`, `make install`, `make uninstall`) and clarified CLI
  defaults for platform teams provisioning tenants and projects.

## [0.16.0] - 2025-12-10

### Added

- Ported the entire kubeOP platform API into `kubeop-operator/apis/paas/v1alpha1` with validation markers, printer columns, and
  status subresources. Generated CRDs now live under `kubeop-operator/kustomize/bases/crds/` and ship with samples for every
  resource.
- Introduced the `kubeop-bootstrap` CLI (see `kubeop-operator/cmd/bootstrap`) which installs CRDs/RBAC/webhooks, seeds default
  network/runtime/billing profiles, and streamlines tenant/project/domain/registry provisioning with CloudEvents audit trails.
- Documented all platform custom resources in `docs/CRDs.md` and expanded the README/CLI docs with bootstrap instructions.

### Changed

- Expanded the kubeop-operator Makefile with `make crds`, `make validate`, `make install`, and `make uninstall` targets that
  wire in `controller-gen`, `kubectl kustomize`, and `kubeconform` checks.
- Updated CI to build and validate the new operator assets (`make crds`, `make validate`, and the bootstrap binary) so PRs gate
  on schema drift and manifest hygiene.
- Bumped the project version to `v0.16.0` to capture the API surface and CLI additions.

## [0.15.4] - 2025-10-24

### Fixed

- The kubeop-operator now installs or updates the bundled App CRD before starting, eliminating startup crashes caused by missing
  CRDs in freshly provisioned clusters.

## [0.15.3] - 2025-10-24

### Fixed

- Bundled the App CRD manifest with the operator, added regression tests, and updated installation docs so the controller no
  longer fails to start when the CRD is missing from managed clusters.

## [0.15.2] - 2025-10-23

### Changed

- Bumped the build metadata to `v0.15.2` to capture the latest runtime fixes.

### Fixed

- Development builds now fall back to the `0.0.0-dev` placeholder instead of panicking when non-semantic version strings are
  provided, logging the correction for operators.

## [0.15.0] - 2025-11-07

### Added

- Documentation updates describing the event bridge toggle as an explicit opt-in and aligning release metadata with the 0.15.0
  baseline.

### Changed

- Bumped the build metadata to `v0.15.0` and updated the README plus environment reference for the fresh baseline release.
- Simplified configuration loading by removing the deprecated `K8S_EVENTS_BRIDGE` environment variable alias; the bridge now
  honours `EVENT_BRIDGE_ENABLED` only.

### Removed

- Legacy `K8S_EVENTS_BRIDGE` configuration alias and associated documentation references.

### Security

- Reinforced that `/v1/events/ingest` remains disabled unless the operator explicitly sets `EVENT_BRIDGE_ENABLED=true`.

## [0.14.0] - 2025-05-26

### Changed

- Replaced the legacy `/v1/version` compatibility payload with immutable build metadata and removed deprecation warnings from the API and CLI.
- Standardised on the `kubeop.app.id` label across all selectors, services, and log collectors; removed the legacy `kubeop.app-id` alias.
- Simplified the service layer by deleting unused hard-delete project code paths and deprecated documentation.
- Rebuilt README and the core documentation set (Quickstart, Install, Environment, Architecture, API, Roadmap, Style guide) to match the cleaned architecture.
- Updated GitHub Support guidance to point at the new documentation set and trimmed unused snippets/examples.

### Removed

- Deprecated documentation (CLI, Security, Operations, FAQ, Troubleshooting, Glossary, IA sitemap, release policy, ADR log) and associated snippets/examples that no longer reflect the architecture.
- Compatibility/deprecation metadata from `/v1/version` responses and associated tests.

## [0.11.3] - 2025-12-07

### Security

- Helm chart downloads now require canonical paths, rejecting relative or
  backslash segments before issuing network requests to close the remaining
  CodeQL SSRF finding.

## [0.11.2] - 2025-11-06

### Security

- Sanitised Helm chart download requests now re-use the validated URL object,
  closing a CodeQL-reported SSRF vector.
- Git manifest resolution re-validates checkout paths before filesystem access
  and re-checks each manifest read, preventing repository escapes flagged by
  CodeQL.

## [0.9.1] - 2025-11-04

### Fixed

- Hardened Git delivery path validation to reject absolute and parent-relative
  inputs flagged by CodeQL, preventing repository escapes during manifest
  rendering.

### Removed

- _None yet_

## [0.9.0] - 2025-11-03

### Added

- Delivery metadata and SBOM persistence, including `/v1/projects/{id}/apps/{appId}/delivery`, schema migrations, and release store updates.
- Git checkout helpers in `internal/delivery` with signature verification utilities and manifest digest builders.
- Service and API layers for retrieving delivery info, plus tests across stores, services, and API routing.
- Template/app binding repository for associating rendered apps with their source templates.

### Changed

- Validation responses now surface SBOM fingerprints so automation can gate rollouts on manifest digests.
- README and docs highlight the delivery inspection flow and link to new walkthroughs under `docs/apps/`.

### Fixed

- Ensured release ingestion retains SBOM payloads and delivery metadata survives repeated fetches.

## [0.8.30] - 2025-11-02

### Added

- Event bridge ingest endpoint `/v1/events/ingest` (behind `K8S_EVENTS_BRIDGE`/`EVENT_BRIDGE_ENABLED`) with batch summaries, service and API tests, and documentation covering configuration and usage.
- Controller-runtime based `kubeop-operator` module scaffold with App CRD types, reconciler stub, standalone tests, documentation, and CI integration (Phase 0 roadmap deliverable).
- Job and CronJob manifest samples under `samples/jobs/` plus documentation in `docs/samples/02-jobs.md` for manual batch workload experiments.
- `samples/Makefile` targets and a bootstrap `.env.example` to drive the roadmap samples from the repository root with logged steps and guardrails.

### Changed

- Centralised the samples documentation under `docs/samples/` and replaced in-repo
  Markdown in the automation directories with plain-text pointers to comply with
  repository guidelines.
- `kubeop-operator` now marks `App` resources as `Ready` and records the observed
  generation after each reconcile so operators can verify controller health from
  status fields.

### Fixed

- Switched the App status helper to `api/meta.SetStatusCondition` so controller builds
  and `go vet` checks succeed with supported Kubernetes API machinery versions.

### Removed

- Retired the standalone `repo-sanity` workflow and Python helper in favour of
  consolidated hygiene checks inside the main CI pipeline.

## [0.8.29] - 2025-11-01

### Fixed

- Prevented Helm chart downloads from reusing unsanitised URLs and blocked Git
  manifest paths and symlinks from escaping cloned repositories, closing the
  CodeQL-reported SSRF and path traversal issues.

## [0.8.25] - 2025-10-31

### Added

- Scaffolded the `samples/` automation suite with shared logging helpers, a bootstrap example,
  and documentation updates covering environment variables and tutorials.

## [0.8.24] - 2025-10-30

### Added

- Global maintenance mode toggle with `/v1/admin/maintenance` endpoints, database persistence, and service guards that block
  mutating app/project/cluster operations while upgrades are in progress.
- OCI manifest bundle deployments (`ociBundle` sources) with registry credential reuse, digest tracking in releases, validation support, and a dedicated tutorial.

### Changed

- Completed Git delivery documentation by wiring OpenAPI schemas for `git` sources across validation, deploy, and release payloads and aligned handler/service coverage.

### Fixed

- Hardened Helm chart downloads to ensure HTTP requests use validated targets only and avoid request forgery vectors during chart fetches.
- Replaced the dead ORAS CLI documentation link in the OCI bundle tutorial to keep markdown link checks passing.

## [0.8.22] - 2025-10-30

### Added

- Git-backed application deployments with support for raw manifest folders and Kustomize overlays, including commit tracking in validation responses and release history.
- Tutorial, API reference, and README examples covering Git workflows alongside optional `ALLOW_GIT_FILE_PROTOCOL` for local testing.

### Changed

- Environment reference and `.env.example` now document the Git file protocol toggle and highlight credential reuse for repository access.
- Quickstart and architecture guides explain Git delivery flows and updated service responsibilities.

## [0.8.20] - 2025-10-29

### Added

- Structured version metadata now exposes compatibility ranges (`minClientVersion`, API min/max) and optional deprecation deadlines via `/v1/version`, logging warnings when running deprecated builds. Documentation, tutorials, and README sections cover upgrade guidance and compatibility checks.

## [0.8.19] - 2025-10-28

### Added

- Helm OCI chart deployments via the `helm.oci` payload, including registry credential integration, dry-run support, and documentation/tutorial updates covering validation and rollout flows.

## [0.8.18] - 2025-10-27

### Added

- Cluster inventory metadata and status history: `/v1/clusters/{id}`, `/v1/clusters/{id}/status`, and enhanced `/v1/clusters` responses now expose ownership, environment, and persisted health snapshots alongside tutorials and API docs.
- Environment defaults `CLUSTER_DEFAULT_ENVIRONMENT` and `CLUSTER_DEFAULT_REGION` to seed metadata when registration payloads omit values.

### Changed

- Cluster health responses include probe messages, API server versions, and structured `details`, and the scheduler persists each check for auditing.

## [0.8.17] - 2025-10-26

### Fixed

- Tightened release pagination to require matching project IDs and clarified
  cursor usage across API docs, guides, and tutorials.
