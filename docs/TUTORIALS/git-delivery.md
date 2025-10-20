# Deploy applications from Git repositories

This tutorial walks through deploying an application sourced from a Git repository using kubeOP's REST API. It covers both raw
YAML and Kustomize overlays, highlights commit tracking in release history, and demonstrates how to validate the payload before
deploying it to a tenant project.

## Prerequisites

- kubeOP running via the [Docker Compose quickstart](../getting-started.md#docker-compose) with the API available at
  `http://localhost:8080`.
- A tenant project created and an admin token exported as `AUTH_H` (see the [README quickstart](../../README.md#quickstart-docker-compose)).
- `git`, `jq`, and `curl` installed locally.
- Optional: a Git credential stored via `/v1/credentials/git` when cloning private repositories.

> **Local file repositories**
>
> For smoke tests that clone `file://` repositories (for example, during CI or when using a temporary Git repo), set
> `ALLOW_GIT_FILE_PROTOCOL=true` in `.env`. Keep the flag disabled in shared or production environments.

## 1. Seed a sample Git repository

Create a minimal repository containing both raw manifests and a Kustomize overlay so you can exercise each delivery mode.

```bash
mkdir -p /tmp/kubeop-git-demo/base
cat <<'YAML' > /tmp/kubeop-git-demo/base/deployment.yaml
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

cat <<'YAML' > /tmp/kubeop-git-demo/base/service.yaml
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

cat <<'YAML' > /tmp/kubeop-git-demo/kustomization.yaml
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization
resources:
  - base/deployment.yaml
  - base/service.yaml
patchesStrategicMerge:
  - overlays/prod/replicas.yaml
YAML

mkdir -p /tmp/kubeop-git-demo/overlays/prod
cat <<'YAML' > /tmp/kubeop-git-demo/overlays/prod/replicas.yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: web
spec:
  replicas: 2
YAML

git -C /tmp/kubeop-git-demo init
(cd /tmp/kubeop-git-demo && git add . && git commit -m "seed demo app")
```

## 2. Validate the Git payload

Use the `/v1/apps/validate` endpoint to confirm kubeOP can clone the repo, render the manifests, and compute the object summary
without touching the cluster.

```bash
curl -s $AUTH_H -H 'Content-Type: application/json' \
  -d '{
        "projectId": "<project-id>",
        "name": "git-app",
        "git": {
          "url": "file:///tmp/kubeop-git-demo",
          "ref": "refs/heads/master",
          "mode": "kustomize"
        }
      }' \
  http://localhost:8080/v1/apps/validate | jq
```

The response reports `source: "git:kustomize"` and includes the `gitRepo`, `gitCommit`, and `renderedObjects` arrays so you know
which commit will ship and which Kubernetes objects the overlay produces.

## 3. Deploy the application

Send the same payload to the project deployment endpoint. kubeOP clones the repository, renders the overlay, applies the
manifests to the project namespace, and persists the commit hash for audit trail purposes.

```bash
curl -s $AUTH_H -H 'Content-Type: application/json' \
  -d '{
        "name": "git-app",
        "git": {
          "url": "file:///tmp/kubeop-git-demo",
          "ref": "refs/heads/master",
          "mode": "kustomize"
        }
      }' \
  http://localhost:8080/v1/projects/<project-id>/apps | jq
```

For HTTPS or SSH repositories replace `file:///tmp/kubeop-git-demo` with the real clone URL. When the repo requires
authentication, reference the credential you created with `/v1/credentials/git`:

```json
"git": {
  "url": "https://github.com/example/platform-configs.git",
  "ref": "refs/heads/main",
  "path": "apps/web/prod",
  "credentialId": "<git-credential-id>"
}
```

## 4. Inspect the release history

Every Git deployment captures the commit hash and manifest digest. Query the release history to confirm the commit landed as
expected:

```bash
curl -s $AUTH_H "http://localhost:8080/v1/projects/<project-id>/apps/<app-id>/releases?limit=5" | jq '.releases[0] | {commit: .source.git.commit, source: .source.type}'
```

The `commit` field matches the repository revision, enabling reproducible rollbacks and audit reviews.

## 5. Clean up

Remove the example application and delete the temporary Git repository when finished.

```bash
curl -s -X DELETE $AUTH_H http://localhost:8080/v1/projects/<project-id>/apps/<app-id>
rm -rf /tmp/kubeop-git-demo
```

You now have an end-to-end Git delivery pipeline with validation, deployment, and release auditing wired into kubeOP. Extend the
example by pointing `git.url` at your organisation's repositories and storing scoped Git credentials through the credential
vault.
