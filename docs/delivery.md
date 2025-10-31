---
outline: deep
---

# Delivery Controller

This page documents how kubeOP delivers applications and tracks rollouts.

## Kinds

- Image: Deploy a container image into a namespace managed by kubeOP.
- Raw: Apply raw Kubernetes manifests (multi-doc YAML) via server-side apply.
- Git: (planned) Render manifests from a Git repo/ref/path.
- Helm: (planned) Render a chart + values, then apply.

## Revisions

- Image: The operator computes `status.revision` from a stable hash of the
  `spec.image` string. The Deployment template is annotated with
  `kubeop.io/revision` so rollouts produce a new ReplicaSet record.
- Raw/Git/Helm: will hash content and record revision similarly.

## Hooks

- `spec.hooks.pre` and `spec.hooks.post` run as one-shot Jobs per hook. The
  operator emits events on success/failure.

## Rollout and rollback

- Rollout occurs when the revision changes; Deployments reconcile to the new
  template and status reflects availability. Rollback can be implemented by
  writing a previous image or content to `spec`.

## Tests

- E2E: `hack/e2e/delivery_test.go` creates an Image app, captures `status.revision`,
  updates the image, and asserts that the revision changes (new rollout).
- Future: add polling of Deployment `.spec.template.spec.containers[0].image`
  and ReplicaSet history for deterministic validation across environments.

