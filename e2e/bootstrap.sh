#!/usr/bin/env bash
set -euo pipefail

ARTIFACTS_DIR=${ARTIFACTS_DIR:-artifacts}
mkdir -p "$ARTIFACTS_DIR"

collect_diagnostics() {
  echo "[e2e] Collecting diagnostics to $ARTIFACTS_DIR" >&2
  kubectl get events -A --sort-by=.lastTimestamp > "$ARTIFACTS_DIR/events.txt" 2>&1 || true
  kubectl get all -A -o wide > "$ARTIFACTS_DIR/resources.txt" 2>&1 || true
  kubectl -n kubeop-system get deploy,pods,svc,cm,sa,roles,rolebindings,clusterroles,clusterrolebindings -o wide > "$ARTIFACTS_DIR/kubeop-system.txt" 2>&1 || true
  # Operator logs if present
  kubectl -n kubeop-system logs deploy/kubeop-operator --tail=-1 > "$ARTIFACTS_DIR/operator.log" 2>&1 || true
}

trap collect_diagnostics EXIT

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
  --set image.repository=kubeop-operator --set image.tag=dev --set image.pullPolicy=IfNotPresent \
  --set leaderElection.enabled=false
echo "[e2e] Waiting for operator rollout"
if ! kubectl -n kubeop-system rollout status deploy/kubeop-operator --timeout=300s; then
  echo "[e2e] Operator failed to become ready within timeout" >&2
  echo "[e2e] Dumping kubeop-system state" >&2
  kubectl -n kubeop-system get deploy,pods,svc -o wide >&2 || true
  echo "[e2e] Operator describe" >&2
  kubectl -n kubeop-system describe deploy/kubeop-operator >&2 || true
  echo "[e2e] Operator logs" >&2
  kubectl -n kubeop-system logs deploy/kubeop-operator --tail=-1 >&2 || true
  exit 1
fi

echo "[e2e] Bootstrap complete"
