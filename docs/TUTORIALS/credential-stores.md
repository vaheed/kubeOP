# Tutorial: Encrypt Git and registry credentials

This tutorial demonstrates how to create, list, retrieve, and delete encrypted
credentials using the new `/v1/credentials/*` APIs. The flow starts from a fresh
control plane and ends with usable credential IDs that delivery pipelines can
reference without embedding secrets in application specs.

## Prerequisites

- kubeOP API running locally (Docker Compose or `go run ./cmd/api`).
- Admin token exported as `AUTH_H` (for example,
  `export AUTH_H="-H 'Authorization: Bearer $TOKEN'"`).
- At least one user and project. The snippets below assume the IDs are available
  in `USER_ID` and `PROJECT_ID` environment variables.

```bash
# Fetch a user ID (replace with your bootstrap flow as needed)
USER_ID=$(curl -s $AUTH_H http://localhost:8080/v1/users | jq -r '.[0].id')
PROJECT_ID=$(curl -s $AUTH_H "http://localhost:8080/v1/projects?limit=1" | jq -r '.[0].id')
```

## Step 1 — Store a Git token for source fetches

```bash
curl -s $AUTH_H -H 'Content-Type: application/json' \
  -d '{
        "name": "git-main",
        "scope": {"type": "user", "id": "<user-id>"},
        "auth": {"type": "token", "token": "ghp_example_personal_token"}
      }' \
  http://localhost:8080/v1/credentials/git | jq
```

The response includes the immutable credential ID:

```json
{
  "id": "4c0dd2e9-5f2d-4b83-a70f-97f47f31d3c2",
  "name": "git-main",
  "scopeType": "USER",
  "scopeId": "0bd7f8a4-8d2f-4a3a-9c81-9e2aa22f51f4",
  "authType": "TOKEN",
  "createdAt": "2025-10-24T13:10:00Z",
  "updatedAt": "2025-10-24T13:10:00Z"
}
```

## Step 2 — Store a registry password scoped to a project

```bash
curl -s $AUTH_H -H 'Content-Type: application/json' \
  -d '{
        "name": "dockerhub",
        "registry": "https://index.docker.io/v1/",
        "scope": {"type": "project", "id": "<project-id>"},
        "auth": {"type": "basic", "username": "robot", "password": "s3cret"}
      }' \
  http://localhost:8080/v1/credentials/registries | jq
```

## Step 3 — List credentials to confirm scope isolation

```bash
# User-scoped credentials
curl -s $AUTH_H "http://localhost:8080/v1/credentials/git?userId=${USER_ID}" | jq

# Project-scoped registry credentials
curl -s $AUTH_H "http://localhost:8080/v1/credentials/registries?projectId=${PROJECT_ID}" | jq
```

Each collection includes metadata only—secrets stay encrypted until explicitly
retrieved.

## Step 4 — Fetch decrypted secrets for automation jobs

Use the credential ID returned in steps 1 or 2 when building deployment jobs:

```bash
GIT_CRED_ID=$(curl -s $AUTH_H "http://localhost:8080/v1/credentials/git?userId=${USER_ID}" | jq -r '.[0].id')
REG_CRED_ID=$(curl -s $AUTH_H "http://localhost:8080/v1/credentials/registries?projectId=${PROJECT_ID}" | jq -r '.[0].id')

curl -s $AUTH_H "http://localhost:8080/v1/credentials/git/${GIT_CRED_ID}" | jq '.secret'
curl -s $AUTH_H "http://localhost:8080/v1/credentials/registries/${REG_CRED_ID}" | jq '.secret'
```

## Step 5 — Clean up (optional)

```bash
curl -s $AUTH_H -X DELETE "http://localhost:8080/v1/credentials/git/${GIT_CRED_ID}" | jq
curl -s $AUTH_H -X DELETE "http://localhost:8080/v1/credentials/registries/${REG_CRED_ID}" | jq
```

Deleting a credential frees its name within the associated scope so you can
rotate tokens without lingering records. The encrypted payloads are removed
immediately; audit events capture the deletion for review.
