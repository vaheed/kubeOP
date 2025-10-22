# Deploy an app with curl

This example validates and deploys a container image using the REST API.

```bash
# Reuse the snippet from docs/_snippets/curl-headers.md if you prefer
export KUBEOP_TOKEN="<admin-jwt>"
export KUBEOP_AUTH_HEADER="-H 'Authorization: Bearer ${KUBEOP_TOKEN}'"
PROJECT_ID="<project-id>"
APP_NAME="web"
IMAGE="ghcr.io/example/web:1.2.3"
```

## Validate the spec

```bash
curl -s ${KUBEOP_AUTH_HEADER} -H 'Content-Type: application/json' \
  -d "$(jq -n --arg pid "$PROJECT_ID" --arg name "$APP_NAME" --arg image "$IMAGE" '{projectId:$pid,name:$name,image:$image,replicas:2,ports:[{"containerPort":80,"servicePort":80,"serviceType":"LoadBalancer"}]}')" \
  http://localhost:8080/v1/apps/validate | jq '.summary'
```

## Deploy

```bash
curl -s ${KUBEOP_AUTH_HEADER} -H 'Content-Type: application/json' \
  -d "$(jq -n --arg pid "$PROJECT_ID" --arg name "$APP_NAME" --arg image "$IMAGE" '{projectId:$pid,name:$name,image:$image,replicas:2}')" \
  http://localhost:8080/v1/projects/${PROJECT_ID}/apps | jq
```

## Inspect status

```bash
curl -s ${KUBEOP_AUTH_HEADER} http://localhost:8080/v1/projects/${PROJECT_ID}/apps | jq '.apps[] | {name,status}'
```
