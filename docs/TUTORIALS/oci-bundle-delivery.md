# Deploy OCI manifest bundles

This tutorial demonstrates how to package Kubernetes manifests into an OCI artifact, push it to a registry, and deploy the bundle through kubeOP's REST API. Validation echoes the bundle digest so you can verify which artifact will be applied.

## Prerequisites

- kubeOP running via the [Docker Compose quickstart](../getting-started.md#docker-compose) with the API listening on `http://localhost:8080`.
- A tenant project created and an admin token exported as `AUTH_H` (see [Step 7 in the quickstart](../getting-started.md#7-create-a-project-and-deploy-an-app)).
- The [`oras`](https://oras.land/docs/cli/) CLI (v1.1.0+) installed locally for pushing OCI artifacts.
- `jq`, `tar`, and `curl` available on your workstation.

## 1. Create a manifest bundle

Start with a simple Deployment and Service stored in a temporary working directory.

```bash
mkdir -p /tmp/kubeop-oci-bundle
cat <<'YAML' > /tmp/kubeop-oci-bundle/deployment.yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: web
spec:
  selector:
    matchLabels:
      app: web
  template:
    metadata:
      labels:
        app: web
    spec:
      containers:
      - name: web
        image: ghcr.io/example/web:1.2.3
        ports:
        - containerPort: 8080
YAML

cat <<'YAML' > /tmp/kubeop-oci-bundle/service.yaml
apiVersion: v1
kind: Service
metadata:
  name: web
spec:
  selector:
    app: web
  ports:
  - port: 80
    targetPort: 8080
YAML
```

Bundle the manifests into a tar archive. kubeOP enforces an 8 MiB limit and rejects unsafe paths, so keep the archive lean.

```bash
tar -cvf /tmp/kubeop-oci-bundle.tar -C /tmp/kubeop-oci-bundle .
```

## 2. Push the bundle to an OCI registry

This example uses [ttl.sh](https://ttl.sh/) (an anonymous HTTPS registry) for convenience. Adjust the reference if you publish to your own registry.

```bash
BUNDLE_NAME="$(uuidgen | tr 'A-Z' 'a-z')"
BUNDLE_REF="oci://ttl.sh/${BUNDLE_NAME}-web-bundle:1"
oras push "$BUNDLE_REF" /tmp/kubeop-oci-bundle.tar:application/vnd.kubeop.bundle.v1+tar
```

The CLI prints the pushed digest. kubeOP will report the same digest during validation and deploy.

## 3. Dry-run the bundle

Use `/v1/apps/validate` to confirm kubeOP can fetch and unpack the bundle. Replace `<project-id>` with your tenant project identifier.

```bash
curl -s $AUTH_H -H 'Content-Type: application/json' \
  -d '{
        "projectId": "<project-id>",
        "name": "bundle-demo",
        "ociBundle": {
          "ref": "'"$BUNDLE_REF"'"
        }
      }' \
  http://localhost:8080/v1/apps/validate | jq
```

The response includes `source: "ociBundle"`, `ociBundleRef`, and `ociBundleDigest` along with the rendered object summary.

## 4. Deploy the bundle to the project

Send the same payload (without `projectId`) to the project deployment endpoint. kubeOP fetches the artifact, enforces path and size checks, and applies each manifest to the project namespace.

```bash
curl -s $AUTH_H -H 'Content-Type: application/json' \
  -d '{
        "name": "bundle-demo",
        "ociBundle": {
          "ref": "'"$BUNDLE_REF"'"
        }
      }' \
  http://localhost:8080/v1/projects/<project-id>/apps | jq
```

## 5. Inspect release history

Every deploy records the bundle metadata so you can audit which artifact shipped. Use `/v1/projects/<project-id>/apps/<app-id>/releases` to confirm the digest and rendered objects.

```bash
APP_ID=$(curl -s $AUTH_H http://localhost:8080/v1/projects/<project-id>/apps | jq -r '.[] | select(.name=="bundle-demo") | .appId')
curl -s $AUTH_H "http://localhost:8080/v1/projects/<project-id>/apps/${APP_ID}/releases?limit=1" | jq
```

The release entry lists `source: "ociBundle"`, the `ociBundle.ref`, and the stored digest so future deployments can be compared or rolled back confidently.

