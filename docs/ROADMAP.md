> **What this page explains**: kubeOP's upcoming milestones.
> **Who it's for**: Contributors and stakeholders tracking future work.
> **Why it matters**: Aligns expectations and highlights where help is needed.

# Roadmap

The roadmap follows Keep a Changelog style headings and evolves with each release cycle. Version numbers will tag code milestones.

## Near term

- **Docs 2.0**: Launch the VitePress site with auto-publishing to GitHub Pages.
- **Tenant insights**: Add per-tenant dashboards summarizing usage, quotas, and rollout health.
- **Kubernetes 1.30 validation**: Extend integration tests to validate against the latest upstream release.

## Mid term

- **Pluggable storage**: Abstract PostgreSQL so operators can choose managed services or run in HA mode.
- **OPA integration**: Hook into Open Policy Agent for per-request policy checks.
- **GitOps bridge**: Sync App revisions to Git repositories for declarative reconciliation.

## Long term

- **Marketplace**: Curated catalog of vetted Helm charts and OCI bundles.
- **Edge orchestration**: Offline-first agent syncing for edge clusters with intermittent connectivity.
- **Self-service sandboxing**: Automated preview environments per pull request, with tear-down policies.

