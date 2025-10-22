# Frequently asked questions

## What problem does kubeOP solve?

kubeOP centralises multi-cluster operations. Instead of running per-cluster controllers, you register clusters with kubeOP and
manage tenants, projects, and application delivery through a single API.

## Does kubeOP replace Argo CD or Flux?

kubeOP can deploy manifests, Helm charts, Git repositories, and OCI bundles, but it focuses on tenant onboarding and operational
controls. GitOps tools may complement kubeOP for application release automation.

## How does kubeOP authenticate administrators?

Administrators supply JWTs signed with `ADMIN_JWT_SECRET`. Tokens typically contain `{"role":"admin"}` and optional metadata for
auditing.

## Where are kubeconfigs stored?

kubeconfigs are encrypted at rest using the key configured in `KCFG_ENCRYPTION_KEY` and decrypted only when required for API or
operator actions.

## Does kubeOP run inside the cluster?

No. kubeOP runs as an external control plane (VM, container, or Kubernetes deployment). It deploys the `kubeop-operator` inside
each managed cluster to reconcile `App` CRDs.

## How do I enable multi-tenancy isolation?

Set Pod Security levels via `POD_SECURITY_LEVEL`, configure namespace quotas (`KUBEOP_DEFAULT_*`), and use the bootstrap API to
create per-tenant namespaces with dedicated RBAC.

## Is there a UI?

The repository currently ships API endpoints and reusable `curl` snippets. Contributions for UI components are welcome.

## How do I report security issues?

Email <security@kubeOP.io>. See [`docs/SECURITY.md`](SECURITY.md) for disclosure timelines.
