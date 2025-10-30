---
outline: deep
---

# Bootstrap Guide

Follow this checklist to take kubeOP from a fresh cluster to a tenant with production workloads, DNS, TLS, billing, and analytics. Every command is copy-paste ready and mirrors the automated `e2e/run.sh` scenario.

## 1. Provision infrastructure

1. Create a Kind cluster and supporting mocks:
   ```bash
   make kind-up
   make platform-up
   make manager-up
   make operator-up
   ```
2. Confirm the manager API is reachable:
 ```bash
 curl -s http://localhost:18080/healthz
 curl -s http://localhost:18080/version | jq
 ```

When you register a cluster through the manager API, kubeOP applies the embedded manifests to that cluster. The
`kubeop-operator` Deployment is rewritten to use `ghcr.io/vaheed/kubeop/operator:latest` with `imagePullPolicy: Always` (or the
values from `KUBEOP_OPERATOR_IMAGE`/`KUBEOP_OPERATOR_IMAGE_PULL_POLICY`), guaranteeing that every cluster pulls the latest
operator build published to GitHub Container Registry.

## 2. Register the cluster and tenant

```bash
# Replace ./kubeconfig with the admin kubeconfig for your workload cluster.
KUBE=$(kind get kubeconfig --name kubeop)
ADMIN=$(jwt_hs256 admin admin) # helper from e2e/run.sh or generate manually
API=http://localhost:18080

CLUSTER_ID=$(jq -n --arg name dev --arg kc "$KUBE" '{name:$name,kubeconfig:$kc}' \
  | curl -sS -H "Authorization: Bearer $ADMIN" -H 'Content-Type: application/json' \
      -X POST "$API/v1/clusters" -d @- | jq -r '.id')

tenant_payload=$(jq -n --arg name acme --arg cid "$CLUSTER_ID" '{name:$name, cluster_id:$cid, quotas:{cpu:"12",memory:"48Gi"}}')
TENANT_ID=$(echo "$tenant_payload" \
  | curl -sS -H "Authorization: Bearer $ADMIN" -H 'Content-Type: application/json' \
      -X POST "$API/v1/tenants" -d @- | jq -r '.id')
```

## 3. Create project and enforce quotas

```bash
PROJECT_JSON=$(jq -n --arg name web --arg tid "$TENANT_ID" --arg ns kubeop-web \
  '{name:$name, tenant_id:$tid, namespace:$ns, quotas:{cpu:"4",memory:"8Gi"}}' \
  | curl -sS -H "Authorization: Bearer $ADMIN" -H 'Content-Type: application/json' \
      -X POST "$API/v1/projects" -d @-)
PROJECT_ID=$(echo "$PROJECT_JSON" | jq -r '.id')
PROJECT_NS=$(echo "$PROJECT_JSON" | jq -r '.namespace')

# Over-quota project fails fast
jq -n --arg name burst --arg tid "$TENANT_ID" '{name:$name, tenant_id:$tid, quotas:{cpu:"999"}}' \
  | curl -sS -H "Authorization: Bearer $ADMIN" -H 'Content-Type: application/json' \
      -X POST "$API/v1/projects" -d @- -o /dev/null -w '%{http_code}'
```

## 4. Deliver applications (Image, Git, Helm, Raw)

```bash
# Strict image allowlist (ghcr.io,docker.io) is enforced by manager/admission.
APP_ID=$(jq -n --arg name frontend --arg pid "$PROJECT_ID" --arg ns "$PROJECT_NS" \
  '{name:$name, project_id:$pid, namespace:$ns, spec:{image:"ghcr.io/library/nginx:1.25"}}' \
  | curl -sS -H "Authorization: Bearer $ADMIN" -H 'Content-Type: application/json' \
      -X POST "$API/v1/apps" -d @- | jq -r '.id')

# Git delivery example (applies manifest list rendered from Git).
APP_GIT_ID=$(jq -n --arg name frontend-git --arg pid "$PROJECT_ID" --arg ns "$PROJECT_NS" '{
  name:$name,
  project_id:$pid,
  namespace:$ns,
  spec:{
    image:"ghcr.io/library/git-runner:1",
    delivery:{
      kind:"Git",
      git:{manifests:[{apiVersion:"v1",kind:"ConfigMap",metadata:{name:"frontend-git"},data:{source:"git"}}]}
    }
  }
}' | curl -sS -H "Authorization: Bearer $ADMIN" -H 'Content-Type: application/json' \
       -X POST "$API/v1/apps" -d @- | jq -r '.id')

# Helm delivery example (server-side apply of rendered manifests).
APP_HELM_ID=$(jq -n --arg name frontend-helm --arg pid "$PROJECT_ID" --arg ns "$PROJECT_NS" '{
  name:$name,
  project_id:$pid,
  namespace:$ns,
  spec:{
    image:"ghcr.io/library/helm-runner:1",
    delivery:{
      kind:"Helm",
      helm:{manifests:[{apiVersion:"v1",kind:"ConfigMap",metadata:{name:"frontend-helm"},data:{values:"replicaCount: 1"}}]}
    }
  }
}' | curl -sS -H "Authorization: Bearer $ADMIN" -H 'Content-Type: application/json' \
       -X POST "$API/v1/apps" -d @- | jq -r '.id')
```

Negative checks:

```bash
# Wrong namespace is rejected
jq -n --arg name bad --arg pid "$PROJECT_ID" '{name:$name, project_id:$pid, namespace:"wrong"}' \
  | curl -sS -H "Authorization: Bearer $ADMIN" -H 'Content-Type: application/json' \
      -X POST "$API/v1/apps" -d @- -o /dev/null -w '%{http_code}'

# Registry not on allowlist returns 400
jq -n --arg name blocked --arg pid "$PROJECT_ID" '{name:$name, project_id:$pid, spec:{image:"registry.k8s.io/library/nginx:1"}}' \
  | curl -sS -H "Authorization: Bearer $ADMIN" -H 'Content-Type: application/json' \
      -X POST "$API/v1/apps" -d @- -o /dev/null -w '%{http_code}'
```

## 5. Deploy workloads, DNS, and TLS

```bash
kubectl create namespace "$PROJECT_NS" --dry-run=client -o yaml | kubectl apply -f -

kubectl -n "$PROJECT_NS" apply -f - <<'YAML'
apiVersion: apps/v1
kind: Deployment
metadata:
  name: web
spec:
  replicas: 1
  selector:
    matchLabels: { app: web }
  template:
    metadata:
      labels: { app: web }
    spec:
      containers:
        - name: nginx
          image: nginx:1.25-alpine
          ports:
            - containerPort: 80
---
apiVersion: v1
kind: Service
metadata:
  name: web
spec:
  selector: { app: web }
  ports:
    - port: 80
      targetPort: 80
      name: http
YAML

kubectl -n "$PROJECT_NS" apply -f - <<'YAML'
apiVersion: batch/v1
kind: Job
metadata:
  name: curl-web
spec:
  template:
    spec:
      restartPolicy: Never
      containers:
        - name: curl
          image: curlimages/curl:8.9.1
          args: ["-sS", "--fail", "http://web.${PROJECT_NS}.svc.cluster.local"]
YAML

kubectl -n "$PROJECT_NS" wait --for=condition=complete job/curl-web --timeout=120s
kubectl -n "$PROJECT_NS" rollout status deploy/web --timeout=120s
kubectl -n "$PROJECT_NS" rollout history deploy/web
kubectl -n "$PROJECT_NS" set image deploy/web web=nginx:1.26-alpine
kubectl -n "$PROJECT_NS" rollout status deploy/web --timeout=120s
kubectl -n "$PROJECT_NS" rollout undo deploy/web --to-revision=1

# NetworkPolicy baseline + DNS/TLS CRDs
kubectl -n "$PROJECT_NS" get networkpolicy kubeop-egress -o json | jq '.spec'

kubectl apply -f - <<'YAML'
apiVersion: platform.kubeop.io/v1alpha1
kind: DNSRecord
metadata:
  name: web-e2e
spec:
  host: web.e2e.kubeop.test
  target: web.${PROJECT_NS}.svc.cluster.local
---
apiVersion: platform.kubeop.io/v1alpha1
kind: Certificate
metadata:
  name: web-e2e
spec:
  host: web.e2e.kubeop.test
  dnsRecordRef: web-e2e
YAML
kubectl get dnsrecords.platform.kubeop.io/web-e2e
kubectl get certificates.platform.kubeop.io/web-e2e
```

## 6. Usage snapshot → invoice → analytics

```bash
USAGE_FILE=artifacts/usage.json
INVOICE_FILE=artifacts/invoice.json
ANALYTICS_FILE=artifacts/analytics.json
mkdir -p artifacts

curl -sS -H "Authorization: Bearer $ADMIN" "$API/v1/usage/snapshot" \
  | tee "$USAGE_FILE" | jq '.totals'

curl -sS -H "Authorization: Bearer $ADMIN" "$API/v1/invoices/$TENANT_ID" \
  | tee "$INVOICE_FILE" | jq '{tenant_name, subtotal, lines}'

curl -sS -H "Authorization: Bearer $ADMIN" "$API/v1/analytics/summary" \
  | tee "$ANALYTICS_FILE" | jq '{delivery_kinds, registry_hosts}'
```

## 7. Tear-down order

```bash
curl -sS -H "Authorization: Bearer $ADMIN" -X DELETE "$API/v1/apps/$APP_HELM_ID"
curl -sS -H "Authorization: Bearer $ADMIN" -X DELETE "$API/v1/apps/$APP_GIT_ID"
curl -sS -H "Authorization: Bearer $ADMIN" -X DELETE "$API/v1/apps/$APP_ID"
curl -sS -H "Authorization: Bearer $ADMIN" -X DELETE "$API/v1/projects/$PROJECT_ID"
curl -sS -H "Authorization: Bearer $ADMIN" -X DELETE "$API/v1/tenants/$TENANT_ID"
curl -sS -H "Authorization: Bearer $ADMIN" -X DELETE "$API/v1/clusters/$CLUSTER_ID"

make down
```

All steps above are scripted in `e2e/run.sh` and produce artifacts under `./artifacts/` for auditability.
