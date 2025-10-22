# Helm repository deployment sample

This sample demonstrates how to deploy an application from a Helm chart hosted in
an HTTPS repository. The automation under `samples/02-helm-repo/` prepares the
payload for `/v1/apps/validate`, deploys the chart via `/v1/projects/{id}/apps`,
inspects delivery metadata, and cleans up the app when you are finished.

## Prerequisites

- kubeOP API reachable from your workstation.
- Admin token capable of creating apps in the chosen project.
- A project identifier returned by `/v1/projects`.
- Helm chart URL that resolves to an allowed host (HTTPS port 443) and returns a
  `.tgz` archive.
- `curl`, `jq`, and optionally `kubectl` installed locally.

## Configure the sample

```bash
cd samples/02-helm-repo
cp .env.example .env
# Edit .env with AUTH_TOKEN, PROJECT_ID, APP_NAME, and HELM_CHART_URL values.
```

Key environment variables:

- `AUTH_TOKEN` — admin JWT used in the `Authorization` header.
- `PROJECT_ID` — target project for the deployment.
- `APP_NAME` — kubeOP app name and Helm release name.
- `HELM_CHART_URL` — HTTPS URL to the Helm chart archive (`.tgz`).
- `HELM_VALUES_JSON` — optional JSON object merged into `helm.values`.
- `CONTAINER_PORT`, `SERVICE_PORT`, `SERVICE_TYPE` — default service exposure.
- `DRY_RUN` — defaults to `1` (log-only). Set to `0` to execute the API calls.

## Preview the API calls

```bash
make -C samples helm-repo-plan
```

`curl.sh` renders the payload with the supplied chart URL and values, logs the
`/v1/apps/validate` request, and stores dry-run output under
`samples/.data/helm-repo/`.

## Deploy the chart

```bash
make -C samples helm-repo-exec
```

When `DRY_RUN=0`, the script posts to `/v1/projects/{id}/apps`, saves the
response to `samples/.data/helm-repo/deploy.json`, and records the returned app
identifier in `samples/.data/helm-repo/app_id` for later verification.

## Inspect delivery metadata

```bash
make -C samples helm-repo-verify
```

`verify.sh` retrieves `/v1/projects/{id}/apps/{appId}` and the associated
`/delivery` payload, pretty-prints the JSON responses, and logs optional `kubectl`
commands if the CLI is available.

## Clean up

```bash
make -C samples helm-repo-clean
```

The cleanup script deletes the app (when `DRY_RUN=0`) and removes the sample
state stored under `samples/.data/helm-repo/`.

## Run everything in sequence

```bash
make -C samples helm-repo-all
```

This composite target previews the payloads, deploys the chart, verifies the
app, and cleans up afterwards.
