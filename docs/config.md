# Configuration

This reference enumerates all CLI flags and environment variables read by the Go code under `cmd/` and `internal/` packages.

## Operator CLI flags

Defined in [`cmd/operator/main.go`](https://github.com/vaheed/kubeOP/blob/main/cmd/operator/main.go#L19-L47):

| Flag | Default | Description |
|------|---------|-------------|
| `--metrics-bind-address` | `:8081` | Bind address for the controller-runtime metrics server. |
| `--health-probe-bind-address` | `:8082` | Bind address for health and readiness probes. |
| `--leader-elect` | `false` | Enables controller-runtime leader election using the `kubeop-operator-leader` lease. |

Additional environment variables consumed by the operator runtime:

| Variable | Default | Source | Purpose |
|----------|---------|--------|---------|
| `DNS_MOCK_URL` | empty | [`cmd/operator/main.go`](https://github.com/vaheed/kubeOP/blob/main/cmd/operator/main.go#L40-L45) | Optional base URL for posting DNS record events during reconciliation. |
| `ACME_MOCK_URL` | empty | [`cmd/operator/main.go`](https://github.com/vaheed/kubeOP/blob/main/cmd/operator/main.go#L41-L46) | Optional base URL for posting certificate events. |
| `KUBEOP_RECONCILE_SPIN_MS` | empty | [`internal/operator/controllers/controllers.go`](https://github.com/vaheed/kubeOP/blob/main/internal/operator/controllers/controllers.go#L169-L177) | When set to an integer >0, each App reconciliation spins the CPU for that many milliseconds to simulate load. |

## Manager environment variables

Parsed in [`internal/config.Parse`](https://github.com/vaheed/kubeOP/blob/main/internal/config/config.go#L18-L52) and [`cmd/manager/main.go`](https://github.com/vaheed/kubeOP/blob/main/cmd/manager/main.go#L18-L63):

| Variable | Default | Notes |
|----------|---------|-------|
| `KUBEOP_DB_URL` | _required_ | PostgreSQL DSN; must be set unless tests skip or dev insecure mode handles it. |
| `KUBEOP_REQUIRE_AUTH` | `false` | Enables JWT validation when `true`. |
| `KUBEOP_HTTP_ADDR` | `:8080` | HTTP listen address for the manager. |
| `KUBEOP_DEV_INSECURE` | `false` | When `true`, allows missing JWT/KMS keys and generates a random KMS key. |
| `KUBEOP_DB_MAX_OPEN` | `10` | Max open connections on the DB pool. |
| `KUBEOP_DB_MAX_IDLE` | `5` | Max idle connections on the DB pool. |
| `KUBEOP_DB_CONN_MAX_LIFETIME` | `1800` | Seconds before DB connections are recycled. |
| `KUBEOP_DB_TIMEOUT_MS` | `2000` | Used for DB ping and invoice generation timeouts. |
| `KUBEOP_JWT_SIGNING_KEY` | none | Base64-encoded HMAC secret; required when auth is enabled. |
| `KUBEOP_KMS_MASTER_KEY` | none | Base64-encoded envelope key; required unless `KUBEOP_DEV_INSECURE=true`. |
| `KUBEOP_AGGREGATOR` | `false` | When set to `true`, [`cmd/manager/main.go`](https://github.com/vaheed/kubeOP/blob/main/cmd/manager/main.go#L48-L68) runs `usage.Aggregator` hourly. |
| `LOG_LEVEL` | `info` | Parsed by [`internal/logging.New`](https://github.com/vaheed/kubeOP/blob/main/internal/logging/log.go#L8-L18) to switch between `debug`, `info`, and `warn`. |

Webhook and billing configuration from [`internal/api/server.go`](https://github.com/vaheed/kubeOP/blob/main/internal/api/server.go):

| Variable | Default | Reference | Purpose |
|----------|---------|-----------|---------|
| `KUBEOP_HOOK_URL` | empty | [`Server.New`](https://github.com/vaheed/kubeOP/blob/main/internal/api/server.go#L27-L36) | Optional webhook endpoint for emitting lifecycle events. |
| `KUBEOP_HOOK_SECRET` | empty | [`Server.New`](https://github.com/vaheed/kubeOP/blob/main/internal/api/server.go#L27-L36) | Shared secret for `X-KubeOP-Signature` HMAC. |
| `KUBEOP_RATE_CPU_MILLI` | `0.000001` | [`invoice` handler](https://github.com/vaheed/kubeOP/blob/main/internal/api/server.go#L406-L417) | Default price per milliCPU-hour when no tenant-specific rate is stored. |
| `KUBEOP_RATE_MEM_MIB` | `0.0000002` | [`invoice` handler](https://github.com/vaheed/kubeOP/blob/main/internal/api/server.go#L406-L417) | Default price per MiB-hour when no tenant override is stored. |

## Auxiliary service environment variables

Environment variables for the binaries built from `cmd/admission`, `cmd/delivery`, `cmd/meter`, and `cmd/healthcheck`:

| Variable | Default | Source | Purpose |
|----------|---------|--------|---------|
| `ADMISSION_HTTP_ADDR` | `:8090` | [`cmd/admission/main.go`](https://github.com/vaheed/kubeOP/blob/main/cmd/admission/main.go#L13-L30) | HTTP bind address for the admission facade.
| `DELIVERY_HTTP_ADDR` | `:8091` | [`cmd/delivery/main.go`](https://github.com/vaheed/kubeOP/blob/main/cmd/delivery/main.go#L13-L30) | HTTP bind address for the delivery facade.
| `METER_HTTP_ADDR` | `:8092` | [`cmd/meter/main.go`](https://github.com/vaheed/kubeOP/blob/main/cmd/meter/main.go#L13-L30) | HTTP bind address for the metering facade.
| `HEALTH_URL` | `http://localhost:8080/readyz` | [`cmd/healthcheck/main.go`](https://github.com/vaheed/kubeOP/blob/main/cmd/healthcheck/main.go#L9-L24) | Target URL the `/hc` binary probes; exit code indicates readiness. |

## Documentation build commands

The documentation site uses the scripts in [`docs/package.json`](https://github.com/vaheed/kubeOP/blob/main/docs/package.json):

```bash
cd docs
npm install
npm run docs:build
```

See [DOCS.md](https://github.com/vaheed/kubeOP/blob/main/DOCS.md) for more details.
