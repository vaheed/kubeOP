---
outline: deep
---

# End-to-End (Kind) Walkthrough

This guide runs the full flow locally with Kind and Docker Compose:

1) Create a Kind cluster

```bash
make kind-up
```

2) Bootstrap kubeOP components into the cluster and wait for readiness

```bash
bash e2e/bootstrap.sh
```

3) Start the Manager API and Postgres locally

```bash
cp env.example .env
docker compose up -d db
KUBEOP_AGGREGATOR=true docker compose up -d manager
curl -sSf localhost:18080/readyz
```

4) Create a tenant and project via API

```bash
TENANT=$(curl -s -X POST localhost:18080/v1/tenants -H 'Content-Type: application/json' -d '{"name":"acme"}' | jq -r .id)
PROJECT=$(curl -s -X POST localhost:18080/v1/projects -H 'Content-Type: application/json' -d '{"tenantID":"'"$TENANT"'","name":"web"}' | jq -r .id)
```

5) Apply CRDs for Tenant, Project, App, DNSRecord, Certificate and wait for Ready

```bash
cat <<'YAML' | kubectl apply -f -
apiVersion: paas.kubeop.io/v1alpha1
kind: Tenant
metadata:
  name: acme
spec:
  name: acme
---
apiVersion: paas.kubeop.io/v1alpha1
kind: Project
metadata:
  name: web
spec:
  tenantRef: acme
  name: web
---
apiVersion: paas.kubeop.io/v1alpha1
kind: App
metadata:
  name: web
  namespace: kubeop-acme-web
spec:
  type: Image
  image: docker.io/library/nginx:1.25
---
apiVersion: paas.kubeop.io/v1alpha1
kind: DNSRecord
metadata:
  name: web-local
  namespace: kubeop-acme-web
spec:
  host: web.local.dev
  target: 127.0.0.1
---
apiVersion: paas.kubeop.io/v1alpha1
kind: Certificate
metadata:
  name: web-local
  namespace: kubeop-acme-web
spec:
  host: web.local.dev
  dnsRecordRef: web-local
YAML

kubectl -n kubeop-acme-web wait app/web --for=condition=Ready --timeout=90s
kubectl -n kubeop-acme-web get dnsrecords.paas.kubeop.io web-local -o jsonpath='{.status.ready}'
kubectl -n kubeop-acme-web get certificates.paas.kubeop.io web-local -o jsonpath='{.status.ready}'
```

6) Run the E2E test suite

```bash
KUBEOP_E2E=1 go test ./hack/e2e -v -timeout=20m
```

Artifacts (operator/admission logs, cluster resources, compose logs) are saved under `./artifacts`.

