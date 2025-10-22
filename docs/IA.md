# Documentation Inventory & Information Architecture Plan

## Audit summary

The repository currently contains 60+ Markdown files spread across multiple ad-hoc directories (`docs/api/`, `docs/guides/`,
`docs/TUTORIALS/`, standalone guides, and README fragments). Content frequently overlaps (for example, cluster onboarding
appears in `docs/getting-started.md`, `docs/TUTORIALS/cluster-inventory-service.md`, and `docs/guides/tenants-projects-apps.md`),
which makes it difficult to maintain a single source of truth. Several documents use inconsistent heading levels, lack
front-matter, or mix reference and task content in the same page. The existing `docs/index.md` is a landing page stub without
navigation, and the VitePress configuration is absent, so the static site renders as an unstructured list. Style guidance is not
defined, resulting in mixed tone, bullet casing, and callout usage. The changelog, contributing guide, and code of conduct live
inside `docs/`, conflicting with GitHub conventions and the repository automation that expects these files at the project root.

## Proposed information architecture

The new information architecture reorganises documentation into a set of opinionated sections that map directly to the product
lifecycle. Each section has a dedicated Markdown file (or directory, where appropriate) with a clear purpose and ownership.

```
README.md
CHANGELOG.md
CONTRIBUTING.md
CODE_OF_CONDUCT.md
SUPPORT.md

docs/
├── STYLEGUIDE.md
├── QUICKSTART.md
├── INSTALL.md
├── ENVIRONMENT.md
├── ARCHITECTURE.md
├── API.md
├── CLI.md
├── OPERATIONS.md
├── SECURITY.md
├── TROUBLESHOOTING.md
├── ROADMAP.md
├── FAQ.md
├── GLOSSARY.md
├── _snippets/
│   ├── env-table.md
│   └── curl-headers.md
├── examples/
│   ├── docker-compose.md
│   ├── curl-app-deploy.md
│   └── kubernetes-agent.md
├── media/
│   ├── architecture.mmd
│   ├── architecture.svg
│   ├── data-flow.mmd
│   └── data-flow.svg
├── openapi.yaml
├── .vitepress/
│   ├── config.ts
│   └── sidebar.ts
└── index.md
```

The VitePress site will feature:

- **Overview** – `index.md` summarises kubeOP, key features, supported platforms, and links.
- **Quickstart** – step-by-step workflow to reach a running stack in <10 minutes.
- **Install** – supported deployment modes (Docker Compose and Kubernetes) with prerequisite matrix.
- **Configuration** – environment variable reference with defaults and examples sourced from `_snippets/env-table.md`.
- **Architecture** – diagrams, component responsibilities, and data flow.
- **API Reference** – top-level endpoint catalogue plus links to the machine-readable OpenAPI specification.
- **CLI** – binary flags, environment helpers, and reusable curl snippets.
- **Operations** – backups, upgrades, migrations, observability, and HA guidance.
- **Security** – threat model, RBAC, secrets handling, and vulnerability disclosure process.
- **Troubleshooting** – symptom → cause → resolution tables with copy-ready commands.
- **FAQ & Glossary** – canonical answers and terminology definitions.
- **Contributing/Roadmap/Changelog** – project governance surfaced at the repository root and linked from the docs home page.

## Mapping from legacy docs

| Legacy document | Action |
| --- | --- |
| `docs/API.md`, `docs/api/*.md` | Consolidated into `docs/API.md` with tables and request/response examples. |
| `docs/getting-started.md`, `docs/zero-to-prod.md`, tutorials under `docs/TUTORIALS/` | Content folded into `README.md`, `docs/QUICKSTART.md`, `docs/INSTALL.md`, and `docs/OPERATIONS.md`. |
| `docs/configuration.md`, `docs/ENVIRONMENT.md` | Replaced by the new `docs/ENVIRONMENT.md` with canonical environment variable table and `_snippets/env-table.md`. |
| `docs/architecture.md` | Rewritten with updated diagrams and linked assets under `docs/media/`. |
| `docs/operations.md`, `docs/security.md`, `docs/troubleshooting.md` | Reauthored to align with the new structure and tone. |
| `docs/contributing.md`, `docs/code-of-conduct.md`, `docs/changelog.md`, `docs/ROADMAP.md` | Promoted to root-level `CONTRIBUTING.md`, `CODE_OF_CONDUCT.md`, `CHANGELOG.md`, and refreshed `docs/ROADMAP.md`. |
| `docs/guides/*`, `docs/samples/*` | Distilled into reusable quickstart/install/operations examples and `docs/examples/`. |
| `docs/index.md` | Replaced by a VitePress-aware homepage that mirrors the README overview. |

This reorganisation delivers a consistent navigation hierarchy, establishes editorial standards, and eliminates duplicated or
stale guidance. Subsequent commits will implement the structure above, migrate content, and wire automated linting and site
builds.
