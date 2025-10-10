API Reference (Phase 1)

Auth

- All `/v1/*` endpoints require a Bearer token signed with `ADMIN_JWT_SECRET` and claim `{ "role": "admin" }`.

Health

- GET `/healthz` → 200 `{ "status": "ok" }`
- GET `/readyz` → 200 `{ "status": "ready" }` if DB reachable, else 503 `{ "status": "not_ready" }`

Version

- GET `/v1/version` → 200 `{ "version": "...", "commit": "...", "date": "..." }`

Clusters

- POST `/v1/clusters`
  - Request: `{ "name": "my-cluster", "kubeconfig": "<file contents>" }`
  - Response: `201 { "id": "uuid", "name": "my-cluster", "created_at": "RFC3339" }`
  - Notes: kubeconfig is encrypted and stored at rest. Not returned in responses.

- GET `/v1/clusters`
  - Response: `200 [ { "id": "uuid", "name": "...", "created_at": "..." }, ... ]`

Users

- POST `/v1/users`
  - Request: `{ "name": "Alice", "email": "alice@example.com" }`
  - Response: `201 { "id": "uuid", "name": "Alice", "email": "alice@example.com", "created_at": "..." }`

- GET `/v1/users`
  - Response: `200 [ { "id": "uuid", "name": "...", "email": "...", "created_at": "..." }, ... ]`

- GET `/v1/users/{id}`
  - Response: `200 { "id": "uuid", "name": "...", "email": "...", "created_at": "..." }` or `404` if not found

Error Format

- Errors return a JSON body like `{ "error": "message" }` with the appropriate HTTP status code.

