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
