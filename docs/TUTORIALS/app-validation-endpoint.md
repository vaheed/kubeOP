# Tutorial: Dry-run an app with `/v1/apps/validate`

This scenario walks from a fresh clone to validating an application spec without touching the cluster.

## 1. Clone and boot kubeOP

```bash
git clone https://github.com/vaheed/kubeOP.git
cd kubeOP
cp .env.example .env
mkdir -p logs
docker compose up -d --build
```

Wait until `curl http://localhost:8080/readyz` returns `{"status":"ready"}`.

## 2. Bootstrap a user and project

```bash
export TOKEN="$(openssl rand -hex 32)"
export AUTH_H="-H 'Authorization: Bearer $TOKEN'"
export CLUSTER_ID="$(curl -s $AUTH_H -H 'Content-Type: application/json' \
  -d '{"name":"demo","kubeconfig_b64":"'$(base64 -w0 /path/to/admin.kubeconfig)'"}' \
  http://localhost:8080/v1/clusters | jq -r '.id')"
PROJECT_JSON=$(curl -s $AUTH_H -H 'Content-Type: application/json' \
  -d '{"userEmail":"demo@example.com","userName":"Demo","clusterId":"'"$CLUSTER_ID"'","name":"Demo Project"}' \
  http://localhost:8080/v1/projects)
export PROJECT_ID="$(echo "$PROJECT_JSON" | jq -r '.project.id')"
```

## 3. Validate an app spec

```bash
curl -s $AUTH_H -H 'Content-Type: application/json' \
  -d '{"projectId":"'"$PROJECT_ID"'","name":"web","image":"nginx:1","ports":[{"containerPort":80,"servicePort":80,"serviceType":"ClusterIP"}]}' \
  http://localhost:8080/v1/apps/validate | jq
```

The response includes the generated Kubernetes resource name (`kubeName`), replica count, and a `renderedObjects[]` summary.

## 4. Deploy once satisfied

```bash
curl -s $AUTH_H -H 'Content-Type: application/json' \
  -d '{"name":"web","image":"nginx:1","ports":[{"containerPort":80,"servicePort":80,"serviceType":"ClusterIP"}]}' \
  http://localhost:8080/v1/projects/$PROJECT_ID/apps | jq
```

You now have an app deployed with the same spec previously validated. Re-run `/v1/apps/validate` whenever you tweak manifests, Helm charts, or resource overrides to catch errors before deployment.
