# Documentation Information Architecture

This sitemap captures the current structure of the kubeOP documentation set and how it
maps to the product surface area.

## Current inventory

```
README.md
CHANGELOG.md
CONTRIBUTING.md
CODE_OF_CONDUCT.md
SUPPORT.md

/docs
├── API.md                 # REST endpoints, payload examples, OpenAPI pointers
├── ARCHITECTURE.md        # Control plane, scheduler, operator responsibilities
├── CLI.md                 # CLI/`curl` usage patterns and authentication snippets
├── ENVIRONMENT.md         # Configuration keys consumed by internal/config
├── FAQ.md                 # Common operational questions mapped to service logic
├── GLOSSARY.md            # Terminology aligned with store/models.go types
├── IA.md                  # This sitemap
├── INSTALL.md             # Docker Compose & Kubernetes install paths
├── OPERATIONS.md          # Backup, monitoring, maintenance-mode guidance
├── QUICKSTART.md          # 10-minute workflow matching README quickstart
├── ROADMAP.md             # Engineering roadmap (auto-linked from README)
├── SECURITY.md            # Threat model, admin auth expectations
├── STYLEGUIDE.md          # Authoring guidance for docs contributions
├── TROUBLESHOOTING.md     # Symptom → remediation tied to service/logging
├── adr.md                 # Architectural decision log (append-only)
├── index.md               # VitePress landing content (mirrors README)
├── openapi.yaml           # Machine-readable API schema
├── _snippets/             # Reusable tables and curl header fragments
│   ├── curl-headers.md
│   └── env-table.md
├── examples/
│   ├── curl-app-deploy.md
│   ├── docker-compose.env
│   ├── docker-compose.md
│   ├── kubeop-deployment.yaml
│   └── kubernetes-agent.md
├── media/                 # Mermaid source and generated SVGs
│   ├── architecture.mmd
│   ├── architecture.svg
│   ├── data-flow.mmd
│   ├── data-flow.svg
│   └── puppeteer-config.json
├── public/robots.txt
└── .vitepress/
    ├── config.ts
    └── sidebar.ts
```

## Proposed adjustments

Planned documentation work (see `docs/ROADMAP.md`) introduces the following updates:

| Area | Planned change | Source of truth |
| --- | --- | --- |
| Releases | Add `docs/RELEASES.md` describing SemVer policy, branching, and how
  `internal/version/version.go` drives builds. Link from README and docs sidebar. |
| Operations | Expand `docs/OPERATIONS.md` with backup/restore runbooks and
  automation scripts referenced in the Ops track roadmap items. |
| API | Embed an interactive explorer sourced from `docs/openapi.yaml` once the Docs
  track automation lands. |
| Security | Document admin token rotation and the deprecation of the `DisableAuth`
  flag alongside roadmap hardening work. |
| CLI | Incorporate CLI binary usage (from the Mid-term UX track) without dropping
  existing curl snippets. |

## Navigation conventions

- Keep landing, quickstart, install, environment, API, operations, security, and
  troubleshooting topics directly under `docs/` to mirror the sidebar.
- Place task or tutorial content in `docs/examples/` and reference it from the main
  guides instead of duplicating steps.
- Store shared tables or request snippets in `_snippets/` and include them using
  VitePress `include` directives to avoid drift.
- Append new design decisions to `adr.md`; do not rewrite existing entries.

## Maintenance responsibilities

- Validate Markdown with `npm run docs:lint` before committing.
- Regenerate diagrams via `npm run docs:diagrams` when architecture or data flow
  changes.
- Keep README highlights, Quickstart, and `docs/index.md` in sync whenever new
  endpoints, configuration flags, or workflows ship.
