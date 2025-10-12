# Documentation Plan

## Target Audiences
- **New developer** – needs environment setup, architecture orientation, testing workflow, and contribution checklist.
- **Maintainer** – cares about release process, CI/CD wiring, dependency decisions, and upgrade paths.
- **Operations/SRE** – focuses on deployment, configuration, runbooks, security controls, and recovery procedures.

## Doc Set Overview
| Filename | Audience | Purpose | Status |
| --- | --- | --- | --- |
| `README.md` | All | High-level product overview, quickstart, curl examples, doc map. | Updated with production notes & scheduler summary. |
| `docs/ARCHITECTURE.md` | New dev, maintainer | Mermaid system diagram, package layout, data flow. | Refreshed; needs per-component SLA callouts. |
| `docs/API_REFERENCE.md` | Dev, maintainer | Endpoint catalog with curl samples & auth notes. | Updated; ensure OpenAPI parity each release. |
| `docs/ENVIRONMENT.md` | Ops, dev | Environment variables, defaults, secret guidance. | Updated with scheduler + DNS keys. |
| `docs/OPERATIONS.md` | Ops | Deployment matrix, maintenance, backup, troubleshooting. | Expanded; runbook gaps remain. |
| `docs/SECURITY.md` | Maintainer, ops | Auth model, encryption, hardening checklist. | Expanded with scheduler scope; awaiting threat model sign-off. |
| `docs/CONTRIBUTING.md` | New dev | PR checklist, branch strategy, testing/linting expectations. | New draft; needs community guidelines later. |
| `docs/CHANGELOG.md` | Maintainer | Release notes following Keep a Changelog + SemVer. | Updated to v0.3.0. |
| `docs/DOCUMENTATION_PLAN.md` | Maintainer | Living inventory, priorities, and risks. | New. |

## Gaps, Risks & Assumptions
- Runbooks for database recovery, cluster credential rotation, and scheduler incident response still pending.
- Assume GitHub Actions secrets provide production credentials; no secrets stored in repo.
- Threat model and pen-test results unavailable; SECURITY doc flags placeholders.
- Multi-cluster scaling expectations (max clusters, tick frequency) still undefined; tied to roadmap open questions.
- Docsify site deployment assumed via GitHub Pages (gh-pages branch) – confirm pipeline before publishing changes.

## Draft Key Docs (next revisions)
- **README.md** – keep quickstart runnable, add pointer to documentation plan, highlight scheduler helper and manifest builders, ensure change badges reflect v0.3.0.
- **ARCHITECTURE.md** – extend Mermaid diagram with `ClusterHealthScheduler`, manifest helpers, and controller-runtime interactions.
- **API_REFERENCE.md** – call out `/v1/clusters` base64 requirement, include scheduler observability endpoints when added.
- **ENVIRONMENT.md** – document new scheduler timeout knobs once exposed, reiterate DNS/Ingress label dependencies.
- **OPERATIONS.md** – add health tick troubleshooting checklist, include log sampling guidance, and backup/restore runbooks.
- **CONTRIBUTING.md** – enforce lint/test/build/doc requirements, add PR checklist table, reference documentation plan updates.
- **SECURITY.md** – clarify key rotation process and SLOs for incident response; add assumptions about secrets providers.
- **CHANGELOG.md** – maintain Keep a Changelog structure, include links to releases and highlight documentation updates.

## Assorted Actions
- Embed runnable curl examples across docs; verify scripts under `docs/QUICKSTART_*.md` stay in sync with OpenAPI schema.
- Use Mermaid diagrams where architecture or workflows are non-trivial (architecture, operations, roadmap status board).
- Align doc updates with tests in the same PR per repository policy; mention this coupling in CONTRIBUTING.
