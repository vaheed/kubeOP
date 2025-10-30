#!/usr/bin/env bash
set -euo pipefail

KUBECTL=${KUBECTL:-kubectl}

echo "[e2e] Applying kubeOP namespace and CRDs"
$KUBECTL apply -f deploy/k8s/namespace.yaml
$KUBECTL apply -f deploy/k8s/crds/

echo "[e2e] Building and loading mock images into Kind"
docker build -f deploy/Dockerfile.dnsmock -t dnsmock:dev .
docker build -f deploy/Dockerfile.acmemock -t acmemock:dev .
kind load docker-image dnsmock:dev --name kubeop-e2e || true
kind load docker-image acmemock:dev --name kubeop-e2e || true

echo "[e2e] Building and loading operator image into Kind"
docker build -f deploy/Dockerfile.operator -t kubeop-operator:dev .
kind load docker-image kubeop-operator:dev --name kubeop-e2e || true

echo "[e2e] Installing operator chart via Helm (replicas=1, mocks enabled)"
helm upgrade --install kubeop-operator charts/kubeop-operator -n kubeop-system --create-namespace \
  --set replicaCount=1 --set mocks.enabled=true \
  --set mocks.dns.image.repository=dnsmock --set mocks.dns.image.tag=dev --set mocks.dns.image.pullPolicy=IfNotPresent \
  --set mocks.acme.image.repository=acmemock --set mocks.acme.image.tag=dev --set mocks.acme.image.pullPolicy=IfNotPresent \
  --set image.repository=kubeop-operator --set image.tag=dev --set image.pullPolicy=IfNotPresent
kubectl -n kubeop-system rollout status deploy/kubeop-operator --timeout=180s

echo "[e2e] Bootstrap complete"
