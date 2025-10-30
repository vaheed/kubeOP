# Contributing

## Repository layout

- Go module root: [`go.mod`](https://github.com/vaheed/kubeOP/blob/main/go.mod) (module `github.com/vaheed/kubeop`).
- Manager binary: [`cmd/manager`](https://github.com/vaheed/kubeOP/tree/main/cmd/manager) with supporting packages under [`internal`](https://github.com/vaheed/kubeOP/tree/main/internal).
- Operator binary: [`cmd/operator`](https://github.com/vaheed/kubeOP/tree/main/cmd/operator) and [`internal/operator`](https://github.com/vaheed/kubeOP/tree/main/internal/operator).
- Helm chart: [`charts/kubeop-operator`](https://github.com/vaheed/kubeOP/tree/main/charts/kubeop-operator).
- Kubernetes manifests: [`deploy/k8s`](https://github.com/vaheed/kubeOP/tree/main/deploy/k8s).
- Documentation: [`docs/`](https://github.com/vaheed/kubeOP/tree/main/docs) (this site) and [`DOCS.md`](https://github.com/vaheed/kubeOP/blob/main/DOCS.md) for build instructions.

## Development environment

- Install Go 1.24+, Docker, Docker Compose, Kind, kubectl, and Helm (see [Getting Started](./getting-started.md)).
- Copy `env.example` to `.env` to supply required environment variables when running the manager locally.
- The manager requires PostgreSQL; the Compose stack in [`docker-compose.yml`](https://github.com/vaheed/kubeOP/blob/main/docker-compose.yml#L22-L36) provisions a compatible database.

## Tests and linting

- Run unit tests and static checks with `make right` (`fmt`, `vet`, `tidy`, and build) and `go test ./...` (`Makefile`).
- End-to-end tests live under [`hack/e2e`](https://github.com/vaheed/kubeOP/tree/main/hack/e2e) and run via `make test-e2e`, bootstrapping a Kind cluster and Helm chart.
- Documentation builds with `npm run docs:build` from the [`docs/package.json`](https://github.com/vaheed/kubeOP/blob/main/docs/package.json) scripts.

## Coding guidelines

- Configuration should flow through [`internal/config.Config`](https://github.com/vaheed/kubeOP/blob/main/internal/config/config.go#L10-L52) and validated via `Config.Validate` before use.
- Emit logs through [`internal/logging.New`](https://github.com/vaheed/kubeOP/blob/main/internal/logging/log.go#L8-L19) or the zap logger in `cmd/operator`.
- When interacting with the database, prefer the methods on [`internal/models.Store`](https://github.com/vaheed/kubeOP/tree/main/internal/models) and record latency with [`metrics.ObserveDB`](https://github.com/vaheed/kubeOP/blob/main/internal/metrics/metrics.go#L27-L39).

## Release process

- Update `VERSION` and append a changelog entry following [Keep a Changelog](https://github.com/vaheed/kubeOP/blob/main/CHANGELOG.md).
- Build multi-arch images using `make image-manager` and `make image-operator`; push via `make push-images` (see [`Makefile`](https://github.com/vaheed/kubeOP/blob/main/Makefile#L32-L71)).
- Package the Helm chart with `make helm-package` to produce artifacts in `dist/charts`.
