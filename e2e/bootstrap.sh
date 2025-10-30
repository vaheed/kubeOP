#!/usr/bin/env bash
set -euo pipefail

KUBECTL=${KUBECTL:-kubectl}

echo "[e2e] Applying kubeOP namespace and CRDs"
$KUBECTL apply -f deploy/k8s/namespace.yaml
$KUBECTL apply -f deploy/k8s/crds/

echo "[e2e] Installing operator chart via Helm (replicas=1, mocks enabled)"
helm upgrade --install kubeop-operator charts/kubeop-operator -n kubeop-system --create-namespace \
  --set replicaCount=1 --set mocks.enabled=true
kubectl -n kubeop-system rollout status deploy/kubeop-operator --timeout=180s

echo "[e2e] Bootstrap complete"
