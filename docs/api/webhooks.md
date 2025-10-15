# Git webhook API

kubeOP exposes a generic webhook endpoint for Git providers. When payloads reference repositories tied to applications, kubeOP patches their Deployments to trigger rollouts.

## `POST /v1/webhooks/git`

- **Headers**
  - `X-Hub-Signature-256` – Optional HMAC signature. When present, kubeOP validates it using the app-specific `webhookSecret` or the global `GIT_WEBHOOK_SECRET`.
- **Body**
  - JSON payload. kubeOP expects `repository.full_name` (or `repository.clone_url`) and `ref`.

### Example

```bash
curl -s -X POST \
  -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' \
  -H 'X-Hub-Signature-256: sha256=<signature>' \
  -d '{"ref":"refs/heads/main","repository":{"full_name":"example/app"}}' \
  http://localhost:8080/v1/webhooks/git
```

### Behaviour

1. kubeOP finds apps with matching `repo` fields.
2. If a secret is configured, it verifies `X-Hub-Signature-256` against the payload.
3. For each match, kubeOP loads the project, obtains a Kubernetes client, and patches the Deployment annotation `kubeop.io/redeploy=<timestamp>` to trigger a rollout restart.
4. Errors are logged per app; the endpoint still returns success if at least one app was processed.

### Responses

- `200 OK` – `{ "status": "handled" }`.
- `400 Bad Request` – invalid JSON, missing repository/ref, or signature verification failure.
