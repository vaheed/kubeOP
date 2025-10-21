#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"

source "${ROOT_DIR}/.env.samples"
if [ -f "${SCRIPT_DIR}/.env" ]; then
        # shellcheck disable=SC1090
        source "${SCRIPT_DIR}/.env"
fi
# shellcheck disable=SC1090
source "${ROOT_DIR}/lib/common.sh"

require_env AUTH_TOKEN
require_env USER_EMAIL
require_env CLUSTER_ID
require_env PROJECT_ID
require_command jq
require_command curl
require_command base64

log_step "Generating bootstrap payloads"
USER_PAYLOAD=$(jq -n --arg email "$USER_EMAIL" --arg cluster "$CLUSTER_ID" '{name:"Sample User",email:$email,clusterId:$cluster}')
PROJECT_PAYLOAD=$(jq -n --arg project "$PROJECT_ID" '{name:$project}')

log_info "POST /v1/users/bootstrap"
log_info "Payload: ${USER_PAYLOAD}"
log_info "curl -sS -H 'Authorization: Bearer ***' -H 'Content-Type: application/json' \\
     -d '${USER_PAYLOAD}' ${KUBEOP_BASE_URL}/v1/users/bootstrap"

log_info "POST /v1/projects"
log_info "Payload: ${PROJECT_PAYLOAD}"
log_info "curl -sS -H 'Authorization: Bearer ***' -H 'Content-Type: application/json' \\
     -d '${PROJECT_PAYLOAD}' ${KUBEOP_BASE_URL}/v1/projects"

log_warn "The bootstrap sample is a dry run. Remove the log_info wrappers to execute the calls."
