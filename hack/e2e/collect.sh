#!/usr/bin/env bash
set -euo pipefail

CLUSTER_NAME=${1:?cluster name required}
ARTIFACTS=${2:?artifact dir required}

if ! command -v kubectl >/dev/null 2>&1; then
  exit 0
fi

echo "[e2e] gathering controller logs"
for ns in kubeop kube-system; do
  mkdir -p "${ARTIFACTS}/logs/${ns}"
  kubectl -n "${ns}" get pods -o name | while read -r pod; do
    safe=${pod//\//-}
    kubectl -n "${ns}" logs "${pod}" --all-containers >"${ARTIFACTS}/logs/${ns}/${safe}.log" 2>&1 || true
  done
done

echo "[e2e] exporting custom resources"
for resource in tenants projects apps policies registries dnsrecords certificates; do
  kubectl get "${resource}.paas.kubeop.io" -A -o yaml >"${ARTIFACTS}/cr-${resource}.yaml" 2>&1 || true
done
