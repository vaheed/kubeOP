# Internal Worklog

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
