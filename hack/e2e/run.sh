#!/usr/bin/env bash
set -euo pipefail

ARTIFACTS_DIR=${ARTIFACTS_DIR:-/tmp/test-artifacts/e2e}
CLUSTER_NAME=${KUBEOP_E2E_CLUSTER:-kubeop-e2e}
KUBECONFIG_PATH=${KUBECONFIG:-"${ARTIFACTS_DIR}/kubeconfig"}
SUITE=${KUBEOP_E2E_SUITE:-all}
TIMEOUT=${KUBEOP_E2E_TIMEOUT:-3600}
ROOT_DIR=$(cd "$(dirname "$0")/.." && pwd)
SCRIPT_DIR=$(cd "$(dirname "$0")" && pwd)

mkdir -p "${ARTIFACTS_DIR}"
export KUBECONFIG="${KUBECONFIG_PATH}"

echo "[e2e] artifacts: ${ARTIFACTS_DIR}"
echo "[e2e] cluster: ${CLUSTER_NAME}"
echo "[e2e] suite: ${SUITE}"

echo "[e2e] creating kind cluster"
kind create cluster --name "${CLUSTER_NAME}" --config "${SCRIPT_DIR}/kind-config.yaml" --kubeconfig "${KUBECONFIG_PATH}" >"${ARTIFACTS_DIR}/kind-create.log" 2>&1
trap '"${SCRIPT_DIR}/teardown.sh" "${CLUSTER_NAME}" "${ARTIFACTS_DIR}"' EXIT

echo "[e2e] waiting for core components"
"${SCRIPT_DIR}/wait-for.sh" kube-system kube-dns

"${SCRIPT_DIR}/bootstrap.sh" "${CLUSTER_NAME}" "${ARTIFACTS_DIR}" "${SUITE}" "${ROOT_DIR}" "${TIMEOUT}"

echo "[e2e] collecting diagnostics"
"${SCRIPT_DIR}/collect.sh" "${CLUSTER_NAME}" "${ARTIFACTS_DIR}" || true
