---
outline: deep
---

# kubeOP Documentation

kubeOP is a multi-tenant application platform for Kubernetes. The platform bundles a PostgreSQL-backed management API, a controller-runtime operator, and high-availability admission webhooks that enforce tenant boundaries and delivery policies.

This site replaces the legacy Markdown notes and serves as the canonical source for deploying, operating, and extending kubeOP.

The published site is hosted at https://vaheed.github.io/kubeOP/ with a scoped `/kubeOP/` base path so that static assets load
correctly on GitHub Pages. When running `npm run docs:dev` locally, open `http://localhost:5173/kubeOP/` (or override the base
with `DOCS_BASE=/`) to mirror the production layout.

## Platform components

- **Manager API** (`cmd/manager`): REST API that stores metadata in PostgreSQL, exposes cluster/tenant/project/app CRUD, and bootstraps kubeOP components into registered clusters using server-side apply.
- **Operator** (`cmd/operator`): Reconciles kubeOP CRDs, handles multi-source delivery (Image, Git, Helm, Raw), orchestrates hooks, revisions, DNS, TLS, quotas, and NetworkPolicy enforcement.
- **Admission Webhooks** (`cmd/admission`): Mutate and validate incoming CRDs and workload manifests to ensure ownership, namespace alignment, resource quotas, allowed registries, and egress baselines.
- **Supporting assets**: Helm chart (`charts/kubeop-operator`, package via `go run ./tools/helmchart`), Docker Compose stacks (`docker-compose.yml`, `deploy/compose/`), Kind assets (`e2e/kind-config.yaml`), and integration tests (`tests/`).

## What to expect

This documentation is organised as follows:

| Section | Purpose |
| --- | --- |
| [Roadmap](./roadmap.md) | Phase-by-phase breakdown of the Foundation (00–04) deliverables, code, tests, and manifests. |
| [Getting Started](./getting-started.md) | Local evaluation with Kind/Compose/Helm and real-cluster deployment instructions. |
| [Bootstrap Guide](./bootstrap-guide.md) | End-to-end workflow: cluster → tenant → project → app → DNS/TLS → usage → invoice → analytics. |
| [Operations](./operations.md) | Day-2 procedures: upgrades, backups, migrations, secrets/KMS, RBAC, multi-tenant policies, quotas, network policy, and egress. |
| [Delivery](./delivery.md) | Delivery flows (Git/Helm/Image/Raw), revision storage, hooks, rollout/rollback playbooks. |
| [API Reference](./api-reference.md) | Endpoint catalogue with request/response samples. |
| [Production Hardening](./production-hardening.md) | Guidance for sizing, HA, PDBs, probes, observability, and disaster recovery. |

Diagrams live in `docs/diagrams/` and are embedded where relevant.

## Release cadence

- Version metadata lives in `VERSION` and `CHANGELOG.md` following SemVer and Keep a Changelog.
- Container images are built in CI (`.github/workflows/ci.yaml`) and published to `ghcr.io/vaheed/kubeop/<pkg>` for releases and `ghcr.io/vaheed/kubeop/<pkg>-dev` for `develop` snapshots.
- kubeOP is distributed under the MIT License (see `LICENSE`).

Continue to [Getting Started](./getting-started.md) to bootstrap kubeOP locally.
