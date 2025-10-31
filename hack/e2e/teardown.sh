#!/usr/bin/env bash
set -euo pipefail

CLUSTER_NAME=${1:?cluster name required}
ARTIFACTS_DIR=${2:?artifacts dir required}

echo "[e2e] dumping cluster info"
if command -v kubectl >/dev/null 2>&1; then
  kubectl cluster-info dump >"${ARTIFACTS_DIR}/cluster-info.log" 2>&1 || true
fi

echo "[e2e] deleting kind cluster ${CLUSTER_NAME}"
kind delete cluster --name "${CLUSTER_NAME}" >"${ARTIFACTS_DIR}/kind-delete.log" 2>&1 || true
