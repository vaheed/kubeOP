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

SAMPLE_KEY="helm-repo"
DATA_DIR="${SAMPLES_DATA_DIR}/${SAMPLE_KEY}"
mkdir -p "${DATA_DIR}"

require_env AUTH_TOKEN
require_env PROJECT_ID
require_env APP_NAME
require_env HELM_CHART_URL
require_env CONTAINER_PORT
require_env SERVICE_PORT
require_env SERVICE_TYPE
require_command jq
require_command curl

AUTH_HEADER="Authorization: Bearer ${AUTH_TOKEN}"
CONTENT_HEADER="Content-Type: application/json"

log_step "Preparing Helm deployment payload"
log_info "Project: ${PROJECT_ID}"
log_info "App name: ${APP_NAME}"
log_info "Helm chart URL: ${HELM_CHART_URL}"

if ! PORTS_JSON=$(jq -n \
        --argjson container "${CONTAINER_PORT}" \
        --argjson service "${SERVICE_PORT}" \
        --arg serviceType "${SERVICE_TYPE}" \
        '[{containerPort:$container, servicePort:$service, serviceType:$serviceType}]'); then
        log_error "Failed to encode service ports as JSON"
        exit 1
fi

VALUES_JSON='{}'
if [ -n "${HELM_VALUES_JSON:-}" ]; then
        if ! VALUES_JSON=$(printf '%s' "${HELM_VALUES_JSON}" | jq -c '.' 2>/dev/null); then
                log_error "HELM_VALUES_JSON must be valid JSON"
                exit 1
        fi
        if ! printf '%s' "${VALUES_JSON}" | jq -e 'type=="object"' >/dev/null; then
                log_error "HELM_VALUES_JSON must encode a JSON object"
                exit 1
        fi
else
        log_info "No HELM_VALUES_JSON provided; using empty overrides"
fi

if ! PAYLOAD=$(jq -n \
        --arg project "${PROJECT_ID}" \
        --arg name "${APP_NAME}" \
        --arg chart "${HELM_CHART_URL}" \
        --argjson ports "${PORTS_JSON}" \
        --argjson values "${VALUES_JSON}" \
        '{projectId:$project,name:$name,helm:{chart:$chart},ports:$ports} | if ($values | length) > 0 then (.helm.values = $values) else . end'); then
        log_error "Failed to construct deployment payload"
        exit 1
fi

log_info "Payload: ${PAYLOAD}"

validate_endpoint="${KUBEOP_BASE_URL}/v1/apps/validate"
deploy_endpoint="${KUBEOP_BASE_URL}/v1/projects/${PROJECT_ID}/apps"
validation_file="${DATA_DIR}/validation.json"
deploy_file="${DATA_DIR}/deploy.json"
app_id_file="${DATA_DIR}/app_id"

log_step "Validating Helm deployment"
if [ "${DRY_RUN}" = "0" ]; then
        validation_response=$(curl -fsS -H "${AUTH_HEADER}" -H "${CONTENT_HEADER}" -X POST \
                -d "${PAYLOAD}" "${validate_endpoint}")
        printf '%s\n' "${validation_response}" | jq '.' > "${validation_file}"
        log_info "Validation response saved to ${validation_file}"
else
        log_warn "DRY_RUN=1 skipping validation request"
        log_info "curl -sS -H 'Authorization: Bearer ***' -H 'Content-Type: application/json' -X POST \\" 
        log_info "     -d '${PAYLOAD}' ${validate_endpoint}"
fi

log_step "Deploying Helm chart"
if [ "${DRY_RUN}" = "0" ]; then
        deploy_response=$(curl -fsS -H "${AUTH_HEADER}" -H "${CONTENT_HEADER}" -X POST \
                -d "${PAYLOAD}" "${deploy_endpoint}")
        printf '%s\n' "${deploy_response}" | jq '.' > "${deploy_file}"
        app_id=$(printf '%s' "${deploy_response}" | jq -r '.appId // empty')
        if [ -z "${app_id}" ]; then
                log_error "Failed to parse appId from deployment response"
                exit 1
        fi
        printf '%s\n' "${app_id}" > "${app_id_file}"
        log_info "Stored appId ${app_id} in ${app_id_file}"
else
        log_warn "DRY_RUN=1 skipping deployment request"
        log_info "curl -sS -H 'Authorization: Bearer ***' -H 'Content-Type: application/json' -X POST \\" 
        log_info "     -d '${PAYLOAD}' ${deploy_endpoint}"
fi

log_info "Helm repository sample complete"
if [ "${DRY_RUN}" = "1" ]; then
        log_warn "Set DRY_RUN=0 in .env to execute the API calls"
fi
