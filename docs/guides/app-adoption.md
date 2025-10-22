# App adoption during the Phase 5 cutover

Phase 5 moves kubeOP from the legacy watcher to the `kubeop-operator` controller as the
source of truth for workloads. Existing Deployments that pre-date the migration need to
be captured as `App` Custom Resources so the operator can reconcile and enforce guardrails.

This guide walks through adopting unmanaged Deployments into kubeOP ownership without
causing downtime.

## Prerequisites

- kubeOP v0.11.0+ with the operator rollout enabled on your target cluster.
- Cluster access (`kubectl`) with permissions to read workloads in the project namespace.
- Admin access to the kubeOP API for creating and updating Apps.

## 1. Inventory unmanaged Deployments

List workloads in the target namespace and filter for objects missing the
`kubeop.io/managed=true` label. These resources were created outside kubeOP.

```bash
set -euo pipefail

PROJECT_NAMESPACE=hello-platform
TARGET_KUBECONFIG=~/.kube/targets/hello.kubeconfig

kubectl --kubeconfig "${TARGET_KUBECONFIG}" -n "${PROJECT_NAMESPACE}" \
  get deploy -o json | \
  jq -r '.items[] | select(.metadata.labels["kubeop.io/managed"] != "true") | .metadata.name'
```

Capture the spec of each Deployment you intend to adopt so you can re-create it as an
`App` CRD payload.

```bash
set -euo pipefail

APP_NAME=hello-nginx
kubectl --kubeconfig "${TARGET_KUBECONFIG}" -n "${PROJECT_NAMESPACE}" \
  get deploy "${APP_NAME}" -o yaml > "${APP_NAME}.deployment.yaml"
```

## 2. Build the App spec

Translate the Deployment into an `App` definition. The simplest approach is to reuse the
existing container image, environment, and replicas in a raw manifest payload.

```bash
set -euo pipefail

APP_PAYLOAD=$(yq -o=json '{
  name: strenv(APP_NAME),
  manifests: [load(strenv(APP_NAME) + ".deployment.yaml") | dump]
}')
```

If the workload was originally installed via Helm, prefer adding the chart metadata so
future upgrades continue through kubeOP. See the Helm example in
[`docs/zero-to-prod.md`](../zero-to-prod.md#13-deploy-helm-charts-and-raw-manifests) for a
starting point that references the `https://helm.nginx.com/stable` repository.

## 3. Create the App through the API

Submit the translated payload to the kubeOP API. The operator will reconcile the new CRD
and label derived resources so the adoption becomes authoritative.

```bash
set -euo pipefail

API_ORIGIN=https://kubeop.example.com
PROJECT_ID=proj-uuid
TOKEN=$(pass kubeop/admin-token)

curl -sS -H "Authorization: Bearer ${TOKEN}" \
  -H "Content-Type: application/json" \
  -d "${APP_PAYLOAD}" \
  "${API_ORIGIN}/v1/projects/${PROJECT_ID}/apps" | jq
```

Review the response for the generated `appId` and `resourceVersion`. Track the
`resourceVersion` for future updates (scale, image change, etc.).

## 4. Verify ownership and cleanup

Confirm the operator has reconciled the workload and applied the management labels. The
Deployment should now include `kubeop.io/managed=true` alongside the app identifier.

```bash
set -euo pipefail

kubectl --kubeconfig "${TARGET_KUBECONFIG}" -n "${PROJECT_NAMESPACE}" \
  get deploy "${APP_NAME}" -o jsonpath='{.metadata.labels}'
```

Use the `/v1/projects/${PROJECT_ID}/apps/${APP_ID}` endpoint to confirm the CRD status
reports `Ready=True`. Once every legacy Deployment is adopted, you can disable any
remaining watcher deployments with confidence that kubeOP owns the lifecycle end-to-end.

## Troubleshooting

- If the API rejects the payload with validation errors, run
  `curl .../apps/validate` to inspect schema feedback before creating the App.
- When adopting Helm releases, ensure the rendered manifests do not include cluster-wide
  resources that clash with existing kubeOP-managed objects. Scope the chart values to
  the project namespace before creation.
- If the operator does not label derived resources, verify the Deployment has the
  `kubeop.io/app-id` annotation populated by the API response. Missing metadata indicates
  the App was not created successfully.
