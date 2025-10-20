# Tutorial: Deploy a Helm chart from an OCI registry

This scenario demonstrates using kubeOP to validate and deploy an OCI-backed Helm chart while reusing the registry credential
vault. The commands assume a fresh environment created from `.env.example` and Docker Compose.

## 1. Boot kubeOP and export helpers

```bash
git clone https://github.com/vaheed/kubeOP.git
cd kubeOP
cp .env.example .env
mkdir -p logs
docker compose up -d --build

export TOKEN="$(openssl rand -hex 32)"
export AUTH_H="-H 'Authorization: Bearer '"$TOKEN
```

Wait for the API to become ready:

```bash
curl http://localhost:8080/readyz
```

## 2. Register a demo cluster and project

```bash
export CLUSTER_ID="$(curl -s $AUTH_H -H 'Content-Type: application/json' \
  -d '{"name":"demo","kubeconfig_b64":"'$(base64 -w0 /path/to/admin.kubeconfig)'"}' \
  http://localhost:8080/v1/clusters | jq -r '.id')"
PROJECT_JSON=$(curl -s $AUTH_H -H 'Content-Type: application/json' \
  -d '{"userEmail":"demo@example.com","userName":"Demo","clusterId":"'"$CLUSTER_ID"'","name":"Demo Project"}' \
  http://localhost:8080/v1/projects)
export PROJECT_ID="$(echo "$PROJECT_JSON" | jq -r '.project.id')"
```

## 3. Store a registry credential (optional)

```bash
export REGISTRY_CRED="$(curl -s $AUTH_H -H 'Content-Type: application/json' \
  -d '{
        "name": "ghcr-demo",
        "registry": "https://ghcr.io",
        "scope": {"type": "project", "id": "'"$PROJECT_ID"'"},
        "auth": {"type": "basic", "username": "ci", "password": "s3cret"}
      }' \
  http://localhost:8080/v1/credentials/registries | jq -r '.id')"
```

Skip this step for public charts; `REGISTRY_CRED` is optional in the following payloads.

## 4. Dry-run the OCI chart

```bash
curl -s $AUTH_H -H 'Content-Type: application/json' \
  -d '{
        "projectId": "'"$PROJECT_ID"'",
        "name": "grafana",
        "helm": {
          "oci": {
            "ref": "oci://ghcr.io/example/charts/grafana:11.0.0",
            "registryCredentialId": "'"$REGISTRY_CRED"'"
          },
          "values": {
            "service": {"type": "ClusterIP"}
          }
        }
      }' \
  http://localhost:8080/v1/apps/validate | jq
```

Confirm the response lists `source: "helm"`, the generated Kubernetes objects, and no errors or quota violations.

## 5. Deploy the chart

```bash
APP_JSON=$(curl -s $AUTH_H -H 'Content-Type: application/json' \
  -d '{
        "name": "grafana",
        "helm": {
          "oci": {
            "ref": "oci://ghcr.io/example/charts/grafana:11.0.0",
            "registryCredentialId": "'"$REGISTRY_CRED"'"
          },
          "values": {
            "service": {"type": "ClusterIP"}
          }
        }
      }' \
  http://localhost:8080/v1/projects/$PROJECT_ID/apps)
export APP_ID="$(echo "$APP_JSON" | jq -r '.appId')"
```

kubeOP logs into the registry (when `REGISTRY_CRED` is present), renders the chart, applies the manifests, and records a release
snapshot referencing the OCI `ref` and Helm values.

## 6. Inspect release history

```bash
curl -s $AUTH_H "http://localhost:8080/v1/projects/$PROJECT_ID/apps/$APP_ID/releases?limit=1" | jq
```

The response shows the stored `spec.source.helm`, load balancer usage, rendered object summaries, and any warnings captured during
planning.

## 7. Cleanup

```bash
curl -s $AUTH_H -X DELETE http://localhost:8080/v1/projects/$PROJECT_ID/apps/$APP_ID
```

Optionally delete the registry credential with:

```bash
curl -s $AUTH_H -X DELETE http://localhost:8080/v1/credentials/registries/$REGISTRY_CRED
```
