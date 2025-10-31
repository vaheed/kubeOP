---
outline: deep
---

# Platform Policy API

“Platform” refers to control‑plane level settings that apply across your
organization’s workloads and clusters. In kubeOP terms, it sits above your
hierarchy of cluster → tenant → project → app and governs guardrails and global
behavior (e.g., image registry allowlist, egress baselines, autoscaling
defaults). These settings are managed through the Manager API so users never
need to helm/kubectl things by hand.

This document covers policy (allowlist/egress/quotas). The Manager persists
policy in a ConfigMap (`kubeop-policy`) and syncs it to the admission webhooks.

## Endpoints

- GET /v1/platform/policy – returns current policy
- PUT /v1/platform/policy – upserts policy and rolls admission

Request body:

```json
{
  "imageAllowlist": ["docker.io", "ghcr.io"],
  "egressBaseline": ["10.0.0.0/8", "172.16.0.0/12"],
  "quotaMax": { "requestsCPU": "4", "requestsMemory": "8Gi" }
}
```

## Admission sync

The Manager writes policy keys into a ConfigMap and ensures the admission
Deployment consumes it via `envFrom`. It then bumps a rollout annotation to
apply changes.

## Manager-side validation

When creating/updating Apps via the Manager API, the image registry host is
validated against the effective allowlist (from env or the `kubeop-policy`
ConfigMap). Requests with non-allowlisted registries are rejected with `400`.

## Examples

```bash
curl -sS -X PUT http://localhost:18080/v1/platform/policy \
  -H 'Content-Type: application/json' \
  -d '{"imageAllowlist":["docker.io","ghcr.io"],"egressBaseline":["10.0.0.0/8"]}'
```
