# Documentation audit and information architecture proposal

This document captures the current Markdown inventory and outlines the future-state information architecture for kubeOP.

## Inventory (current)

| Location | Purpose today | Notes |
| --- | --- | --- |
| `README.md` | High-level overview, highlights, and a 10-minute Docker Compose quickstart. | Lacks table of contents and references to several docs that no longer exist (CLI, Security, Operations). |
| `CHANGELOG.md` | Keep a Changelog log covering versions through `0.14.1`. | `Unreleased` section empty; prior entries mention removed docs. |
| `CONTRIBUTING.md` | Contribution process (Go tooling, docs expectations). | No details on docs linting, vale, or VitePress site.
| `CODE_OF_CONDUCT.md` | Contributor Covenant v2.1. | Up to date. |
| `SUPPORT.md` | Support policy and channels. | Minimal but accurate. |
| `docs/index.md` | VitePress landing page stub. | Uses placeholder copy and outdated navigation. |
| `docs/QUICKSTART.md` | Docker Compose walkthrough. | Does not mention Kubernetes bootstrap or snippets. |
| `docs/INSTALL.md` | Combined install guide. | Needs version matrix, packaging, and explicit prerequisites. |
| `docs/ENVIRONMENT.md` | Environment variable reference. | Missing many keys added to `internal/config.Config`. |
| `docs/ARCHITECTURE.md` | Architecture description with diagrams. | Diagram outdated, lacks operator depiction and scheduler flow. |
| `docs/API.md` | Endpoint overview with curl samples. | Table omits many routes; references `/v1/openapi` which is not implemented. |
| `docs/ROADMAP.md` | Short roadmap bullets. | Needs timeboxes, acceptance criteria, risks per requirement. |
| `docs/STYLEGUIDE.md` | Markdown guidance. | Missing lint configuration details and vale term list. |
| `docs/examples/docker-compose.env` | Sample `.env` for Docker Compose. | Values not aligned with latest config keys. |
| `docs/examples/kubeop-deployment.yaml` | Kubernetes deployment example. | Missing sidecar/env documentation. |
| `docs/_snippets/diagram-*.md` | Mermaid diagram sources shared across docs. | Ensure runtime rendering is validated via `npm run docs:build`. |
| `.github/pull_request_template.md` | PR checklist. | Does not mention docs lint/link checks. |
| `.github/workflows/ci.yml` | CI pipeline (Go build/test, docs build, lychee). | Vale not enforced for docs. |
| `.markdownlint.json` | Markdownlint configuration. | Removed in favour of Vale-only checks. |
| `.vale.ini` | Vale configuration. | Points to `KubeOP` style, but styles directory empty. |

## Gaps identified

- Core references (Configuration, API, Operations, Security, Troubleshooting, FAQ, Glossary) either missing or outdated.
- No reusable snippets for environment tables or curl headers.
- Diagram sources scattered; ensure reusable Mermaid snippets instead of binary exports.
- Style linting wired in package scripts but lacks shared styles and CI enforcement.
- README and docs cross-reference pages that were removed in previous revisions, leading to broken links.
- GitHub Actions does not run Markdownlint or Vale.

## Target information architecture

The restructured docs will follow this tree:

```
README.md
CHANGELOG.md
CONTRIBUTING.md
CODE_OF_CONDUCT.md
SUPPORT.md

/docs
  IA.md                      (this document)
  index.md                   (VitePress landing page)
  STYLEGUIDE.md              (authoring rules)
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
  openapi.yaml
  /examples
    docker-compose.env
    docker-compose.yaml
    kube/namespace.yaml
    curl/register-cluster.sh
  /_snippets
    env-table.md
    curl-auth.md
    docker-compose-prereqs.md
    diagram-architecture.md
    diagram-delivery-flow.md
    diagram-scheduler.md
  /.vitepress
    config.ts
    sidebar.ts
```

## Mapping legacy content → new IA

| Legacy location | Disposition |
| --- | --- |
| `docs/QUICKSTART.md` | Rewrite to align with new Quickstart, reference snippets, and link to Install/Operations. |
| `docs/INSTALL.md` | Replace with structured install guide covering Docker Compose and Kubernetes. |
| `docs/ENVIRONMENT.md` | Expand to include every key from `internal/config.Config` and operator settings. |
| `docs/ARCHITECTURE.md` | Replace diagrams, add scheduler/operator sections, include Mermaid source reference. |
| `docs/API.md` | Rebuild with endpoint catalogue grouped by resource and sample payloads per handler. |
| `README.md` | Overhaul with TOC, value proposition, architecture summary, and quickstart pointers. |
| `docs/ROADMAP.md` | Rewrite as time-boxed roadmap entries with acceptance criteria and risks. |
| `docs/STYLEGUIDE.md` | Update with lint configuration, Vale rules, snippets usage, and tone guidance. |
| `docs/examples/` | Refresh with verified Compose/Kubernetes manifests and curl scripts. |
| `.github/workflows/ci.yml` | Ensure Vale execution, docs build, and link check. |
| `.github/pull_request_template.md` | Update checklist for docs linting and Mermaid validation. |
| `.markdownlint.json` & `.vale.ini` | Remove markdownlint and keep Vale styles under `.github/vale/styles/`. |

