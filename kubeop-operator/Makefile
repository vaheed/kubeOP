SHELL := /bin/bash

MODULE := github.com/vaheed/kubeOP/kubeop-operator
KUSTOMIZE_DIR := kustomize
CRD_DIR := $(KUSTOMIZE_DIR)/bases/crds

CONTROLLER_GEN_BIN := $(shell go env GOBIN)
ifeq ($(CONTROLLER_GEN_BIN),)
CONTROLLER_GEN_BIN := $(shell go env GOPATH)/bin
endif
CONTROLLER_GEN := $(CONTROLLER_GEN_BIN)/controller-gen
KUBECONFORM_FLAGS ?= -strict -summary

.PHONY: all crds lint build run test tidy install uninstall validate tools

all: test

crds: tools
	set -euo pipefail
	$(CONTROLLER_GEN) \
	object:headerFile=./hack/boilerplate.go.txt \
	paths=./apis/... \
	output:crd:dir=$(CRD_DIR) \
	crd:crdVersions=v1

lint:
	set -euo pipefail
	go vet ./...

build:
	set -euo pipefail
	go build ./cmd/manager

run:
	set -euo pipefail
	go run ./cmd/manager

test:
	set -euo pipefail
	go test ./...

tidy:
	set -euo pipefail
	go mod tidy

install:
	set -euo pipefail
	kubectl apply -k $(KUSTOMIZE_DIR)/overlays/default

uninstall:
	set -euo pipefail
	kubectl delete -k $(KUSTOMIZE_DIR)/overlays/default --ignore-not-found

validate:
	set -euo pipefail
	kubectl kustomize $(KUSTOMIZE_DIR)/overlays/default | kubeconform $(KUBECONFORM_FLAGS) -schema-location default

tools:
	set -euo pipefail
	if ! command -v $(CONTROLLER_GEN) >/dev/null 2>&1; then \
	go install sigs.k8s.io/controller-tools/cmd/controller-gen@v0.16.3; \
fi
