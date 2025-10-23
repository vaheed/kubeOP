# CLI usage

kubeOP ships a single binary (`kubeop-api`) that runs the control plane HTTP server. Use the commands below to build, run, and
interact with the API from scripts.

## Building the binary

```bash
make build
./bin/kubeop-api
```

The build embeds version metadata from `internal/version` using `-ldflags`. Set `VERSION`, `COMMIT`, and `DATE` to override the
defaults.

```bash
make build VERSION=0.11.3 COMMIT=$(git rev-parse --short HEAD)
```

Alternatively, run the API directly:

```bash
go run ./cmd/api
```

## Runtime flags

The binary does not expose CLI flags. Configure behaviour through the environment variables listed in
[`ENVIRONMENT.md`](ENVIRONMENT.md). Key variables include `PORT`, `DATABASE_URL`, `ADMIN_JWT_SECRET`, and `KCFG_ENCRYPTION_KEY`.

## Useful helper commands

| Command | Description |
| --- | --- |
| `make run` | Start the API with live reloading through `go run`. |
| `make test` | Run unit tests. |
| `go test -count=1 ./testcase` | Execute integration-style tests. |
| `docker compose up -d --build` | Launch the API, PostgreSQL, and supporting services locally. |

## Authenticating curl requests

Reuse the snippet below for all API calls:

<!-- @include: ./_snippets/curl-headers.md -->

Example: list projects with pagination

```bash
curl -s ${KUBEOP_AUTH_HEADER} \
  'http://localhost:8080/v1/projects?limit=10&cursor=' | jq '.projects[] | {id,name,namespace}'
```

## Inspecting logs

- Application logs stream to STDOUT/STDERR. Use `docker compose logs -f api` or systemd journal entries depending on your runtime.
- Per-project logs live under `${LOGS_ROOT}/projects/<project-id>/`. Tail them with `tail -f` or expose via `/v1/projects/{id}/logs`.

## Operator utilities

The controller manager in `kubeop-operator/` includes its own `make` targets:

```bash
pushd kubeop-operator
make test
make build
./bin/manager --help
popd
```

Refer to [`docs/OPERATIONS.md`](OPERATIONS.md) for production-grade automation tips.
