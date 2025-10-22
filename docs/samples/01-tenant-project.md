# Tenant onboarding & project provisioning sample

This sample automates the workflow for bootstrapping a tenant, creating a
project, and preparing verification commands using the kubeOP REST API. It builds
on the shared logging helpers under `samples/lib/common.sh` so every step is
logged with ISO-8601 timestamps and fails fast when prerequisites are missing.

## Prerequisites

- kubeOP API reachable from your workstation.
- Admin JWT containing `{"role":"admin"}` assigned to `AUTH_TOKEN`.
- Cluster identifier from `/v1/clusters` for `CLUSTER_ID`.
- `curl` and `jq` installed locally.

## 1. Prepare configuration

```bash
cd samples/01-tenant-project
cp .env.example .env
```

Edit `.env` and populate the following variables:

| Variable | Description |
| --- | --- |
| `AUTH_TOKEN` | Admin bearer token for authenticated API calls. |
| `TENANT_EMAIL` | Email address for the tenant being bootstrapped. |
| `TENANT_NAME` | Display name associated with the tenant user. |
| `PROJECT_NAME` | Friendly name for the project namespace. |
| `CLUSTER_ID` | Target cluster identifier returned from `/v1/clusters`. |
| `DRY_RUN` | Defaults to `1`. Set to `0` to execute the API calls instead of logging them. |

All scripts inherit defaults from `samples/.env.samples` (base URL, log
location, scratch directory).

## 2. Preview or execute onboarding calls

The sample orchestrates two API calls: `POST /v1/users/bootstrap` to create the
tenant namespace and `POST /v1/projects` to provision a project scoped to the
cluster. Run the script in preview mode (default `DRY_RUN=1`) to inspect the
payloads and curl commands:

```bash
./curl.sh
```

Example output:

```text
2025-11-05T12:00:00Z [STEP] Preparing tenant bootstrap payloads
2025-11-05T12:00:00Z [STEP] POST /v1/users/bootstrap
2025-11-05T12:00:00Z [INFO] Payload: {"name":"Alice Example","email":"alice@example.com","clusterId":"clu-12345"}
2025-11-05T12:00:00Z [WARN] DRY_RUN=1 skipping POST /v1/users/bootstrap
2025-11-05T12:00:00Z [INFO] curl -sS -H 'Authorization: Bearer ***' -H 'Content-Type: application/json' -X POST -d '{"name":"Alice Example","email":"alice@example.com","clusterId":"clu-12345"}' http://localhost:8080/v1/users/bootstrap
...
2025-11-05T12:00:00Z [WARN] Set DRY_RUN=0 to execute the curl commands against the API
```

To perform the API calls, set `DRY_RUN=0` either in `.env` or inline when
invoking the script:

```bash
DRY_RUN=0 ./curl.sh
```

Responses are logged so you can capture generated IDs or kubeconfig payloads.

## 3. Verify tenant and project access

`verify.sh` demonstrates follow-up commands for listing projects and renewing
project kubeconfigs. The script honours `DRY_RUN`; keep it at `1` to log the
commands or switch to `0` to execute them.

```bash
./verify.sh
```

When `DRY_RUN=0`, the script performs `GET /v1/projects?limit=10` and
`POST /v1/projects/<project-id>/kubeconfig/renew` (replace `<project-id>` in the
log output before running the final command).

## 4. Clean up local artefacts

Both onboarding and verification scripts may write logs or temporary files to
`samples/.data`. Remove them with:

```bash
./cleanup.sh
```

You can run the Makefile wrapper instead of calling scripts directly:

```bash
make -C samples onboard-all
```

This target previews the onboarding flow, displays verification commands, and
cleans up scratch directories in sequence.

## 5. Next steps

- Extend the script with quota overrides by adding a `quotaOverrides` object to
  the project payload.
- Pipe successful responses into `jq` to capture user IDs and project
  namespaces for follow-on automation.
- Combine this sample with the delivery workflows under `samples/jobs/` to spin
  up batch workloads immediately after onboarding a tenant.
