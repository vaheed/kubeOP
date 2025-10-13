> **What this page explains**: kubeOP REST endpoints and how to call them.
> **Who it's for**: API consumers building automation or tooling.
> **Why it matters**: Provides tested payloads and client examples you can drop into CI.

# API guide

kubeOP exposes a JSON REST API under `/v1`. All admin calls require a Bearer token signed with `ADMIN_JWT_SECRET` and the claim `{ "role": "admin" }` unless `DISABLE_AUTH=true`.

## Base endpoints

| Endpoint | Method | Description |
| --- | --- | --- |
| `/healthz` | GET | Liveness probe. |
| `/readyz` | GET | Dependency readiness. |
| `/v1/version` | GET | Build metadata. |
| `/v1/clusters` | POST | Register cluster using `kubeconfig_b64`. |
| `/v1/users/bootstrap` | POST | Provision user namespace, quotas, kubeconfig. |
| `/v1/apps` | POST | Deploy an application (image, manifest, or Helm). |

## Register a cluster

```bash
B64=$(base64 -w0 < ~/.kube/config)
curl -s -X POST "http://localhost:8080/v1/clusters" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d @<(cat <<JSON
{
  "name": "dev-cluster",
  "kubeconfig_b64": "$B64"
}
JSON
)
```

## Bootstrap a tenant

```bash
curl -s -X POST "http://localhost:8080/v1/users/bootstrap" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "Alice",
    "email": "alice@example.com",
    "clusterId": "$(uuidgen)"
  }'
```

The response includes the namespace, quotas, and a base64 kubeconfig for the tenant.

## Deploy an application

```bash
curl -s -X POST "http://localhost:8080/v1/apps" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "projectId": "84d4b1b4-7c9b-4f23-8b2f-5e6ab91a4a1a",
    "name": "web",
    "deployment": {
      "type": "helm",
      "chart": {
        "repo": "https://charts.bitnami.com/bitnami",
        "name": "nginx",
        "version": "15.7.0"
      }
    }
  }'
```

## Stream logs and events

Use the `/v1/apps/{id}/logs` endpoint to open a SSE stream. Filter with query parameters such as `revision` or `container`.

```go
req, _ := http.NewRequest("GET", fmt.Sprintf("%s/v1/apps/%s/logs", baseURL, appID), nil)
req.Header.Set("Authorization", "Bearer "+token)
resp, err := http.DefaultClient.Do(req)
if err != nil {
    log.Fatalf("stream logs: %v", err)
}
defer resp.Body.Close()
scanner := bufio.NewScanner(resp.Body)
for scanner.Scan() {
    fmt.Println(scanner.Text())
}
```

