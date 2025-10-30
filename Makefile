SHELL := /bin/bash

PROJECT := github.com/vaheed/kubeop
VERSION := $(shell cat VERSION)
GIT_SHA := $(shell git rev-parse --short HEAD 2>/dev/null || echo dev)

GO ?= go
DOCKER ?= docker

MANAGER_BIN := bin/manager
OPERATOR_BIN := bin/operator
ADMISSION_BIN := bin/admission
DELIVERY_BIN := bin/delivery
METER_BIN := bin/meter

MANAGER_IMAGE := ghcr.io/vaheed/kubeop/manager
OPERATOR_IMAGE := ghcr.io/vaheed/kubeop/operator

PLATFORMS := linux/amd64,linux/arm64

.PHONY: all right tidy fmt vet build test clean run

all: build

right: fmt vet tidy build

fmt:
	$(GO) fmt ./...

vet:
	$(GO) vet ./...

tidy:
	$(GO) mod tidy

build: $(MANAGER_BIN) $(OPERATOR_BIN) $(ADMISSION_BIN) $(DELIVERY_BIN) $(METER_BIN)

LDFLAGS := -s -w -X $(PROJECT)/internal/version.Version=$(VERSION) -X $(PROJECT)/internal/version.Build=$(GIT_SHA) -X $(PROJECT)/internal/version.BuildDate=$$(date -u +%Y-%m-%dT%H:%M:%SZ)

$(MANAGER_BIN):
	CGO_ENABLED=0 $(GO) build -ldflags "$(LDFLAGS)" -o $@ ./cmd/manager

$(OPERATOR_BIN):
	CGO_ENABLED=0 $(GO) build -ldflags "$(LDFLAGS)" -o $@ ./cmd/operator

$(ADMISSION_BIN):
	CGO_ENABLED=0 $(GO) build -ldflags "$(LDFLAGS)" -o $@ ./cmd/admission

$(DELIVERY_BIN):
	CGO_ENABLED=0 $(GO) build -ldflags "$(LDFLAGS)" -o $@ ./cmd/delivery

$(METER_BIN):
	CGO_ENABLED=0 $(GO) build -ldflags "$(LDFLAGS)" -o $@ ./cmd/meter

clean:
	rm -rf bin

run: $(MANAGER_BIN)
	./$(MANAGER_BIN)

# ----------------------------------------------------------------------------
# Docker images
# ----------------------------------------------------------------------------
.PHONY: image-manager image-operator push-images

image-manager:
	$(DOCKER) buildx build --platform $(PLATFORMS) \
	  --build-arg VERSION=$(VERSION) --build-arg VCS_REF=$(GIT_SHA) \
	  -f deploy/Dockerfile.manager -t $(MANAGER_IMAGE):$(VERSION) .

image-operator:
	$(DOCKER) buildx build --platform $(PLATFORMS) \
	  --build-arg VERSION=$(VERSION) --build-arg VCS_REF=$(GIT_SHA) \
	  -f deploy/Dockerfile.operator -t $(OPERATOR_IMAGE):$(VERSION) .

push-images:
	$(DOCKER) buildx build --platform $(PLATFORMS) -f deploy/Dockerfile.manager \
	  --build-arg VERSION=$(VERSION) --build-arg VCS_REF=$(GIT_SHA) \
	  -t $(MANAGER_IMAGE):$(VERSION) --push .
	$(DOCKER) buildx build --platform $(PLATFORMS) -f deploy/Dockerfile.operator \
	  --build-arg VERSION=$(VERSION) --build-arg VCS_REF=$(GIT_SHA) \
	  -t $(OPERATOR_IMAGE):$(VERSION) --push .

# ----------------------------------------------------------------------------
# Kind + E2E
# ----------------------------------------------------------------------------
KIND_CLUSTER ?= kubeop-e2e
KIND ?= kind
KUBECTL ?= kubectl

.PHONY: kind-up platform-up manager-up operator-up test-e2e down

kind-up:
	$(KIND) get clusters | grep -q "^$(KIND_CLUSTER)$$" || \
	  $(KIND) create cluster --name $(KIND_CLUSTER) --config e2e/kind-config.yaml

platform-up:
	$(KUBECTL) apply -f deploy/k8s/namespace.yaml
	$(KUBECTL) apply -f deploy/k8s/crds/
	helm upgrade --install kubeop-operator charts/kubeop-operator -n kubeop-system --create-namespace --set replicaCount=0

manager-up:
	docker compose up -d db
	sleep 3
	docker compose up -d manager

operator-up:
	helm upgrade --install kubeop-operator charts/kubeop-operator -n kubeop-system --create-namespace

.PHONY: kind-load-operator
kind-load-operator:
	# Build a local operator image and load into Kind, then deploy via Helm
	$(DOCKER) build -f deploy/Dockerfile.operator -t kubeop-operator:dev .
	kind load docker-image kubeop-operator:dev --name $(KIND_CLUSTER)
	helm upgrade --install kubeop-operator charts/kubeop-operator -n kubeop-system --create-namespace \
	  --set image.repository=kubeop-operator --set image.tag=dev --set image.pullPolicy=IfNotPresent --set replicaCount=1

test-e2e:
	KUBEOP_E2E=1 $(GO) test ./hack/e2e -v -timeout=20m

down:
	-$(KIND) delete cluster --name $(KIND_CLUSTER)
	-docker compose down -v

# ----------------------------------------------------------------------------
# Charts
# ----------------------------------------------------------------------------
.PHONY: helm-package
helm-package:
	@mkdir -p dist/charts
	tar -C charts -czf dist/charts/kubeop-operator-$(VERSION).tgz kubeop-operator
