# Examples

This directory contains minimal examples referenced throughout the documentation.

| File | Purpose |
| --- | --- |
| `docker-compose.yaml` | Docker Compose stack for local evaluation with PostgreSQL and the kubeOP API. |
| `docker-compose.env` | Sample environment overrides (port, secrets, database DSN). Copy to `.env` for quickstart. |
| `kube/kubeop-api.yaml` | Kubernetes manifests (Namespace, Secrets, Deployment, Service) for running the API in a cluster. |
| `curl/register-cluster.sh` | Script that encodes a kubeconfig and registers a cluster with the API. |

Regenerate scripts and manifests alongside documentation updates to keep them working. Each script includes logging and `set -euo
pipefail` to fail fast in automation.
