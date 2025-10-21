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

require_command curl

log_step "Checking /healthz"
if curl -fsS "${KUBEOP_BASE_URL}/healthz" >/dev/null; then
        log_info "Health check OK"
else
        log_error "/healthz did not return 200"
        exit 1
fi

log_step "Checking /readyz"
if curl -fsS "${KUBEOP_BASE_URL}/readyz" >/dev/null; then
        log_info "Readiness check OK"
else
        log_error "/readyz did not return 200"
        exit 1
fi
