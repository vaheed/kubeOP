# kubeOP

kubeOP packages a PostgreSQL-backed control plane and a controller-runtime operator for managing tenants, projects, and applications on Kubernetes. Behavior in this README is derived from the Go sources under [`cmd/`](https://github.com/vaheed/kubeOP/tree/main/cmd) and [`internal/`](https://github.com/vaheed/kubeOP/tree/main/internal).

## Features

- **Manager API** (`cmd/manager`) exposes `/healthz`, `/readyz`, `/metrics`, and REST endpoints under `/v1/...` implemented in [`internal/api`](https://github.com/vaheed/kubeOP/tree/main/internal/api).
- **Operator** (`cmd/operator`) reconciles the CRDs defined in [`deploy/k8s/crds`](https://github.com/vaheed/kubeOP/tree/main/deploy/k8s/crds) using the controllers in [`internal/operator/controllers`](https://github.com/vaheed/kubeOP/blob/main/internal/operator/controllers/controllers.go#L29-L269).
- **Auxiliary HTTP services** (`cmd/admission`, `cmd/delivery`, `cmd/meter`) share the same `/healthz`, `/readyz`, `/version`, and `/metrics` surface and bind to `:8090`, `:8091`, and `:8092` by default as shown in [`cmd/admission/main.go`](https://github.com/vaheed/kubeOP/blob/main/cmd/admission/main.go#L13-L31), [`cmd/delivery/main.go`](https://github.com/vaheed/kubeOP/blob/main/cmd/delivery/main.go#L13-L31), and [`cmd/meter/main.go`](https://github.com/vaheed/kubeOP/blob/main/cmd/meter/main.go#L13-L31).
- **Mock integrations** (`cmd/dnsmock`, `cmd/acmemock`) return deterministic payloads for DNS and certificate reconciliations, letting the operator post to `/v1/dnsrecords` and `/v1/certificates` when `DNS_MOCK_URL` and `ACME_MOCK_URL` are set ([`cmd/dnsmock/main.go`](https://github.com/vaheed/kubeOP/blob/main/cmd/dnsmock/main.go#L14-L25), [`cmd/acmemock/main.go`](https://github.com/vaheed/kubeOP/blob/main/cmd/acmemock/main.go#L15-L28)).
- **Usage aggregation** via [`internal/usage.Aggregator`](https://github.com/vaheed/kubeOP/blob/main/internal/usage/aggregator.go#L9-L33) for hourly billing metrics.

## Quickstart

### Local Kind + Compose

```bash
make kind-up            # create Kind cluster defined in e2e/kind-config.yaml
make platform-up        # apply namespace + CRDs
make manager-up         # start Postgres + manager (docker compose)
make operator-up        # install charts/kubeop-operator
```

Verify the deployment, API health, and mock integrations:

```bash
kubectl -n kubeop-system get deploy kubeop-operator
curl -sf localhost:18080/healthz
curl -sf localhost:28080/healthz   # dns-mock
curl -sf localhost:28081/healthz   # acme-mock
```

### Helm install on any cluster

```bash
helm upgrade --install kubeop-operator charts/kubeop-operator \
  -n kubeop-system --create-namespace \
  --set image.repository=ghcr.io/vaheed/kubeop/operator \
  --set image.tag=$(cat VERSION)
```

The chart renders the RBAC, deployment, service, and monitoring objects under [`charts/kubeop-operator/templates`](https://github.com/vaheed/kubeOP/tree/main/charts/kubeop-operator/templates). Set `DNS_MOCK_URL` or `ACME_MOCK_URL` in the operator environment to forward certificate/DNS calls to the mock servers ([`cmd/operator/main.go`](https://github.com/vaheed/kubeOP/blob/main/cmd/operator/main.go#L44-L52)).

## Minimal example

Apply the manifests in [`examples/tenant-project-app`](https://github.com/vaheed/kubeOP/tree/main/examples/tenant-project-app):

```bash
kubectl apply -f examples/tenant-project-app/
```

Watch reconciliation:

```bash
kubectl get tenants.paas.kubeop.io -o wide
kubectl get projects.paas.kubeop.io -o wide
kubectl get apps.paas.kubeop.io -n kubeop-example-example-project --watch
```

The controllers set Ready conditions and deployment revisions as described in [`internal/operator/controllers/controllers.go`](https://github.com/vaheed/kubeOP/blob/main/internal/operator/controllers/controllers.go#L29-L210).

## Documentation

All documentation is generated from the current code:

- [Getting Started](docs/getting-started.md)
- [Architecture](docs/architecture.md)
- [CRDs](docs/crds.md)
- [Controllers](docs/controllers.md)
- [Configuration](docs/config.md)
- [Manager API](docs/api.md)
- [Operations](docs/operations.md)
- [Security](docs/security.md)
- [Troubleshooting](docs/troubleshooting.md)
- [Contributing](docs/contributing.md)

See [DOCS.md](DOCS.md) for instructions on building the VitePress site.
