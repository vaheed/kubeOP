# Internal Worklog

## 2025-10-29 — SemVer metadata + compatibility matrix

**Problem**
- Stream A (Platform Hardening & Governance) requires richer version metadata so operators and API clients can detect supported ranges and upcoming deprecations, but `internal/version/version.go` only exports static build strings today.

**Approach**
- Replace the raw string globals with a struct that captures the current version, supported minimum client version, supported API range, and optional deprecation notices sourced from build flags.
- Add helper functions to parse and validate SemVer ranges using `golang.org/x/mod/semver`, emit compatibility summaries through the `/version` endpoint, and log warnings when running deprecated builds.
- Update configuration docs, changelog, and README versioning guidance to explain the new metadata along with tests covering parsing, validation, and deprecated flag handling.

**Acceptance Criteria**
- Version package exposes structured metadata with parsing/validation that rejects invalid SemVer inputs and supports compatibility checks.
- `/version` endpoint (and CLI output) returns the enriched metadata and logs a warning when a deprecation deadline is set in the past.
- README, changelog, and docs/reference mention the compatibility matrix, and tests under `testcase/` cover range validation plus HTTP handler responses.

**Outcome**
- Replaced the static version globals with structured metadata that validates SemVer ranges, exposes compatibility checks, and parses optional deprecation deadlines.
- `/v1/version` now returns the compatibility matrix, logs deprecation warnings, and the README, API docs, OpenAPI spec, tutorial, and reference content describe the workflow alongside a 0.8.20 changelog entry.
- Follow-ups: automate release tooling to set `rawDeprecationDeadline`/`rawDeprecationNote` when publishing tags so operators receive advance warnings.

## 2025-10-28 — Helm OCI chart delivery

**Problem**
- Roadmap Epoch I calls for Helm OCI chart support so operators can source applications from registries, but kubeOP only accepts HTTP(S) `.tgz` charts today.

**Approach**
- Extend the Helm rendering helpers to pull charts over the `oci://` protocol using Helm's registry client with optional authentication via stored credentials.
- Thread OCI-specific options through the service deployment planner and API payload validation while keeping existing HTTP chart behaviour intact.
- Update configuration, docs, and samples with end-to-end guidance and regression tests.

**Acceptance Criteria**
- App deployments accept a `helm.oci` payload referencing `oci://` registries and render charts successfully, including validation dry-runs.
- Tests cover OCI URL validation, registry pulls, and service planner behaviour with and without credentials.
- README, changelog, API/environment docs, tutorials, and roadmap updates describe the new OCI workflow and usage constraints.

**Outcome**
- Added Helm OCI rendering helpers with registry login support, updated service validation, and exercised new flows through targeted service and renderer tests covering host resolution, insecure registries, and credential scope checks.
- Refreshed README, API/environment docs, OpenAPI schema, roadmap, changelog, and a new tutorial to document the OCI workflow; version bumped to 0.8.19 and smoke-tested with `go test`/`go build` using `.env.example` defaults.
- Follow-ups: monitor registry credential edge cases for project/user scope overlaps once multi-tenant staging coverage expands.

## 2025-10-24 — App validation endpoint

**Problem**
- Operators lack a dry-run path to verify app specs before deployment, leading to failed deployments and manual cleanup.

**Approach**
- Add a service-level validator that reuses deployment input checks without mutating Kubernetes or the database.
- Expose `/v1/apps/validate` in the API to accept app specs and return validation results plus rendered manifest metadata.
- Capture warnings/errors in structured responses and document CLI examples and tutorials.

**Acceptance Criteria**
- Validation endpoint returns success with computed replicas/resources or descriptive errors for invalid specs.
- Tests cover service validation logic and HTTP handler behaviour.
- Docs, OpenAPI, README, and tutorial updated with validation workflow and curl examples.

**Outcome**
- Added `/v1/apps/validate` service method, API handler, tests, and OpenAPI documentation.
- Documented the dry-run workflow across README, guides, tutorials, and changelog; roadmap item marked as complete.
- No follow-ups pending.

## 2025-10-24 — Credential stores for Git/registry secrets

**Problem**
- Roadmap Epoch I requires secure storage of Git and container registry credentials so delivery engines can fetch sources without embedding secrets in app specs.

**Approach**
- Introduce encrypted `git_credentials` and `registry_credentials` tables with CRUD services tied to tenant or project ownership.
- Add service and API layers to manage credentials, reusing AES-GCM encryption derived from `KCFG_ENCRYPTION_KEY` and enforcing audit logging.
- Provide validation, docs, samples, and configuration updates covering new environment toggles and usage guides.

**Acceptance Criteria**
- Credentials can be created, listed, retrieved, and deleted via API with encryption at rest and scope validation (user/project).
- Tests cover store, service, and handler logic including encryption and authorization scenarios.
- Documentation (README, ENVIRONMENT, API reference, tutorials) reflects credential workflows, and roadmap item is marked done with PR link post-merge.

## 2025-10-25 — App templates catalog and instantiation

**Problem**
- Operators lack a reusable catalog for curated application blueprints, so deployments repeat boilerplate specs and risk configuration drift.

**Approach**
- Extend the templates store with metadata, JSON Schema validation, and rendering helpers to merge defaults with user input.
- Add API endpoints for listing, retrieving, and instantiating templates into deployable payloads with audit-friendly logging.
- Document the workflow end-to-end with CLI examples, environment updates, and a runnable tutorial for rapid validation.

**Acceptance Criteria**
- Template payloads validate against stored JSON Schema and produce merged delivery specs ready for `/v1/projects/{id}/apps`.
- API exposes list/detail/render endpoints with structured error handling and coverage from service and handler tests.
- README, docs, changelog, tutorial, and examples explain template usage, and roadmap item is marked done post-merge with version bump.

**Outcome**
- Finalized the JSON Schema-backed template catalog with deployment hook plumbing and rendering safeguards for empty payloads.
- Added store and service regression tests for validation failures, template execution errors, missing catalog entries, and deploy hook propagation.
- Updated docs, OpenAPI specs, tutorial, and README to reflect the template workflow; lint, build, tests, and docs build now pass locally.
- Follow-ups: None.

## 2025-10-26 — Release history and audit trail

**Problem**
- Deployments lack an immutable history, leaving operators without spec digests or rendered manifest metadata required for audits and rollback analysis.

**Approach**
- Add a `releases` table and store helpers to persist per-deployment digests, summaries, and warnings keyed by app/project.
- Extend the service deploy flow to record release snapshots after successful applies and expose a paginated API endpoint for retrieval.
- Document the workflow across README, API reference, guides, and a new tutorial while updating OpenAPI and changelog entries.

**Acceptance Criteria**
- Releases persist with source type, spec/rendered digests, rendered object summaries, load balancer usage, and warnings.
- `/v1/projects/{id}/apps/{appId}/releases` returns recent releases with deterministic ordering and curl examples in docs.
- Tests cover store/service/API paths; docs, changelog, version, and roadmap updates accompany the code.

**Outcome**
- Implemented release persistence with migrations, store helpers, service pagination, and API handler plus regression tests.
- Updated README, guides, API reference, OpenAPI spec, changelog, and new tutorial to document auditing workflows.
- Bumped version to 0.8.16, refreshed CI expectations locally (fmt/vet/test/docs build), and marked the roadmap item complete.

## 2025-10-27 — Cluster inventory service foundation

**Problem**
- Roadmap Stream B calls for a multi-cluster registry with ownership, environment, and health insights, but clusters currently only store a name and kubeconfig with no metadata or persisted health snapshots.

**Approach**
- Extend the clusters schema to capture owner/team, environment, region, tags, API endpoint, and last-seen timestamps plus a dedicated `cluster_status` table for persisted health summaries.
- Add service methods and API endpoints to create, update, and fetch cluster metadata and status, emitting structured logs and integrating with the existing health scheduler.
- Provide CLI/documentation samples that show registering clusters with metadata, listing them, and reading health summaries while updating environment configuration and tests.

**Acceptance Criteria**
- Database migrations add metadata columns and status table with rollback scripts, and store/service layers persist and retrieve full records with validation.
- `/v1/clusters` supports metadata fields on create/update plus new `/v1/clusters/{id}` and `/v1/clusters/{id}/status` endpoints, all covered by tests and OpenAPI documentation.
- README, docs, changelog, environment config, tutorials, and roadmap entry document the new registry workflow with runnable examples and updated versioning.

**Outcome**
- Added cluster metadata columns, status history table, store helpers, service methods, and API handlers with tests covering registration defaults, status persistence, and route wiring.
- Updated README, getting started, environment docs, API reference, OpenAPI schema, tutorials, roadmap, and changelog; version bumped to 0.8.18.
- Follow-ups: consider exposing node counts in status history once collectors are available.

## 2025-10-30 — Git delivery support (plan)

### Goal
- Implement roadmap Epoch I delivery item for Git-backed app deployments so tenants can fetch manifests or Kustomize overlays directly from repositories.

### Scope
- Extend app deployment API/service to accept a `git` source with repository URL, ref, path, mode (`manifests` or `kustomize`), and optional credential reference.
- Fetch and render Git content during validation/deploy flows, including commit detection and manifest rendering.
- Persist source metadata, record release digests, and expose outputs consistent with existing image/Helm flows.
- Update docs (README, API reference, ENVIRONMENT, tutorials, architecture) and changelog; add tests covering Git modes and credential scoping.

### Acceptance Criteria
- `/v1/projects/{id}/apps` accepts Git payloads, validating inputs and rendering manifests or Kustomize builds.
- Validation endpoint mirrors deploy logic, returning rendered object summaries and commit metadata.
- Releases persist Git source info with deterministic manifest hashing; unit/integration tests cover Git fetch success/failure paths.
- Documentation and tutorials describe Git deployment workflows, including credential usage and curl examples.

### Risks
- Git operations add latency and require careful credential handling (SSH/token); need strict URL validation and cleanup of temp directories.
- Kustomize rendering introduces additional dependencies and potential incompatibilities with complex overlays.
- Increased test surface may slow CI; ensure fixture repos remain lightweight.

## 2025-10-31 — Maintenance mode toggles (plan)

### Goal
- Deliver the Stream A "Admin Ops Toolbox" roadmap capability for global maintenance mode toggles so operators can safely pause mutating API flows during upgrades.

### Scope
- Persist global maintenance state (enabled flag, message, actor, timestamps) with migrations and store/service helpers.
- Expose authenticated API endpoints to read and update maintenance state and document operational usage.
- Enforce maintenance blocks across mutating service operations (project/app deploy, scaling, image updates, quotas, cluster registration) with structured errors and logging.

### Acceptance Criteria
- New maintenance state table with migrations and rollback, store/service APIs, and coverage tests.
- `/v1/admin/maintenance` GET/PUT endpoints return current state and allow toggling with validation; API docs and README updated with curl examples and tutorial.
- Mutating operations return a 503-style maintenance error when enabled, with tests ensuring enforcement and allowing maintenance disable actions.

### Risks
- Broad enforcement may inadvertently block background tasks; need scoped guard checks to avoid breaking health probes or log/event ingestion.
- Potential race conditions if multiple admins toggle simultaneously; ensure updates are transactional and last write wins with clear logging.

## 2025-10-31 — Maintenance mode toggles (outcome)

**Problem**
- Operators needed a first-class way to pause mutating kubeOP APIs during upgrades; the roadmap called for maintenance toggles but none existed in the control plane.

**Approach**
- Added a `maintenance_state` table with store helpers, wired service-level guards that short-circuit mutating operations when enabled, and exposed `/v1/admin/maintenance` GET/PUT endpoints with structured logging.
- Documented the workflow across README highlights, API reference, and a dedicated tutorial while expanding tests (store, service, API) to cover toggling and enforcement.

**Outcome**
- Maintenance toggles persist actor/message metadata, block deploy/scale/delete operations with HTTP 503 responses, and surface the latest state via API and logs.
- Docs now describe enabling/disabling maintenance mode with curl examples, and roadmap Stream A marks maintenance toggles complete with the new tutorial link.
- Follow-ups: extend the guard list to include credential CRUD if future governance requires and explore pre-scheduled maintenance windows.

### Out of Scope
- Webhook-driven automatic redeployments beyond existing annotation patching.
- OCI bundle or GitOps controller integration; only direct Git fetch/render via API.
- UI changes or operator automation outside REST API and docs.


## 2025-10-30 — Git delivery support (done)

**What changed**
- Added Git-backed deployment support to `internal/service/apps.go` with commit tracking, manifest summaries, and release recording updates plus a dedicated `git_source` helper for cloning, authentication, and Kustomize rendering.
- Extended API request structs, validation responses, and release serialization to surface Git metadata; wired new unit tests in `testcase/service_app_validation_test.go` covering manifests and Kustomize modes.
- Documented the workflow across README, API reference, architecture guide, and a new tutorial while introducing the `ALLOW_GIT_FILE_PROTOCOL` environment toggle and bumping the version to `0.8.22`.

**Follow-ups**
- Evaluate Git commit signature verification and shallow-clone caching once repository scale grows.
- Add OCI bundle coverage to round out the remaining delivery type in the roadmap bullet.

## 2025-10-31 — Git delivery OpenAPI & validation coverage

**Goal**
- Finish the Git-backed application delivery roadmap item by completing the OpenAPI contract, tests, and documentation polish so the feature is fully consumable by clients.

**Scope**
- Update `docs/openapi.yaml` schemas for Git sources across validation, deployment, and release resources.
- Add handler/service integration tests covering Git payloads for validation and deployment flows.
- Refresh documentation or examples that reference the updated API, ensuring consistency.
- Confirm configuration toggles (e.g., `ALLOW_GIT_FILE_PROTOCOL`) behave as documented and remain disabled by default.

**Acceptance Criteria**
- OpenAPI definitions expose `git` source objects for apps, releases, and deploy requests with accurate enums.
- Service and API tests cover Git manifest and Kustomize payload handling without regressions to existing delivery types.
- Docs and changelog remain accurate with no dangling TODOs; version metadata stays at 0.8.22.
- CI suite (fmt, vet, tests, build) passes locally and via GitHub Actions with updated coverage.

**Risks**
- Schema mismatches between OpenAPI and request validators could break clients; must align structs and tests.
- Additional git/kustomize fixtures may increase test runtime; use lightweight repositories and mocks.

**Out of Scope**
- Expanding deployment engine to additional Git protocols beyond those already supported.
- Introducing new configuration flags or altering release persistence semantics beyond Git metadata exposure.

**Outcome**
- Documented Git payloads across OpenAPI and tests, added API/service coverage for Git validation and deploy flows, and ensured module dependencies include go-git/kustomize without bumping the runtime version.
- Updated version expectations and OpenAPI metadata to 0.8.22 so binaries, docs, and schema stay aligned.

**Follow-ups**
- None.


## 2025-11-01 — OCI bundle delivery support (plan)

**Goal**
- Implement the remaining delivery type from the roadmap by allowing applications to source Kubernetes manifests from OCI bundles.

**Scope**
- Accept an `ociBundle` source in app deploy/validate payloads with registry URL, ref, and optional credential reference.
- Fetch OCI artifacts, extract manifest documents, and reuse existing manifest apply/render flows with structured logging.
- Update services, API layer, store release metadata, docs (README, API reference, ENVIRONMENT), tutorials, and changelog/versioning.
- Extend tests (service + handler + release persistence) and OpenAPI spec for the new delivery type.

**Acceptance Criteria**
- `/v1/projects/{id}/apps` and `/v1/apps/validate` accept `ociBundle` payloads, fetch the referenced artifact, and deploy or validate manifests with deterministic summaries.
- OCI bundles support registry credentials and reject untrusted hosts per existing allowlist logic.
- Documentation (README quickstart, API reference, tutorial) and changelog describe the OCI bundle workflow; version bumped with tests and CI passing.

**Risks**
- OCI registry interactions may require new dependencies and could introduce large downloads or credential handling bugs.
- Tarball parsing must guard against path traversal and oversized payloads to avoid resource exhaustion.
- Need to ensure release history and validation caching stay consistent across delivery types.

**Out of Scope**
- Signature verification or cosign attestations for OCI bundles.
- Changes to scheduler, reconciler, or watcher components.
- Enhancements to existing Helm or Git delivery flows beyond necessary integration adjustments.


## 2025-11-01 — OCI bundle delivery support (outcome)

**What changed**
- Added an OCI bundle fetcher backed by go-containerregistry, host/IP allowlisting, and tarball safety checks; wired `ociBundle` payloads through the API, service planner, deployment flow, and release metadata with digest recording.
- Extended validation/deploy responses with bundle ref/digest fields, exposed release history for bundles, and updated OpenAPI schemas, docs (README, API reference, architecture), and a new tutorial covering end-to-end usage.
- Added regression tests for validation and deploy paths plus release persistence expectations, introduced bundle fetch stubs for tests, and bumped the version to 0.8.24.

**Follow-ups**
- Consider exposing configurable bundle size limits via configuration if larger artifacts become common.
- Explore verifying OCI bundle signatures (cosign/notation) as part of the fetch pipeline once registries standardise signature metadata.


## 2025-10-31 — Samples scaffolding foundation

**Goal**
- Deliver the roadmap Epoch IV milestone for scaffolding the samples suite so new users have an executable starting point with shared logging helpers.

**Scope**
- Create `samples/` hierarchy with shared library (`lib/common.sh`), global `.env.samples`, and template README for sample directories.
- Provide at least one example sample directory (e.g., `00-bootstrap/`) with stub scripts (`README.md`, `.env.example`, `curl.sh`, `verify.sh`, `cleanup.sh`) that demonstrate logging helpers without touching live APIs.
- Integrate scripts with `set -euo pipefail` and structured logging consistent with repo standards.

**Acceptance Criteria**
- Running `samples/lib/common.sh` sourced by sample scripts emits `log_step` and `log_info` messages with timestamps.
- Sample README documents prerequisites and execution steps; scripts pass shellcheck (where applicable) and are referenced from the main repo documentation.
- Tests updated to cover helper behaviour (Go or shell) and documentation references added to README and docs.

**Risks**
- Shell scripts might introduce portability issues on macOS/Linux if commands differ; mitigate by using POSIX-compatible constructs.
- Without real API endpoints, samples could become stale; mitigate by documenting placeholders and adding unit tests for helper logging.

**Out of Scope**
- Executing live API or Kubernetes operations, KinD automation, or GitHub Actions runners for samples.
- Implementing the full samples CI workflow or advanced delivery examples (Helm, Git, OCI bundles).


**Outcome**
- Introduced a reusable samples library with logging helpers, environment bootstrap, and a 00-bootstrap dry-run sample.
- Added Go tests covering the helper functions plus documentation updates (README, tutorials, environment references) and bumped the binary/OpenAPI version to 0.8.25.
- Updated roadmaps and templates to track samples work and added CI checklist coverage for `samples/` changes.

**Follow-ups**
- Expand the samples suite with live deployment flows (Helm/Git/OCI) and integrate shellcheck in CI when future scripts land.
