#!/usr/bin/env bash
# shellcheck shell=bash
# Common logging and validation helpers for kubeOP sample scripts.

set -euo pipefail

__samples_timestamp() {
        date -u +"%Y-%m-%dT%H:%M:%SZ"
}

__samples_log() {
        local level="$1"
        shift
        local message="$*"
        local ts
        ts="$(__samples_timestamp)"
        if [ "$level" = "ERROR" ]; then
                printf '%s [%s] %s\n' "$ts" "$level" "$message" 1>&2
        else
                printf '%s [%s] %s\n' "$ts" "$level" "$message"
        fi
}

log_step() {
        __samples_log "STEP" "$*"
}

log_info() {
        __samples_log "INFO" "$*"
}

log_warn() {
        __samples_log "WARN" "$*"
}

log_error() {
        __samples_log "ERROR" "$*"
}

require_env() {
        if [ "$#" -ne 1 ]; then
                log_error "require_env expects exactly one argument"
                exit 1
        fi
        local name="$1"
        if [ "${!name-__unset__}" = "__unset__" ]; then
                log_error "Environment variable ${name} must be set"
                exit 1
        fi
        if [ -z "${!name}" ]; then
                log_error "Environment variable ${name} must not be empty"
                exit 1
        fi
}

require_command() {
        if [ "$#" -ne 1 ]; then
                log_error "require_command expects exactly one argument"
                exit 1
        fi
        local bin="$1"
        if ! command -v "$bin" >/dev/null 2>&1; then
                log_error "Required command not found: ${bin}"
                exit 1
        fi
}

with_temp_dir() {
        local dir
        dir="$(mktemp -d)"
        log_info "Created temp dir ${dir}"
        cleanup() {
                if [ -d "$dir" ]; then
                        rm -rf "$dir"
                        log_info "Removed temp dir ${dir}"
                fi
        }
        trap cleanup EXIT
        echo "$dir"
}
