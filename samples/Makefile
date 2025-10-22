SHELL := /usr/bin/env bash
.SHELLFLAGS := -euo pipefail -c

ROOT_DIR := $(abspath $(dir $(lastword $(MAKEFILE_LIST))))
BOOTSTRAP_DIR := $(ROOT_DIR)/00-bootstrap
ONBOARD_DIR := $(ROOT_DIR)/01-tenant-project
HELM_REPO_DIR := $(ROOT_DIR)/02-helm-repo

.DEFAULT_GOAL := help

.PHONY: help bootstrap-env bootstrap-dry-run bootstrap-verify bootstrap-clean bootstrap-all \
        onboard-env onboard-plan onboard-exec onboard-verify onboard-clean onboard-all \
        helm-repo-env helm-repo-plan helm-repo-exec helm-repo-verify helm-repo-clean helm-repo-all

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
        @echo "  helm-repo-env      Copy 02-helm-repo/.env.example to 02-helm-repo/.env if missing"
        @echo "  helm-repo-plan     Build Helm repo payloads and validation request (DRY_RUN=1)"
        @echo "  helm-repo-exec     Deploy Helm repo app via the API (DRY_RUN=0)"
        @echo "  helm-repo-verify   Fetch app status and delivery metadata"
        @echo "  helm-repo-clean    Delete the Helm repo app and remove sample data"
        @echo "  helm-repo-all      Run plan, exec, verify, and clean in order"

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

helm-repo-env:
        @if [ -f "$(HELM_REPO_DIR)/.env" ]; then \
                echo "Helm repo .env already present at $(HELM_REPO_DIR)/.env"; \
        else \
                echo "Creating Helm repo .env from example"; \
                cp "$(HELM_REPO_DIR)/.env.example" "$(HELM_REPO_DIR)/.env"; \
                echo "Populate $(HELM_REPO_DIR)/.env before running other targets."; \
        fi

helm-repo-plan:
        @if [ ! -f "$(HELM_REPO_DIR)/.env" ]; then \
                echo "Missing $(HELM_REPO_DIR)/.env. Run 'make -C samples helm-repo-env' first." >&2; \
                exit 1; \
        fi
        @echo "==> Previewing Helm repository deployment"
        "$(HELM_REPO_DIR)/curl.sh"

helm-repo-exec:
        @if [ ! -f "$(HELM_REPO_DIR)/.env" ]; then \
                echo "Missing $(HELM_REPO_DIR)/.env. Run 'make -C samples helm-repo-env' first." >&2; \
                exit 1; \
        fi
        @echo "==> Deploying Helm repository app"
        DRY_RUN=0 "$(HELM_REPO_DIR)/curl.sh"

helm-repo-verify:
        @if [ ! -f "$(HELM_REPO_DIR)/.env" ]; then \
                echo "Missing $(HELM_REPO_DIR)/.env. Run 'make -C samples helm-repo-env' first." >&2; \
                exit 1; \
        fi
        @echo "==> Inspecting Helm repository deployment"
        "$(HELM_REPO_DIR)/verify.sh"

helm-repo-clean:
        @echo "==> Cleaning Helm repository sample"
        "$(HELM_REPO_DIR)/cleanup.sh"

helm-repo-all: helm-repo-plan helm-repo-exec helm-repo-verify helm-repo-clean
        @echo "==> Helm repository sample completed"
