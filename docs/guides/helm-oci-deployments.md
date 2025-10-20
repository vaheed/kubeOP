# Deploying Helm charts from OCI registries

kubeOP fetches and renders Helm charts directly from OCI registries. This guide walks through validating and deploying a chart
from GHCR while reusing the registry credential vault for private authentication.

## 1. (Optional) Store a registry credential

```bash
curl -s $AUTH_H -H 'Content-Type: application/json' \
  -d '{
        "name": "ghcr-example",
        "registry": "https://ghcr.io",
        "scope": {"type": "project", "id": "<project-id>"},
        "auth": {"type": "basic", "username": "ci", "password": "s3cret"}
      }' \
  http://localhost:8080/v1/credentials/registries | jq
```

Save the returned `id` as `<registry-credential-id>` for the next steps. Skip this section for public registries.

## 2. Dry-run the deployment

```bash
curl -s $AUTH_H -H 'Content-Type: application/json' \
  -d '{
        "projectId": "<project-id>",
        "name": "grafana",
        "helm": {
          "oci": {
            "ref": "oci://ghcr.io/example/charts/grafana:11.0.0",
            "registryCredentialId": "<registry-credential-id>"
          },
          "values": {
            "service": {"type": "ClusterIP"}
          }
        }
      }' \
  http://localhost:8080/v1/apps/validate | jq
```

The validation response returns the generated Kubernetes object summaries, load balancer quota usage, and the OCI reference
(`helmChart`). Fix any reported errors before proceeding.

## 3. Deploy to the project

```bash
curl -s $AUTH_H -H 'Content-Type: application/json' \
  -d '{
        "name": "grafana",
        "helm": {
          "oci": {
            "ref": "oci://ghcr.io/example/charts/grafana:11.0.0",
            "registryCredentialId": "<registry-credential-id>"
          },
          "values": {
            "service": {"type": "ClusterIP"}
          }
        }
      }' \
  http://localhost:8080/v1/projects/<project-id>/apps | jq
```

kubeOP resolves the registry host, performs a credentialed login when `registryCredentialId` is supplied, renders the chart with
Helm, and applies the manifests to the project namespace. Release history now records the OCI reference and Helm values so you
can audit future rollouts.

## 4. Working with on-prem registries

For trusted development registries that only expose HTTP endpoints, set `"insecure": true` inside `helm.oci`. kubeOP keeps the
insecure flag scoped to the single request and continues to validate the resolved IP addresses against RFC1918/loopback
restrictions.

```json
{
  "helm": {
    "oci": {
      "ref": "oci://harbor.example.local/platform/app:1.0.0",
      "registryCredentialId": "<registry-credential-id>",
      "insecure": true
    }
  }
}
```

Only enable this option for environments where TLS termination is handled upstream.

## 5. Cleanup

Delete the application when finished to remove the rendered resources and release history:

```bash
curl -s $AUTH_H -X DELETE http://localhost:8080/v1/projects/<project-id>/apps/<app-id>
```

Optional: delete the registry credential with `DELETE /v1/credentials/registries/<registry-credential-id>` if it is no longer
required.
