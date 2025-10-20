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
