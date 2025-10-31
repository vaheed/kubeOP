#!/usr/bin/env bash
set -euo pipefail

NAMESPACE=${1:?namespace required}
RESOURCE=${2:?resource required}
TIMEOUT=${KUBEOP_WAIT_TIMEOUT:-180}

start=$(date +%s)
while true; do
  if kubectl -n "${NAMESPACE}" get pod -l "k8s-app=${RESOURCE}" >/dev/null 2>&1; then
    ready=$(kubectl -n "${NAMESPACE}" get pod -l "k8s-app=${RESOURCE}" -o jsonpath='{range .items[*]}{.status.phase}{"\n"}{end}')
    if [[ "${ready}" == *"Running"* ]]; then
      echo "[e2e] ${RESOURCE} in namespace ${NAMESPACE} is ready"
      break
    fi
  fi
  now=$(date +%s)
  if (( now - start > TIMEOUT )); then
    echo "[e2e] timeout waiting for ${RESOURCE} in ${NAMESPACE}" >&2
    exit 1
  fi
  sleep 5
done
