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
require_env PROJECT_NAME
require_command curl
require_command jq

AUTH_HEADER="Authorization: Bearer ${AUTH_TOKEN}"

log_step "Previewing project lookup"
PROJECT_QUERY="/v1/projects?limit=10"
if [ "${DRY_RUN}" = "0" ]; then
        RESPONSE=$(curl -fsS -H "$AUTH_HEADER" "${KUBEOP_BASE_URL}${PROJECT_QUERY}")
        log_info "Response: ${RESPONSE}"
        log_warn "Filter the response locally to locate project '${PROJECT_NAME}'"
else
        log_warn "DRY_RUN=1 skipping GET ${PROJECT_QUERY}"
        log_info "curl -sS -H 'Authorization: Bearer ***' ${KUBEOP_BASE_URL}${PROJECT_QUERY} | jq"
fi

log_step "Previewing tenant namespace kubeconfig renewal"
RENEW_PATH="/v1/projects/<project-id>/kubeconfig/renew"
log_info "Replace <project-id> with the identifier returned from /v1/projects"
if [ "${DRY_RUN}" = "0" ]; then
        log_info "Executing POST ${RENEW_PATH}"
        curl -fsS -H "$AUTH_HEADER" -H "Content-Type: application/json" -X POST \
                "${KUBEOP_BASE_URL}${RENEW_PATH}" >/dev/null
        log_info "Kubeconfig renewal succeeded"
else
        log_warn "DRY_RUN=1 skipping POST ${RENEW_PATH}"
        log_info "curl -sS -H 'Authorization: Bearer ***' -H 'Content-Type: application/json' -X POST ${KUBEOP_BASE_URL}${RENEW_PATH}"
fi
