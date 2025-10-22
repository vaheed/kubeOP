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
APP_ID_FILE="${DATA_DIR}/app_id"

require_env AUTH_TOKEN
require_env PROJECT_ID
require_env APP_NAME
require_command jq
require_command curl

AUTH_HEADER="Authorization: Bearer ${AUTH_TOKEN}"

APP_ID="${APP_ID:-}"
if [ -z "${APP_ID}" ]; then
        if [ -f "${APP_ID_FILE}" ]; then
                APP_ID=$(tr -d '\n' < "${APP_ID_FILE}")
                log_info "Loaded appId ${APP_ID} from ${APP_ID_FILE}"
        else
                log_error "APP_ID is not set and ${APP_ID_FILE} is missing"
                exit 1
        fi
fi

if [ -z "${APP_ID}" ]; then
        log_error "AppId is empty after loading configuration"
        exit 1
fi

app_endpoint="${KUBEOP_BASE_URL}/v1/projects/${PROJECT_ID}/apps/${APP_ID}"
delivery_endpoint="${app_endpoint}/delivery"

log_step "Fetching app status"
if [ "${DRY_RUN}" = "0" ]; then
        status_response=$(curl -fsS -H "${AUTH_HEADER}" "${app_endpoint}")
        printf '%s\n' "${status_response}" | jq '.'
else
        log_warn "DRY_RUN=1 skipping status request"
        log_info "curl -sS -H 'Authorization: Bearer ***' ${app_endpoint}"
fi

log_step "Fetching delivery metadata"
if [ "${DRY_RUN}" = "0" ]; then
        delivery_response=$(curl -fsS -H "${AUTH_HEADER}" "${delivery_endpoint}")
        printf '%s\n' "${delivery_response}" | jq '.'
else
        log_warn "DRY_RUN=1 skipping delivery request"
        log_info "curl -sS -H 'Authorization: Bearer ***' ${delivery_endpoint}"
fi

if command -v kubectl >/dev/null 2>&1; then
        log_step "kubectl inspection commands"
        log_info "kubectl --namespace <project-namespace> get deploy ${APP_NAME}" || true
        log_info "kubectl --namespace <project-namespace> get svc ${APP_NAME}" || true
else
        log_info "kubectl not detected; skipping cluster inspection hints"
fi
