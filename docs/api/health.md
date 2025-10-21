# Health and metadata

## `GET /healthz`

Returns a static JSON payload when the router is up.

```bash
curl http://localhost:8080/healthz
# {"status":"ok"}
```

- **Responses**
  - `200 OK` – router accepting connections.

## `GET /readyz`

Runs `Service.Health`, which pings PostgreSQL. Logs success/failure via `internal/logging` and records metrics (`metrics.ObserveReadyzFailure`).

```bash
curl http://localhost:8080/readyz
# {"status":"ready"}
```

- **Responses**
  - `200 OK` – database reachable.
  - `503 Service Unavailable` – includes `{"status":"not_ready","error":"..."}`.

## `GET /v1/version`

Exposes build metadata and compatibility ranges from `internal/version`.

```bash
curl http://localhost:8080/v1/version
# {"version":"0.8.28","commit":"<sha>","date":"2025-10-29","compatibility":{"minClientVersion":"0.8.16","minApiVersion":"v1","maxApiVersion":"v1"}}
```

- `compatibility.minClientVersion` — minimum kubeOP CLI/automation version supported by this API build.
- `compatibility.minApiVersion` / `maxApiVersion` — REST API version range currently handled by the server.
- `deprecation.deadline` (optional) — RFC3339 timestamp after which the build is considered unsupported; a warning is logged once exceeded.

## `GET /metrics`

Prometheus metrics (no auth). Scrape over HTTP or HTTPS (fronted by reverse proxy). Includes:

- HTTP request counters and latency histograms.
- Cluster health scheduler timings and failure counts.
