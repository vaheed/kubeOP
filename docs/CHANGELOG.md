# Changelog

All notable changes to this project are documented here. The format follows
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/) and the project
adheres to [Semantic Versioning](https://semver.org/).

## [Unreleased]

### Changed
- Refined quickstart and attachment documentation (README, API reference, app guide, roadmap) into step-by-step flows for clearer execution.

## [0.2.0] - 2025-10-13

### Added
- ConfigMap and Secret attachment endpoints for apps, including selective key
  support with optional prefixes and detach helpers that clean env references.
- Unit tests covering attachment helpers and API routing plus documentation for
  new flows across README, API reference, and quickstarts.
- CI artifact upload of the compiled API binary for reference alongside lint,
  build, and test steps.

### Changed
- Bumped API specification and version metadata to v0.2.0 and expanded PR
  checklist expectations for new endpoints.

## [0.1.3] - 2025-10-12

### Changed
- Consolidated Kubernetes app status collection and deployment mutation helpers
  to remove duplicated controller-runtime calls and emit warn-level logging when
  reads fail.

### Added
- Tests covering `service.CollectAppStatus` to exercise pod, service, and
  ingress summarisation paths.
- Contributor pull request checklist guidance to make required updates explicit.

## [0.1.2] - 2024-??-??

### Added
- Initial public release of the control plane baseline.
