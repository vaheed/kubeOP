# Getting Started

This guide reflects the current Go sources under [`cmd`](https://github.com/vaheed/kubeOP/tree/main/cmd) and [`internal`](https://github.com/vaheed/kubeOP/tree/main/internal). It covers prerequisites, installation paths, upgrades, and removal using the charts and manifests committed in this repository.

## Prerequisites

- Go 1.24 or newer (matches `go.mod`).
- Docker and Docker Compose (used by [`Makefile` Kind/Compose targets](https://github.com/vaheed/kubeOP/blob/main/Makefile#L53-L118)).
- Kind, kubectl, and Helm (used to install CRDs and the operator chart).
- Node.js 18+ (to build the documentation site described in [`docs/package.json`](https://github.com/vaheed/kubeOP/blob/main/docs/package.json)).

Copy `.env` from [`env.example`](https://github.com/vaheed/kubeOP/blob/main/env.example) if you want to run the PostgreSQL-backed manager locally.

## Install on Kind (local development)

```bash
make kind-up            # creates the Kind cluster defined in e2e/kind-config.yaml
make platform-up        # applies deploy/k8s/namespace.yaml, deploy/k8s/crds/
make manager-up         # starts Postgres + manager via docker compose
make operator-up        # installs charts/kubeop-operator into kubeop-system
```

These targets map directly to the shell commands embedded in the [`Makefile`](https://github.com/vaheed/kubeOP/blob/main/Makefile#L72-L111).

After deployment, verify both services and the optional mocks started by `docker-compose.yml`:

```bash
kubectl -n kubeop-system get deploy kubeop-operator -o wide
curl -sf localhost:18080/healthz
curl -sf localhost:28080/healthz   # dns-mock wired by DNS_MOCK_URL
curl -sf localhost:28081/healthz   # acme-mock wired by ACME_MOCK_URL
```

The operator deployment exposes `/metrics` and `/version` on ports `8081`/`8083` as configured in [`cmd/operator/main.go`](https://github.com/vaheed/kubeOP/blob/main/cmd/operator/main.go#L19-L72). The manager exposes `/healthz`, `/readyz`, `/version`, `/metrics`, and the `/v1/...` REST API via [`internal/api/Server.Router`](https://github.com/vaheed/kubeOP/blob/main/internal/api/server.go#L32-L76).

## Install on any cluster via Helm

```bash
helm upgrade --install kubeop-operator charts/kubeop-operator \
  -n kubeop-system --create-namespace \
  --set image.repository=ghcr.io/vaheed/kubeop/operator \
  --set image.tag=$(cat VERSION)
```

The chart renders the deployment, RBAC, service, and ServiceMonitor resources defined under [`charts/kubeop-operator`](https://github.com/vaheed/kubeOP/tree/main/charts/kubeop-operator/templates).

## Upgrades

1. Update the operator image/tag values or bump `VERSION`.
2. Re-run `helm upgrade` with the new values. The deployment uses rolling updates with a single replica; the controller-runtime manager handles leader election when `--leader-elect` is enabled (flag described in [Config](./config.md)).
3. For the manager API, rebuild the binary (`make build`) or restart the Compose service after updating environment variables described in [`internal/config`](https://github.com/vaheed/kubeOP/blob/main/internal/config/config.go#L10-L68).

## Uninstall

```bash
helm uninstall kubeop-operator -n kubeop-system
kubectl delete namespace kubeop-system
```

If you created the Kind cluster only for kubeOP, run `make down` to delete the cluster and stop Compose services.

## Minimal to realistic example

The manifests under [`examples/tenant-project-app`](https://github.com/vaheed/kubeOP/tree/main/examples/tenant-project-app) demonstrate a full reconciliation path:

```bash
kubectl apply -f examples/tenant-project-app/00-tenant.yaml
kubectl apply -f examples/tenant-project-app/01-project.yaml
kubectl apply -f examples/tenant-project-app/02-app-image.yaml
kubectl apply -f examples/tenant-project-app/03-dnsrecord.yaml
kubectl apply -f examples/tenant-project-app/04-certificate.yaml
```

Watch readiness:

```bash
kubectl get tenants.paas.kubeop.io -o wide
kubectl get projects.paas.kubeop.io -o wide
kubectl get apps.paas.kubeop.io -n kubeop-example-example-project --watch
```

Expected steady-state output:

```text
NAME          READY   AGE
example       True    2m
NAME                      NAMESPACE         READY   AGE
example-project          kubeop-example-example-project   True    2m
NAME          TYPE    HOST               READY   REVISION           AGE
example-app  Image   app.example.test   True    20250101-120000    2m
```

The Ready condition, namespace annotations, and revision stamp come from the reconciler logic in [`internal/operator/controllers/controllers.go`](https://github.com/vaheed/kubeOP/blob/main/internal/operator/controllers/controllers.go#L29-L210).

To uninstall the example, delete the manifests in reverse order or run `kubectl delete -f examples/tenant-project-app/`.
