# Changelog

All notable changes to this project are documented here. The format follows the guidance at <https://keepachangelog.com/en/1.1.0/>,
and the project adheres to Semantic Versioning (<https://semver.org/>).

## [Unreleased]

### Added

- Information architecture audit (`docs/IA.md`) documenting legacy vs target structure.
- VitePress homepage, CLI, Operations, Security, Troubleshooting, FAQ, Glossary, and refreshed examples/snippets.

### Changed

- Rewrote README, Quickstart, Install, Environment, Architecture, API, Roadmap, and Style Guide for the current control plane.
- Replaced generated PNG/SVG diagrams with reusable Mermaid snippets and removed the export tooling.
- Switched documentation linting to Vale-only, removed Markdownlint configuration, and refreshed CI plus PR checklist guidance.

### Fixed

- Set the VitePress base path to `/kubeOP/` so the GitHub Pages deployment renders correctly.

### Security

- Enforced an explicit Helm chart host allow-list (`HELM_CHART_ALLOWED_HOSTS`) and tightened Git manifest path sanitisation to
  close CodeQL-reported SSRF and path traversal findings.

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
