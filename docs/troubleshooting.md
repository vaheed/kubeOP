# Troubleshooting

- Operator not ready: kubectl -n kubeop-system logs deploy/kubeop-operator
- Admission TLS/CABundle: check ValidatingWebhookConfiguration
- Manager DB: docker compose ps; verify KUBEOP_DB_URL
