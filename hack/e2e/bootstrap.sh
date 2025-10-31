#!/usr/bin/env bash
set -euo pipefail

CLUSTER_NAME=${1:?cluster name required}
ARTIFACTS=${2:?artifact dir required}
SUITE=${3:-all}
ROOT_DIR=${4:?root dir required}
TIMEOUT=${5:-3600}

export KUBECONFIG=${KUBECONFIG:-${ARTIFACTS}/kubeconfig}

echo "[e2e] applying CRDs"
kubectl apply -f "${ROOT_DIR}/deploy/k8s/crds" >"${ARTIFACTS}/apply-crds.log" 2>&1

echo "[e2e] creating namespaces"
kubectl apply -f "${ROOT_DIR}/deploy/k8s/namespace.yaml" >"${ARTIFACTS}/apply-namespace.log" 2>&1
kubectl get namespace kubeop >/dev/null 2>&1 || kubectl create namespace kubeop >"${ARTIFACTS}/namespace-kubeop.log" 2>&1

echo "[e2e] installing operator manifests"
kubectl apply -f "${ROOT_DIR}/deploy/k8s/operator" >"${ARTIFACTS}/apply-operator.log" 2>&1

if ! kubectl get deployment/kubeop-operator -n kubeop-system >/dev/null 2>&1; then
  echo "deployment kubeop-operator missing" >>"${ARTIFACTS}/wait-operator.log"
else
  kubectl get deployment/kubeop-operator -n kubeop-system -o yaml >"${ARTIFACTS}/wait-operator.log" 2>&1
fi

echo "[e2e] seeding sample resources"
kubectl apply -f "${ROOT_DIR}/deploy/k8s/samples/tenant.yaml" >"${ARTIFACTS}/sample-tenant.log" 2>&1
kubectl get tenant demo-tenant -o yaml >"${ARTIFACTS}/sample-tenant-state.yaml" 2>&1 || true

kubectl apply -f "${ROOT_DIR}/deploy/k8s/samples/project.yaml" >"${ARTIFACTS}/sample-project.log" 2>&1
kubectl get project demo-project -o yaml >"${ARTIFACTS}/sample-project-state.yaml" 2>&1 || true

kubectl apply -f "${ROOT_DIR}/deploy/k8s/samples/app.yaml" -n kubeop >"${ARTIFACTS}/sample-app.log" 2>&1
kubectl get app demo-app -n kubeop -o yaml >"${ARTIFACTS}/sample-app-state.yaml" 2>&1 || true

echo "[e2e] running conformance suite ${SUITE}"
if command -v gotestsum >/dev/null 2>&1; then
  GOFLAGS="${GOFLAGS:-}" gotestsum --format testname --junitfile "${ARTIFACTS}/junit.xml" --jsonfile "${ARTIFACTS}/go-test.json" -- -tags=e2e -run "${SUITE}" -count=1 -timeout "${TIMEOUT}s" -covermode=atomic -coverprofile="${ARTIFACTS}/coverage.cov" ./hack/e2e/smoke
else
  GOFLAGS="${GOFLAGS:-}" go test ./hack/e2e/smoke -tags=e2e -run "${SUITE}" -count=1 -timeout "${TIMEOUT}s" -covermode=atomic -coverprofile="${ARTIFACTS}/coverage.cov" -json | tee "${ARTIFACTS}/go-test.json"
fi
