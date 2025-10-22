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
require_command curl

AUTH_HEADER="Authorization: Bearer ${AUTH_TOKEN}"

APP_ID="${APP_ID:-}"
if [ -z "${APP_ID}" ] && [ -f "${APP_ID_FILE}" ]; then
        APP_ID=$(tr -d '\n' < "${APP_ID_FILE}")
        if [ -n "${APP_ID}" ]; then
                log_info "Loaded appId ${APP_ID} from ${APP_ID_FILE}"
        fi
fi

delete_endpoint="${KUBEOP_BASE_URL}/v1/projects/${PROJECT_ID}/apps/${APP_ID}"

log_step "Cleaning Helm repository sample"
if [ -n "${APP_ID}" ]; then
        if [ "${DRY_RUN}" = "0" ]; then
                curl -fsS -H "${AUTH_HEADER}" -X DELETE "${delete_endpoint}" >/dev/null
                log_info "Deleted app ${APP_ID}"
        else
                log_warn "DRY_RUN=1 skipping app deletion"
                log_info "curl -sS -H 'Authorization: Bearer ***' -X DELETE ${delete_endpoint}"
        fi
else
        log_warn "No appId found; skipping API delete"
fi

if [ -d "${DATA_DIR}" ]; then
        rm -rf "${DATA_DIR}"
        log_info "Removed ${DATA_DIR}"
else
        log_info "No data directory present"
fi
