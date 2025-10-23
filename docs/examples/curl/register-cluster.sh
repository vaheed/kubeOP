#!/usr/bin/env bash
set -euo pipefail

if [[ -z "${KUBEOP_TOKEN:-}" ]]; then
  echo "KUBEOP_TOKEN is required. Export it before running." >&2
  exit 1
fi

KUBECONFIG_PATH=${KUBECONFIG_PATH:-$HOME/.kube/config}
if [[ ! -f "${KUBECONFIG_PATH}" ]]; then
  echo "KUBECONFIG_PATH ${KUBECONFIG_PATH} not found" >&2
  exit 1
fi

echo "Encoding kubeconfig from ${KUBECONFIG_PATH}" >&2
B64=$(base64 -w0 <"${KUBECONFIG_PATH}")

API_URL=${API_URL:-http://localhost:8080}
AUTH_HEADER=("-H" "Authorization: Bearer ${KUBEOP_TOKEN}")

PAYLOAD=$(jq -n --arg name "${CLUSTER_NAME:-edge-cluster}" \
  --arg owner "${CLUSTER_OWNER:-platform}" \
  --arg env "${CLUSTER_ENVIRONMENT:-staging}" \
  --arg region "${CLUSTER_REGION:-eu-west}" \
  --arg contact "${CLUSTER_CONTACT:-}" \
  --arg desc "${CLUSTER_DESCRIPTION:-}" \
  --arg kubeconfig "$B64" \
  '{name:$name,owner:$owner,environment:$env,region:$region,contact:$contact,description:$desc,kubeconfig_b64:$kubeconfig}')

echo "Registering cluster against ${API_URL}" >&2
curl -sS "${AUTH_HEADER[@]}" \
  -H 'Content-Type: application/json' \
  -d "${PAYLOAD}" \
  "${API_URL}/v1/clusters" | jq
