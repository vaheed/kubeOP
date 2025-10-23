# Documentation information architecture

This sitemap inventories the published documentation, the code it reflects, and the
immediate gaps detected while reviewing the repository (`cmd/`, `internal/`,
`kubeop-operator/`, `docs/`, and supporting assets).

## Current inventory

| Path | Audience | Source of truth | Notes |
| --- | --- | --- | --- |
| `README.md` | Evaluators, contributors | `cmd/api`, `internal/service`, `docs/*` | Accurate overview, architecture, and quickstart. Table must stay in sync with new docs such as `docs/RELEASES.md`. |
| `CHANGELOG.md` | Operators, release managers | `internal/version`, migrations | Up to v0.15.0 with security highlights. Needs `[Unreleased]` updates when roadmap shifts. |
| `docs/index.md` | Docs site landing page | All | Navigation matches current docs but lacks release policy link. |
| `docs/QUICKSTART.md`, `docs/examples/*` | Practitioners | Docker Compose samples, API scripts | Commands match repo Makefile and samples. |
| `docs/INSTALL.md` | Platform engineers | `docker-compose.yaml`, Kubernetes manifests | Covers Docker Compose/Kubernetes; relies on manual release matrix updates. |
| `docs/ENVIRONMENT.md` | Operators | `internal/config.Config` | Enumerates every config key (PodSecurity, DNS, operator image). |
| `docs/ARCHITECTURE.md` | Architects | `internal/service`, `internal/store`, `kubeop-operator` | Uses reusable Mermaid snippets and reflects scheduler/operator flows. |
| `docs/API.md`, `docs/openapi.yaml` | API consumers | `internal/api/*`, `testcase/api_*` | Mirrors routes in `internal/api/router.go`. Keep payload examples aligned with Go structs. |
| `docs/CLI.md` | Operators | `Makefile`, `cmd/api` | Documents binary build/run flows; no standalone CLI beyond API binary. |
| `docs/OPERATIONS.md` | SREs | Scheduler, logging, maintenance endpoints | Observability table mentions scheduler metrics that do not exist yet (gap). |
| `docs/SECURITY.md` | Security reviewers | `internal/crypto`, `pkg/security` | Matches SSRF/path sanitisation and secret handling. |
| `docs/TROUBLESHOOTING.md` | Support | API handlers, scheduler, operator | Symptom→fix table matches logs/endpoints. |
| `docs/FAQ.md`, `docs/GLOSSARY.md` | Onboarding | Entire system | Up to date. |
| `docs/STYLEGUIDE.md` | Doc authors | npm/VitePress config | Describes Vale linting and snippet reuse. |
| `docs/ROADMAP.md` | PMs, leads | This change | Rewritten in this commit to match code reality. |
| `docs/CODEQL.md` | Security | Hardened flows | References SSRF/path traversal mitigations. |
| `docs/RELEASES.md` | Release managers | (New) | Describes versioning and release process. |
| `_snippets/*` | Docs authors | Shared by README/docs | Mermaid/env snippets compiled by VitePress. |
| `.github/workflows/ci.yml` | Contributors | CI | Ensures gofmt, lint, tests, docs build, link check. |
| `.github/pull_request_template.md` | Contributors | CI, docs | Checklist aligned with repository rules. |

## Proposed adjustments

1. Surface the release policy everywhere docs point to lifecycle information
   (README, docs landing page, roadmap) so operators understand cadence.
2. Add explicit backlinks from feature/tech-debt issues to roadmap items to
   enforce the track/phase workflow.
3. Capture documentation ownership in future roadmap updates to avoid drift
   (each roadmap item references the doc sections to update).

## Sitemap by topic

```
README.md
CHANGELOG.md
CONTRIBUTING.md
CODE_OF_CONDUCT.md
SUPPORT.md

/docs
  index.md                (Docs landing page)
  RELEASES.md             (Release & versioning policy)
  QUICKSTART.md
  INSTALL.md
  ENVIRONMENT.md
  ARCHITECTURE.md
  API.md
  CLI.md
  OPERATIONS.md
  SECURITY.md
  TROUBLESHOOTING.md
  FAQ.md
  GLOSSARY.md
  ROADMAP.md
  STYLEGUIDE.md
  CODEQL.md
  IA.md                  (this sitemap)
  openapi.yaml
  /_snippets             (Mermaid + shared content)
  /examples              (Docker Compose env, curl helpers, kube manifests)
  /public                (VitePress assets)
```

## Observed gaps tied to code

- `docs/OPERATIONS.md` advertises scheduler metrics that do not exist in
  `internal/service/healthscheduler.go`; roadmap item GH-203 resolves this.
- No release documentation existed for `internal/version` and tag workflow
  prior to this change; addressed by `docs/RELEASES.md` and README updates.
- The docs map lacked an authoritative reference for issue templates and
  roadmap tracking; this update points contributors to `.github/ISSUE_TEMPLATE/*`.

## Maintenance ownership

- Documentation build and lint: `npm run docs:lint`, `npm run docs:build`.
- Link checking: `.github/workflows/ci.yml` `lycheeverse/lychee-action`.
- Roadmap stewardship: Staff PM/Tech Lead (owner placeholder on each roadmap
  item) responsible for keeping docs/ROADMAP.md in sync with GitHub issues.
