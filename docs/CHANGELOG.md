# Changelog

All notable changes to this project are documented here. The format follows
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/) and the project
adheres to [Semantic Versioning](https://semver.org/).

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
