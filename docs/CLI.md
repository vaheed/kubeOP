# CLI usage

kubeOP ships a single binary (`kubeop`) that embeds the HTTP API, scheduler, and configuration loader. You can build it from
source or download images that wrap the same binary.

## Build the binary

```bash
make build
./bin/kubeop --help  # prints usage
```

The build injects version metadata from `internal/version/version.go`. Use the `VERSION`, `COMMIT`, and `DATE` variables to embed
release information:

```bash
make build VERSION=0.14.1 COMMIT=$(git rev-parse --short HEAD) DATE=$(date -u +%Y-%m-%dT%H:%M:%SZ)
```

## Run locally

```bash
KCFG_ENCRYPTION_KEY=dev-not-secure-key \
ADMIN_JWT_SECRET=dev-admin-secret-change-me \
DATABASE_URL=postgres://postgres:postgres@localhost:5432/kubeop?sslmode=disable \
go run ./cmd/api
```

The binary logs configuration, build metadata, and scheduler startup messages. Use `CTRL+C` to shut down gracefully.

## Command-line flags

The binary exposes standard Go `flag` help via `--help`. All configuration lives in environment variables or the optional YAML file
pointed to by `CONFIG_FILE`.

```
Usage of kubeop:
  (environment variables configure behaviour; no command-line flags are required)
```

## Operational scripts

| Script | Location | Purpose |
| --- | --- | --- |
| `docs/examples/curl/register-cluster.sh` | Repository root | Helper to register a cluster using an exported JWT. |
| `make build` | Makefile | Build the `kubeop` binary with version metadata. |
| `make run` | Makefile | Run the API with your current environment variables. |
| `make test` | Makefile | Execute unit tests (`go test ./...`). |
| `make tidy` | Makefile | Run `go mod tidy` to keep dependencies clean. |

## Container images

The Dockerfile at the repository root builds the API binary. Use the provided Docker Compose sample for local evaluation or build
an image:

```bash
docker build -t kubeop-api:dev .
docker run --rm -p 8080:8080 \
  -e ADMIN_JWT_SECRET=dev-admin-secret-change-me \
  -e KCFG_ENCRYPTION_KEY=dev-not-secure-key \
  -e DATABASE_URL=postgres://postgres:postgres@host.docker.internal:5432/kubeop?sslmode=disable \
  kubeop-api:dev
```

## Useful endpoints

After starting the binary or container, verify:

```bash
curl http://localhost:8080/healthz
curl http://localhost:8080/v1/version | jq
```

Use the [Quickstart](QUICKSTART.md) and [API reference](API.md) for end-to-end workflows.
