# Cluster inventory and health tutorial

This tutorial walks through registering a cluster with metadata, updating
ownership details, and reviewing the health snapshots recorded by kubeOP. The
steps assume you have a running kubeOP control plane, admin credentials, and a
base64-encodable kubeconfig for the target cluster.

## Prerequisites

- kubeOP API reachable at `http://localhost:8080` (adjust as needed).
- `curl`, `jq`, and `base64` installed locally.
- Admin bearer token exported as `TOKEN` and helper header `AUTH_H="-H 'Authorization: Bearer $TOKEN'"`.
- A kubeconfig file saved to `./kubeconfig` for the cluster you want to register.

## 1. Register the cluster with metadata

Encode the kubeconfig and call `/v1/clusters` with ownership metadata. Tags are
deduplicated and lowercased automatically.

```bash
export AUTH_H="-H 'Authorization: Bearer $TOKEN'"
B64=$(base64 -w0 < kubeconfig)
curl -s $AUTH_H -H 'Content-Type: application/json' \
  -d "$(jq -n --arg name 'talos-stage' --arg b64 "$B64" '{name:$name,kubeconfig_b64:$b64,"owner":"platform","environment":"staging","region":"eu-west-1","apiServer":"https://10.0.0.10:6443","tags":["platform","staging"]}')" \
  http://localhost:8080/v1/clusters | jq
```

The response includes the generated cluster ID, metadata, and timestamps:

```json
{
  "id": "f7de1f39-0a78-4a2b-870d-1c6b48b62f6f",
  "name": "talos-stage",
  "owner": "platform",
  "environment": "staging",
  "region": "eu-west-1",
  "tags": ["platform", "staging"],
  "created_at": "2025-10-27T14:05:03.219Z"
}
```

## 2. Update metadata without rotating credentials

If ownership changes, patch the cluster instead of re-registering it. The API
validates and normalises the tags for you.

```bash
curl -s $AUTH_H -X PATCH -H 'Content-Type: application/json' \
  -d '{"owner":"sre","environment":"production","tags":["platform","prod"]}' \
  http://localhost:8080/v1/clusters/f7de1f39-0a78-4a2b-870d-1c6b48b62f6f | jq
```

## 3. List clusters and inspect health snapshots

Query `/v1/clusters` to see the inventory along with the most recent health
status persisted by the scheduler.

```bash
curl -s $AUTH_H http://localhost:8080/v1/clusters | jq '.[0]'
```

To review detailed probe history, call the status endpoint. The newest entries
appear first and include the timestamp, probe stage, and API server version.

```bash
curl -s $AUTH_H 'http://localhost:8080/v1/clusters/f7de1f39-0a78-4a2b-870d-1c6b48b62f6f/status?limit=5' | jq
```

Expect output similar to:

```json
[
  {
    "id": "1af9f0fe-6dc7-4b40-b276-0b7a4b21ff79",
    "clusterId": "f7de1f39-0a78-4a2b-870d-1c6b48b62f6f",
    "healthy": true,
    "message": "connected",
    "apiServerVersion": "v1.30.0",
    "checkedAt": "2025-10-27T14:06:55.410Z",
    "details": {"stage": "listNamespaces"}
  }
]
```

If you need an immediate probe outside the scheduler cadence, call the live
health endpoint:

```bash
curl -s $AUTH_H http://localhost:8080/v1/clusters/f7de1f39-0a78-4a2b-870d-1c6b48b62f6f/health | jq
```

This returns the latest `ClusterHealth` snapshot and persists a new entry in the
status history table for auditing.
