SHELL := /bin/bash

PROJECT := github.com/vaheed/kubeop
VERSION := $(shell cat VERSION)
GIT_SHA := $(shell git rev-parse --short HEAD 2>/dev/null || echo dev)

GO ?= go
DOCKER ?= docker
BIN_DIR := bin

MANAGER_BIN := bin/manager
OPERATOR_BIN := bin/operator
ADMISSION_BIN := bin/admission

MANAGER_IMAGE := ghcr.io/vaheed/kubeop/manager
OPERATOR_IMAGE := ghcr.io/vaheed/kubeop/operator

PLATFORMS := linux/amd64,linux/arm64

.PHONY: all right tidy fmt vet build test clean run

all: build

right: fmt vet tidy build

fmt:
	$(GO) fmt ./...

vet:
	CGO_ENABLED=0 $(GO) vet ./...

tidy:
	$(GO) mod tidy

build: $(MANAGER_BIN) $(OPERATOR_BIN) $(ADMISSION_BIN)

LDFLAGS := -s -w -X $(PROJECT)/internal/version.Version=$(VERSION) -X $(PROJECT)/internal/version.Build=$(GIT_SHA) -X $(PROJECT)/internal/version.BuildDate=$$(date -u +%Y-%m-%dT%H:%M:%SZ)

$(BIN_DIR):
	@mkdir -p $(BIN_DIR)

$(MANAGER_BIN): | $(BIN_DIR)
	CGO_ENABLED=0 $(GO) build -ldflags "$(LDFLAGS)" -o $@ ./cmd/manager

$(OPERATOR_BIN): | $(BIN_DIR)
	CGO_ENABLED=0 $(GO) build -ldflags "$(LDFLAGS)" -o $@ ./cmd/operator

$(ADMISSION_BIN): | $(BIN_DIR)
	CGO_ENABLED=0 $(GO) build -ldflags "$(LDFLAGS)" -o $@ ./cmd/admission



clean:
	rm -rf bin
	rm -f manager operator admission dnsmock acmemock

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

.PHONY: kind-up-hpa
kind-up-hpa: ## Create Kind, install metrics-server and apply chart with HPA defaults
	$(MAKE) kind-up
	KUBEOP_INSTALL_METRICS_SERVER=true bash e2e/bootstrap.sh

.PHONY: prod-install
prod-install: ## Install production stack: cert-manager, metrics-server, ExternalDNS (PowerDNS), kubeOP chart with prod values
	@set -euo pipefail; \
	NS=$${NS:-kubeop-system}; \
	LE_EMAIL=$${LE_EMAIL:-you@example.com}; \
	PDNS_API_KEY=$${PDNS_API_KEY:-}; \
	PDNS_SERVER=$${PDNS_SERVER:-http://powerdns.example.com:8081}; \
	DOMAIN_FILTER=$${DOMAIN_FILTER:-example.com}; \
	OPERATOR_DIGEST=$${OPERATOR_DIGEST:-}; \
	ADMISSION_DIGEST=$${ADMISSION_DIGEST:-}; \
	echo "[prod] Installing cert-manager"; \
	kubectl create namespace cert-manager >/dev/null 2>&1 || true; \
	helm repo add jetstack https://charts.jetstack.io >/dev/null 2>&1 || true; \
	helm repo update >/dev/null 2>&1 || true; \
	helm upgrade --install cert-manager jetstack/cert-manager -n cert-manager --set crds.enabled=true; \
	echo "[prod] Applying Let's Encrypt ClusterIssuer"; \
	cat <<'EOF' | kubectl apply -f -
	apiVersion: cert-manager.io/v1
	kind: ClusterIssuer
	metadata:
	  name: letsencrypt-prod
	spec:
	  acme:
	    email: ${LE_EMAIL}
	    server: https://acme-v02.api.letsencrypt.org/directory
	    privateKeySecretRef:
	      name: le-account-key
	    solvers:
	      - http01:
	          ingress:
	            class: nginx
	EOF
	echo "[prod] Installing metrics-server"; \
	helm repo add metrics-server https://kubernetes-sigs.github.io/metrics-server/ >/dev/null 2>&1 || true; \
	helm repo update >/dev/null 2>&1 || true; \
	helm upgrade --install metrics-server metrics-server/metrics-server -n kube-system --create-namespace; \
	if [ -n "$$PDNS_API_KEY" ]; then \
	  echo "[prod] Installing ExternalDNS (PowerDNS)"; \
	  kubectl -n kube-system create secret generic pdns-credentials --from-literal=api-key="$$PDNS_API_KEY" >/dev/null 2>&1 || true; \
	  helm repo add external-dns https://kubernetes-sigs.github.io/external-dns/ >/dev/null 2>&1 || true; \
	  helm repo update >/dev/null 2>&1 || true; \
	  helm upgrade --install external-dns external-dns/external-dns -n kube-system \
	    --set provider=powerdns \
	    --set env[0].name=PDNS_API_KEY \
	    --set env[0].valueFrom.secretKeyRef.name=pdns-credentials \
	    --set env[0].valueFrom.secretKeyRef.key=api-key \
	    --set extraArgs[0]=--pdns-server=$$PDNS_SERVER \
	    --set domainFilters[0]=$$DOMAIN_FILTER \
	    --set policy=sync \
	    --set txtOwnerId=kubeop \
	    --set interval=1m; \
	else \
	  echo "[prod] Skipping ExternalDNS (PowerDNS): set PDNS_API_KEY to enable"; \
	fi; \
	echo "[prod] Installing kubeOP operator + admission (values-prod)"; \
	kubectl create namespace $$NS >/dev/null 2>&1 || true; \
	HELM_TAGS=""; \
	if [ -n "$$OPERATOR_DIGEST" ]; then HELM_TAGS="$$HELM_TAGS --set image.digest=$$OPERATOR_DIGEST"; fi; \
	if [ -n "$$ADMISSION_DIGEST" ]; then HELM_TAGS="$$HELM_TAGS --set admission.image.digest=$$ADMISSION_DIGEST"; fi; \
	HELM_POLICY=""; \
	if [ -n "$$KUBEOP_IMAGE_ALLOWLIST" ]; then HELM_POLICY="$$HELM_POLICY --set admission.policy.imageAllowlist=$$KUBEOP_IMAGE_ALLOWLIST"; fi; \
	if [ -n "$$KUBEOP_EGRESS_BASELINE" ]; then HELM_POLICY="$$HELM_POLICY --set admission.policy.egressBaseline=$$KUBEOP_EGRESS_BASELINE"; fi; \
	helm upgrade --install kubeop-operator charts/kubeop-operator -n $$NS -f charts/kubeop-operator/values-prod.yaml $$HELM_TAGS $$HELM_POLICY --set mocks.enabled=false; \
	kubectl -n $$NS rollout status deploy/kubeop-operator --timeout=180s; \
	kubectl -n $$NS rollout status deploy/kubeop-admission --timeout=180s || true; \
	echo "[prod] Done. Set KUBEOP_IMAGE_ALLOWLIST and KUBEOP_EGRESS_BASELINE environment for Manager as needed."

.PHONY: sync-policy
sync-policy: ## Sync admission policy from environment to cluster Deployment
	@set -e; \
	NS=$${NS:-kubeop-system}; \
	if [ -z "$$KUBEOP_IMAGE_ALLOWLIST$$KUBEOP_EGRESS_BASELINE" ]; then \
	  echo "Set KUBEOP_IMAGE_ALLOWLIST and/or KUBEOP_EGRESS_BASELINE in your environment"; \
	  exit 1; \
	fi; \
	kubectl -n $$NS set env deploy/kubeop-admission \
	  KUBEOP_IMAGE_ALLOWLIST="$$KUBEOP_IMAGE_ALLOWLIST" \
	  KUBEOP_EGRESS_BASELINE="$$KUBEOP_EGRESS_BASELINE"; \
	kubectl -n $$NS rollout status deploy/kubeop-admission --timeout=120s

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
	helm package charts/kubeop-operator --destination dist/charts

.PHONY: docs-gen
docs-gen:
	GO111MODULE=on $(GO) run ./tools/docsgen
