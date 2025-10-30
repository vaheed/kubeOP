# Manager HTTP API

Endpoints are served by [`internal/api/Server.Router`](https://github.com/vaheed/kubeOP/blob/main/internal/api/server.go#L32-L76). Authentication is optional depending on `KUBEOP_REQUIRE_AUTH`; when enabled the helper wrappers (`requireRole`, `requireRoleOrTenant`, `requireRoleOrProject`) enforce role- or scope-based access.

| Path | Methods | Auth requirement | Handler reference | Notes |
|------|---------|------------------|-------------------|-------|
| `/healthz` | GET | none | [`handleReady` is not used; this endpoint returns 200 directly](https://github.com/vaheed/kubeOP/blob/main/internal/api/server.go#L37-L39) | Liveness check. |
| `/readyz` | GET | none | [`handleReady`](https://github.com/vaheed/kubeOP/blob/main/internal/api/server.go#L202-L211) | Pings DB with timeout from `KUBEOP_DB_TIMEOUT_MS` and verifies KMS is initialized. |
| `/version` | GET | none | [`handleVersion`](https://github.com/vaheed/kubeOP/blob/main/internal/api/server.go#L167-L201) | Returns `{service, version, gitCommit, buildDate}` using `internal/version`. |
| `/openapi.json` | GET | none | [`handleOpenAPI`](https://github.com/vaheed/kubeOP/blob/main/internal/api/server.go#L213-L218) | Serves the embedded `openapi.json`. |
| `/metrics` | GET | none | [`PromHandler`](https://github.com/vaheed/kubeOP/blob/main/internal/api/metrics.go#L34-L57) via `PromHandler` | Prometheus metrics including counters/summaries from `internal/metrics`. |
| `/v1/tenants` | GET, POST, PUT/PATCH | `admin` for GET/POST/updates when auth enabled | [`tenantsCollection`](https://github.com/vaheed/kubeOP/blob/main/internal/api/server.go#L220-L275) | Lists, creates, or updates tenants through `models.Store`. |
| `/v1/tenants/{id}` | GET, DELETE | admin or tenant scoped | [`tenantsGetDelete`](https://github.com/vaheed/kubeOP/blob/main/internal/api/server.go#L422-L449) | Fetches or deletes tenants. |
| `/v1/projects` | GET, POST, PUT/PATCH | admin or tenant scope for POST/GET | [`projectsCollection`](https://github.com/vaheed/kubeOP/blob/main/internal/api/server.go#L277-L338) | Lists projects filtered by `tenantID`, creates, or updates names. |
| `/v1/projects/{id}` | GET, DELETE | admin, tenant owner, or project scope | [`projectsGetDelete`](https://github.com/vaheed/kubeOP/blob/main/internal/api/server.go#L451-L482) | Reads or deletes a project. |
| `/v1/apps` | GET, POST, PUT/PATCH | admin or project scope | [`appsCollection`](https://github.com/vaheed/kubeOP/blob/main/internal/api/server.go#L340-L399) | Lists apps filtered by `projectID`, creates, or updates apps. |
| `/v1/apps/{id}` | GET, DELETE | admin or project scope | [`appsGetDelete`](https://github.com/vaheed/kubeOP/blob/main/internal/api/server.go#L484-L515) | Reads or deletes an app. |
| `/v1/usage/snapshot` | GET | admin only | [`usageSnapshot`](https://github.com/vaheed/kubeOP/blob/main/internal/api/server.go#L517-L524) | Returns aggregated usage totals. |
| `/v1/usage/ingest` | POST | admin or tenant scope per payload entry | [`usageIngest`](https://github.com/vaheed/kubeOP/blob/main/internal/api/server.go#L548-L563) | Accepts usage lines and stores hourly samples. |
| `/v1/invoices/{tenantID}` | GET | admin or matching tenant | [`invoice`](https://github.com/vaheed/kubeOP/blob/main/internal/api/server.go#L402-L421) | Generates invoice lines and subtotal using rate env vars or tenant overrides. |
| `/v1/kubeconfigs/{name}` | GET | admin only | [`kubeconfigIssue`](https://github.com/vaheed/kubeOP/blob/main/internal/api/server.go#L565-L589) | Returns minimal kubeconfig YAML for the named namespace. |
| `/v1/kubeconfigs/project/{projectID}` | GET | admin or project scope | [`kubeconfigProject`](https://github.com/vaheed/kubeOP/blob/main/internal/api/server.go#L591-L610) | Emits kubeconfig referencing the project namespace recorded in the DB. |
| `/v1/jwt/project` | POST | admin or tenant scope | [`jwtMintProject`](https://github.com/vaheed/kubeOP/blob/main/internal/api/server.go#L612-L623) | Issues short-lived JWT tokens for project access using the configured signing key. |

### Responses and errors

All handlers respond with JSON and use helper wrappers to set `Content-Type: application/json` (`withJSON`). Errors return JSON bodies such as `{"error":"db"}` or `{"error":"forbidden"}` depending on the failure path.

### Rate limiting and metrics

Each DB interaction records latency via `metrics.ObserveDB`, incrementing counters such as `kubeop_business_created_total` for successful creations. Webhook invocations triggered inside `tenantsCreate`, `projectsCreate`, and `appsCreate` are delivered through [`webhook.Client.Send`](https://github.com/vaheed/kubeOP/blob/main/internal/webhook/webhook.go#L18-L52), which retries up to three times with exponential backoff.
