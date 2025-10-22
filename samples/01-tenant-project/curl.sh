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

: "${DRY_RUN:=1}"

require_env AUTH_TOKEN
require_env TENANT_EMAIL
require_env TENANT_NAME
require_env PROJECT_NAME
require_env CLUSTER_ID
require_command jq
require_command curl

AUTH_HEADER="Authorization: Bearer ${AUTH_TOKEN}"
CONTENT_HEADER="Content-Type: application/json"

call_api() {
        local method="$1"
        local path="$2"
        local payload="${3:-}"

        log_step "${method} ${path}"
        if [ -n "$payload" ]; then
                log_info "Payload: ${payload}"
        else
                log_info "No payload"
        fi

        if [ "${DRY_RUN}" = "0" ]; then
                local response
                if [ -n "$payload" ]; then
                        response=$(curl -fsS -H "$AUTH_HEADER" -H "$CONTENT_HEADER" -X "$method" \
                                -d "$payload" "${KUBEOP_BASE_URL}${path}")
                else
                        response=$(curl -fsS -H "$AUTH_HEADER" -X "$method" "${KUBEOP_BASE_URL}${path}")
                fi
                log_info "Response: ${response}"
        else
                local command="curl -sS -H 'Authorization: Bearer ***' -H 'Content-Type: application/json' -X ${method}"
                if [ -n "$payload" ]; then
                        command+=" -d '${payload}'"
                fi
                command+=" ${KUBEOP_BASE_URL}${path}"
                log_warn "DRY_RUN=1 skipping ${method} ${path}"
                log_info "${command}"
        fi
}

log_step "Preparing tenant bootstrap payloads"
USER_PAYLOAD=$(jq -n --arg name "$TENANT_NAME" --arg email "$TENANT_EMAIL" --arg cluster "$CLUSTER_ID" \
        '{name:$name,email:$email,clusterId:$cluster}')
PROJECT_PAYLOAD=$(jq -n --arg name "$PROJECT_NAME" --arg email "$TENANT_EMAIL" --arg user "$TENANT_NAME" --arg cluster "$CLUSTER_ID" \
        '{name:$name,userEmail:$email,userName:$user,clusterId:$cluster}')

call_api "POST" "/v1/users/bootstrap" "$USER_PAYLOAD"
call_api "POST" "/v1/projects" "$PROJECT_PAYLOAD"

log_info "Tenant onboarding sample complete"
if [ "${DRY_RUN}" = "1" ]; then
        log_warn "Set DRY_RUN=0 to execute the curl commands against the API"
fi
