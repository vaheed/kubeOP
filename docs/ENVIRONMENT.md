# Configuration reference

kubeOP is configured entirely through environment variables. This page lists every supported key, the default value, and when to
change it. Use `.env` with Docker Compose or inject variables into the Deployment when running on Kubernetes.

<!-- @include: ./_snippets/env-table.md -->

## Overrides via YAML

Set `CONFIG_FILE` to load a YAML file whose values are merged with defaults before environment variables are applied. Use this for
sensitive settings managed by configuration stores.

```yaml
# config.yaml
adminJWTSecret: "${ADMIN_JWT_SECRET}"
databaseURL: "postgres://user:pass@db:5432/kubeop?sslmode=require"
namespaceQuotaPods: "50"
```

```bash
export CONFIG_FILE=/etc/kubeop/config.yaml
./kubeop-api
```

## Secrets management

- Store JWT secrets and encryption keys in secret stores (Kubernetes `Secret`, Vault, SOPS).
- Rotate `ADMIN_JWT_SECRET` and `KCFG_ENCRYPTION_KEY` periodically. kubeOP logs a warning if these values are blank.
- Avoid enabling `DISABLE_AUTH` outside development environments.
