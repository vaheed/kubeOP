CI Webhooks

Endpoint

- POST `/v1/webhooks/git`
  - Accepts common push event payloads (`repository.full_name` or `repository.clone_url`, and `ref`).
  - Signature verification supports per-app `webhookSecret` (preferred) and a global `GIT_WEBHOOK_SECRET` (fallback). Header: `X-Hub-Signature-256: sha256=<hex>`.
  - On repo match (and valid signature if configured), KubeOP triggers a redeploy by patching a Deployment annotation.

Setup (GitHub)

- URL: `https://<kubeop>/v1/webhooks/git`
- Content type: `application/json`
- Secret: set to the server’s `GIT_WEBHOOK_SECRET` (optional but recommended)
- Events: `Push` (and others as desired)

Payloads

- KubeOP looks at `repository.full_name` (e.g., `org/repo`) or `repository.clone_url`, and `ref`.
- Associate an app with a repository by setting `repo` (and optional `webhookSecret`) in your deploy request to `/v1/projects/{id}/apps`.
- If both per-app `webhookSecret` and global `GIT_WEBHOOK_SECRET` are present, the per-app secret is used for verification.
