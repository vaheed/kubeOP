SHELL := /usr/bin/env bash
.SHELLFLAGS := -euo pipefail -c

ROOT_DIR := $(abspath $(dir $(lastword $(MAKEFILE_LIST))))
BOOTSTRAP_DIR := $(ROOT_DIR)/00-bootstrap

.DEFAULT_GOAL := help

.PHONY: help bootstrap-env bootstrap-dry-run bootstrap-verify bootstrap-clean bootstrap-all

help:
	@echo "Available sample targets:"
	@echo "  bootstrap-env      Copy 00-bootstrap/.env.example to 00-bootstrap/.env if it is missing"
	@echo "  bootstrap-dry-run  Preview bootstrap API payloads (does not call the API)"
	@echo "  bootstrap-verify   Check /healthz and /readyz endpoints"
	@echo "  bootstrap-clean    Remove temporary data produced by the bootstrap sample"
	@echo "  bootstrap-all      Run dry-run, verification, and cleanup in order"

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
