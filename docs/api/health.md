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

Exposes build metadata from `internal/version`.

```bash
curl http://localhost:8080/v1/version
# {"version":"0.9.2","commit":"<sha>","date":"2025-11-22"}
```

## `GET /metrics`

Prometheus metrics (no auth). Scrape over HTTP or HTTPS (fronted by reverse proxy). Includes:

- HTTP request counters and latency histograms.
- Cluster health scheduler timings and failure counts.
