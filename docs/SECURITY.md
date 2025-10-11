Security

Admin JWT

- All control-plane APIs under `/v1/*` require a Bearer token.
- Tokens must be signed with `HS256` using `ADMIN_JWT_SECRET`.
- Minimal claim check: `{"role":"admin"}` is required. Future phases will introduce per-tenant authN/Z.
- For development, set `DISABLE_AUTH=true` to bypass auth (never in production).

At-Rest Encryption

- Uploaded kubeconfigs are encrypted with AES-256-GCM. Nonce is generated per record, and `nonce||ciphertext` is stored in Postgres.
- Additional data is `"kcfg-v1"` to bind context.
- Encryption key is derived from `KCFG_ENCRYPTION_KEY`. The service accepts Base64 or hex; otherwise a SHA-256 of the raw string is used.

Secrets and Rotation

- Admin JWT secret and encryption key come from environment variables. Rotate by updating env and restarting the service.
- Re-encryption strategy (future): run a background job to decrypt with old key and re-encrypt with the new key. For now, rotation implies re-upload or a custom migration tool.

Transport

- Terminate TLS at an ingress or API gateway in production. The service itself does not handle TLS.

Hardening (Next Phases)

- Tenant-scoped service accounts and per-namespace kubeconfigs.
- RBAC enforcement and request-level authorization policies.
- Structured audit logs, rate limiting, and request signing.

