# Watcher ingest

`POST /v1/events/ingest` accepts normalised Kubernetes events from the
out-of-cluster watcher. Enable the bridge by setting
`K8S_EVENTS_BRIDGE=true` (and typically `EVENTS_DB_ENABLED=true`) on the
control plane. When disabled, kubeOP still responds with `202 Accepted`
but discards the payload so watchers do not aggressively retry while the
bridge is offline.

## Request

- **Method**: `POST`
- **URL**: `https://<public-url>/v1/events/ingest`
- **Headers**:
  - `Authorization: Bearer <watcher-token>` – short-lived JWT obtained via
    `/v1/watchers/register` or `/v1/watchers/refresh` (claims include
    `cluster_id`, `watcher_id`, and expiry). Legacy credentials that only
    embed the cluster UUID in the subject or omit the `cluster_id`
    entirely continue to work; the API loads the watcher by cluster and
    backfills the missing claims before processing the batch.
  - `Content-Type: application/json` (payloads larger than 8 KiB are
    gzip-compressed and sent with `Content-Encoding: gzip`).
- **Body**: JSON array of events produced by the watcher sink. Each
  entry looks like:

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

The watcher drops entries missing a deduplication key or the required
labels. kubeOP accepts both the dashed (`kubeop.project-id`) and historic
(`kubeop.project.id`) label variants when correlating projects and apps.

## Responses

| Status | Meaning |
| --- | --- |
| `202 Accepted` | Batch processed. The body includes `clusterId`, `total`, `accepted`, and `dropped` counters. |
| `400 Bad Request` | Invalid payload (e.g. malformed JSON, mismatched cluster identifiers). |
| `401 Unauthorized` | Missing or invalid JWT / claims. |

The endpoint never issues `429` backpressure today. Watchers should rely
on exponential backoff for transient network failures, and kubeOP emits
structured logs (`watcher_events_ingested`, `watcher_event_append_failed`)
so operators can trace drops alongside watcher metrics
(`kubeop_watcher_events_dropped_total`).
