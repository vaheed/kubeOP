# Frequently asked questions

## Why run kubeOP outside the cluster?

Running the control plane outside managed clusters avoids per-cluster upgrades and reduces blast radius. kubeOP stores state in a
single PostgreSQL database and connects to clusters via kubeconfigs, letting you operate fleets consistently.

## Does kubeOP replace GitOps tools?

No. kubeOP focuses on tenant onboarding, application delivery, and lifecycle automation. You can integrate with GitOps pipelines
by using `/v1/templates/*` to render manifests and `/v1/projects/{id}/apps/{appId}/delivery` to validate digests before committing.

## How do I authenticate end users?

The API assumes administrative automation. Front kubeOP with your identity provider or API gateway to issue JWTs with the
`{"role":"admin"}` claim. For tenant access, distribute kubeconfigs created by `/v1/users/bootstrap`.

## Can I manage clusters in multiple regions?

Yes. Cluster metadata includes `environment`, `region`, and arbitrary `tags`. Use these fields to scope automation or routing.

## What workloads can I deploy?

kubeOP renders Deployments, StatefulSets, Jobs, CronJobs, Services, and Ingresses. Sources include container images, Helm charts,
Git repositories (plain manifests or Kustomize), and OCI bundles.

## How are secrets stored?

kubeOP encrypts kubeconfigs using AES-GCM derived from `KCFG_ENCRYPTION_KEY`. User-provided secrets are stored in PostgreSQL and
synced to Kubernetes via the API when attaching to apps. Use your own secret manager for long-term storage and rotate regularly.

## How do I export audit logs?

Use structured logs emitted by the API (`internal/http/middleware` adds request IDs and actors). Combine with `/v1/projects/{id}/events`
and `/v1/projects/{id}/apps/{appId}/releases` for change history.

## What happens if the operator is down?

Existing workloads continue running, but new deployments or status updates stall. kubeOP logs errors when it cannot reach the
operator. Restart the operator deployment and re-run the requested operation.

## Can I disable DNS automation?

Yes. Leave `DNS_PROVIDER` empty to skip DNS automation. kubeOP will still render ingress hosts using `PAAS_DOMAIN` when provided.

## How do I contribute documentation?

Follow the [STYLEGUIDE](STYLEGUIDE.md), run `npm run docs:lint`, and confirm `npm run docs:build` passes before
submitting a pull request.
