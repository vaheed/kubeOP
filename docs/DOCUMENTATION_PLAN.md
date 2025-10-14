# Documentation Plan (Q4 2025)

## Target Audiences
- **New Developer** – needs setup, mental model of packages, and an immediate list of required checks before opening a PR.
- **Maintainer** – tracks release cadence, dependency decisions, and CI/documentation coupling rules.
- **Operations/SRE** – depends on runbooks, readiness semantics, logging formats, and recovery workflows.

## Doc Set Inventory
| Filename | Audience | Purpose | Status |
| --- | --- | --- | --- |
| `README.md` | All | Product overview, quickstart, links to docs site. | ✅ Updated for 0.3.1 readiness guard & logging cues. |
| `docs/ARCHITECTURE.md` | New dev, maintainer | System diagram, package map, background jobs. | ⚠️ Needs scheduler + kube manager error-path annotations. |
| `docs/API_REFERENCE.md` | Dev, maintainer | Endpoint catalog with curl snippets and examples. | ✅ Ready for release; readiness samples refreshed. |
| `docs/ENVIRONMENT.md` | Ops, dev | Environment variables, defaults, secret expectations. | ✅ Non-expiring kubeconfig notes and DNS provider matrices captured. |
| `docs/OPERATIONS.md` | Ops | Deployment, readiness, backups, troubleshooting. | ✅ Ready; readiness logs + error payload documented. |
| `docs/SECURITY.md` | Maintainer, ops | Auth, encryption, rotation, threat assumptions. | ⚠️ Expand on key escrow, penetration cadence. |
| `docs/CONTRIBUTING.md` | New dev | Workflow, lint/test/doc expectations, PR checklist. | ⚠️ Needs example PR narrative tied to doc plan updates. |
| `docs/CHANGELOG.md` | Maintainer | Release history (Keep a Changelog). | ✅ 0.3.1 entry added. |
| `docs/ROADMAP.md` | Maintainer, ops | Sequenced backlog with sprint-ready steps. | ✅ Expanded with observability & resiliency tasks. |
| `docs/openapi.yaml` | Dev | Canonical API contract. | ✅ Readiness 503 example captured. |
| `docs/PROJECT_AUDIT.md` | Maintainer | Codebase review notes & improvement suggestions. | ✅ New in 0.3.1. |

## Gaps, Risks & Assumptions
- Runbooks for database failure, kubeconfig key rotation, and scheduler incident response still pending – blockers for production go-live.
- Assume GitHub Actions supplies secrets; no secret values stored in repo.
- Observability strategy (metrics, tracing, alerting) remains uncommitted; roadmap now tracks it explicitly.
- Multi-cluster scale limits and SLOs for `/readyz` + scheduler not defined; operations doc flags temporary guidance.

## Near-Term Draft Targets (owned by docs guild)
- **ARCHITECTURE.md** – add background worker swimlanes (Mermaid) and failure-handling notes.
- **ENVIRONMENT.md** – ensure DNS provider credentials and logging verbosity env vars stay current.
- **CONTRIBUTING.md** – include “docs touched” checklist snippet and remind authors to update this plan + roadmap when scope changes.
- **SECURITY.md** – spell out encryption-key custody model and quarterly review cadence.

## Inline Improvements Completed This Iteration
- README quickstart now calls out readiness 503 behaviour and structured logs for dashboards.
- API reference + OpenAPI spec include explicit `service unavailable` payload example.
- Operations doc links readiness status to logging output for monitoring integrations.

## Open Questions
1. Which team owns long-lived kubeconfig encryption keys and how will rotation be audited?
2. Do we document a fallback for `/readyz` when Postgres is read-only (e.g., maintenance mode)?
3. Should Docsify publishing move to an automated workflow or remain manual? (Action item on platform team.)
4. Can we publish sample Grafana alerts once readiness metrics are exposed?

## Roadmap Alignment
- Roadmap immediate-next steps updated with observability metrics, readiness alerting, and manifest drift automation.
- Documentation work items from this plan are cross-linked in `docs/ROADMAP.md` (Phase 1 docs/runbooks lane).
