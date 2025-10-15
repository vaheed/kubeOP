# Watcher ingest (planned)

`POST /v1/events/ingest` is the planned endpoint for watcher event batches. kubeOP v0.9.2 does not route this path yet, but the contract is defined by the watcher sink (`internal/sink`).

## Expected request

- **Method**: `POST`
- **URL**: `https://<public-url>/v1/events/ingest`
- **Headers**:
  - `Authorization: Bearer <watcher-token>` – JWT minted via `service.GenerateWatcherToken` (claims include cluster ID and expiry).
  - `Content-Type: application/json` (optionally `application/json+gzip` when compressed).
- **Body**: JSON array of events. Each event has the shape:

```json
{
  "cluster_id": "cluster-uuid",
  "event_type": "Added",
  "kind": "Pod",
  "namespace": "user-123",
  "name": "web-0",
  "labels": {
    "kubeop.project-id": "project-uuid",
    "kubeop.app-id": "app-uuid"
  },
  "summary": "Pod web-0 added",
  "dedup_key": "uid#resourceVersion"
}
```

Events are deduplicated client-side. The sink drops entries without a `dedup_key` or missing required labels.

## Planned responses

| Status | Meaning |
| --- | --- |
| `202 Accepted` | Batch accepted for processing. |
| `400 Bad Request` | Invalid payload (non-array, missing fields). |
| `401 Unauthorized` | Missing or invalid JWT. |
| `429 Too Many Requests` | Backpressure signal; watcher retries with exponential backoff. |

Until the endpoint is implemented, watchers log batches locally and retry. Operators can validate configuration by monitoring watcher metrics (`kubeop_watcher_queue_depth`, `kubeop_watcher_events_dropped_total`) and kubeOP startup logs.
