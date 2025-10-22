SHELL := /usr/bin/env bash
.SHELLFLAGS := -euo pipefail -c

ROOT_DIR := $(abspath $(dir $(lastword $(MAKEFILE_LIST))))
BOOTSTRAP_DIR := $(ROOT_DIR)/00-bootstrap
ONBOARD_DIR := $(ROOT_DIR)/01-tenant-project

.DEFAULT_GOAL := help

.PHONY: help bootstrap-env bootstrap-dry-run bootstrap-verify bootstrap-clean bootstrap-all \
        onboard-env onboard-plan onboard-exec onboard-verify onboard-clean onboard-all

help:
        @echo "Available sample targets:"
        @echo "  bootstrap-env      Copy 00-bootstrap/.env.example to 00-bootstrap/.env if it is missing"
        @echo "  bootstrap-dry-run  Preview bootstrap API payloads (does not call the API)"
        @echo "  bootstrap-verify   Check /healthz and /readyz endpoints"
        @echo "  bootstrap-clean    Remove temporary data produced by the bootstrap sample"
        @echo "  bootstrap-all      Run dry-run, verification, and cleanup in order"
        @echo "  onboard-env        Copy 01-tenant-project/.env.example to 01-tenant-project/.env if missing"
        @echo "  onboard-plan       Preview tenant onboarding API payloads (DRY_RUN=1)"
        @echo "  onboard-exec       Execute tenant onboarding API calls (DRY_RUN=0)"
        @echo "  onboard-verify     Preview tenant verification commands"
        @echo "  onboard-clean      Remove tenant onboarding scratch data"
        @echo "  onboard-all        Run onboarding plan, verify, and cleanup"

bootstrap-env:
	@if [ -f "$(BOOTSTRAP_DIR)/.env" ]; then \
		echo "Bootstrap .env already present at $(BOOTSTRAP_DIR)/.env"; \
	else \
		echo "Creating bootstrap .env from example"; \
		cp "$(BOOTSTRAP_DIR)/.env.example" "$(BOOTSTRAP_DIR)/.env"; \
		echo "Populate $(BOOTSTRAP_DIR)/.env before running other targets."; \
	fi

bootstrap-dry-run:
	@if [ ! -f "$(BOOTSTRAP_DIR)/.env" ]; then \
		echo "Missing $(BOOTSTRAP_DIR)/.env. Run 'make -C samples bootstrap-env' first." >&2; \
		exit 1; \
	fi
	@echo "==> Previewing bootstrap payloads"
	"$(BOOTSTRAP_DIR)/curl.sh"

bootstrap-verify:
	@if [ ! -f "$(BOOTSTRAP_DIR)/.env" ]; then \
		echo "Missing $(BOOTSTRAP_DIR)/.env. Run 'make -C samples bootstrap-env' first." >&2; \
		exit 1; \
	fi
	@echo "==> Checking kubeOP health endpoints"
	"$(BOOTSTRAP_DIR)/verify.sh"

bootstrap-clean:
        @echo "==> Cleaning bootstrap sample artefacts"
        "$(BOOTSTRAP_DIR)/cleanup.sh"

bootstrap-all: bootstrap-dry-run bootstrap-verify bootstrap-clean
        @echo "==> Bootstrap sample completed"

onboard-env:
        @if [ -f "$(ONBOARD_DIR)/.env" ]; then \
                echo "Tenant onboarding .env already present at $(ONBOARD_DIR)/.env"; \
        else \
                echo "Creating tenant onboarding .env from example"; \
                cp "$(ONBOARD_DIR)/.env.example" "$(ONBOARD_DIR)/.env"; \
                echo "Populate $(ONBOARD_DIR)/.env before running other targets."; \
        fi

onboard-plan:
        @if [ ! -f "$(ONBOARD_DIR)/.env" ]; then \
                echo "Missing $(ONBOARD_DIR)/.env. Run 'make -C samples onboard-env' first." >&2; \
                exit 1; \
        fi
        @echo "==> Previewing tenant onboarding payloads"
        "$(ONBOARD_DIR)/curl.sh"

onboard-exec:
        @if [ ! -f "$(ONBOARD_DIR)/.env" ]; then \
                echo "Missing $(ONBOARD_DIR)/.env. Run 'make -C samples onboard-env' first." >&2; \
                exit 1; \
        fi
        @echo "==> Executing tenant onboarding calls"
        DRY_RUN=0 "$(ONBOARD_DIR)/curl.sh"

onboard-verify:
        @if [ ! -f "$(ONBOARD_DIR)/.env" ]; then \
                echo "Missing $(ONBOARD_DIR)/.env. Run 'make -C samples onboard-env' first." >&2; \
                exit 1; \
        fi
        @echo "==> Previewing tenant verification commands"
        "$(ONBOARD_DIR)/verify.sh"

onboard-clean:
        @echo "==> Cleaning tenant onboarding artefacts"
        "$(ONBOARD_DIR)/cleanup.sh"

onboard-all: onboard-plan onboard-verify onboard-clean
        @echo "==> Tenant onboarding sample completed"
