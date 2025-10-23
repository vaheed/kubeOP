---
name: Bug report
about: Report a defect observed in kubeOP services or operator
labels: []
title: "[Bug]: "
---

## Summary
_Brief description of the defect and affected module (e.g., `internal/api/projects`)._

## Environment
- kubeOP version (`/v1/version`):
- Deployment mode: ☐ Docker Compose ☐ Kubernetes
- Database: ☐ Postgres (version?)
- Operator version (if applicable):
- Additional context (flags, env vars):

## Steps to reproduce
1. _Command/API call with payload._
2. _Expected state._
3. _Observed state with logs/metrics._

## Expected outcome
_Describe the correct behaviour._

## Actual outcome
_Attach error messages, stack traces, or logs (redact secrets)._ 

## Impact assessment
- Reach (clusters/projects/users impacted):
- Severity: ☐ Sev1 ☐ Sev2 ☐ Sev3
- Labels applied: `track:api|delivery|data|ops|security|ux|docs`, `phase:<value>`

## Scope
- **Suspected areas**: _List files/packages to inspect._
- **Out of scope**: _Mention related features unaffected._

## Acceptance criteria
- [ ] Regression test added (e.g., under `testcase/`).
- [ ] Documentation updated if behaviour or workaround changes.
- [ ] Root cause documented in the fix PR.

## Dependencies
_Link to related feature/tech-debt issues._

## Risks & rollback
_Call out rollout considerations, migrations, or feature flags._

## Owner
_Assignee handling the fix._

## Verification plan
_Automated commands (`go test`, `go vet`, etc.) and manual validation steps._

## Roadmap linkage
- Related roadmap item (link):
