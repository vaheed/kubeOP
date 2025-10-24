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
| `go build ./kubeop-operator/cmd/bootstrap` | Repository root | Produce the `kubeop-bootstrap` helper for installing CRDs, RBAC, and defaults. |

## kubeop-bootstrap helper

`kubeop-bootstrap` uses the kubeconfig pointed to by `--kubeconfig` (or the defaults from `$KUBECONFIG`) and performs server-side apply for kubeOP platform resources. Every mutating command requires `--yes`; omit it to halt with an explicit error. Events stream to stderr as CloudEvents JSON payloads, while human-readable output prints to stdout.

```bash
go build -o bin/kubeop-bootstrap ./kubeop-operator/cmd/bootstrap

# Install CRDs, RBAC, and webhooks
bin/kubeop-bootstrap init --yes

# Apply default issuers, runtime classes, billing plans, and network policies
bin/kubeop-bootstrap defaults --yes

# Manage tenants and projects
bin/kubeop-bootstrap tenant create --name acme --billing-account BA-001 --yes
bin/kubeop-bootstrap project create --name dev --namespace dev-acme --tenant acme --purpose "Development env" --yes

# Attach domains and registry credentials (YAML output)
bin/kubeop-bootstrap domain attach --name acme-main --fqdn apps.acme.test --tenant acme --dns-provider external-dns --certificate-policy letsencrypt-prod --output yaml --yes
bin/kubeop-bootstrap registry add --name acme-ecr --tenant acme --secret aws-ecr --type ecr --yes
```

> `init` always installs the bundled CRDs before reconciling RBAC or webhooks. `project create` defaults `--environment` to `dev` when unspecified; pass `--environment stage|prod` to target other stages.

Operator artefacts can be regenerated or applied via the dedicated Makefile targets:

```bash
make -C kubeop-operator crds
make -C kubeop-operator validate   # requires kubectl + kubeconform on PATH
make -C kubeop-operator install
make -C kubeop-operator uninstall
```

See [docs/CRDs.md](CRDs.md) for the full schema reference when preparing manifests for GitOps or CI automation.

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
