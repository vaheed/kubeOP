# kubeOP Operator Guide (Preview)

The `kubeop-operator` module introduces a controller-runtime powered operator that will eventually manage all kubeOP workloads
through Kubernetes CustomResourceDefinitions (CRDs). This preview focuses on the scaffolding delivered during Phase 0 of the
roadmap.

## Project layout

```text
kubeop-operator/
├── api/v1alpha1/       # App CRD type definitions and scheme registration
├── controllers/        # Reconcilers for each CRD (App controller scaffolded)
├── cmd/manager/        # Operator entrypoint and manager wiring
├── config/{crd,rbac,manager}/ # Reserved for manifests as the project matures
├── Makefile            # Convenience targets (test, build, lint, tidy)
└── go.mod              # Standalone Go module
```

## Building and testing

```bash
cd kubeop-operator
make tidy
make test
make build
```

These targets run with `set -euo pipefail` to fail fast on errors. The CI workflow also executes `go fmt`, `go vet`, `go test`,
and builds the manager binary to keep the operator aligned with repository standards.

## Manager behaviour

- **Metrics**: Served on `:8080` by default (`--metrics-bind-address`).
- **Health/Ready probes**: Served on `:8081` (`--health-probe-bind-address`).
- **Leader election**: Disabled by default but can be enabled using `--leader-elect` for HA deployments.
- **Logging**: Uses zap in development mode with explicit UTC timestamps and contextual reconciliation logs.

## Next steps

Future roadmap phases will introduce:

1. Full App reconciliation that renders Deployments, Services, and Ingress resources.
2. Additional CRDs such as `ConfigBundle` and `IngressRule` with dedicated controllers.
3. RBAC, webhook, and configuration manifests under `config/`.
4. API integration that bridges kubeOP REST endpoints with the operator-managed CRDs.

Contributions should include tests in `kubeop-operator/` and documentation updates here to describe new behaviours as they land.
