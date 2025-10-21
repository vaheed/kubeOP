#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"

source "${ROOT_DIR}/.env.samples"
# shellcheck disable=SC1090
source "${ROOT_DIR}/lib/common.sh"

DATA_DIR="${SAMPLES_DATA_DIR}"

log_step "Cleaning temporary sample artifacts"
if [ -d "$DATA_DIR" ]; then
        rm -rf "$DATA_DIR"
        log_info "Removed ${DATA_DIR}"
else
        log_info "No data directory present"
fi
