# Samples scaffolding walkthrough

This tutorial introduces the kubeOP samples suite and shows how to run the
bootstrap example that validates connectivity and logging helpers. The samples
are designed to be copy-paste friendly, emit structured logs, and fail fast when
required tools or environment variables are missing.

## Prerequisites

- kubeOP running locally or remotely (see [quickstart](../getting-started.md)).
- `curl`, `jq`, and `base64` installed.
- An admin JWT with the `{"role":"admin"}` claim.

## 1. Prepare the environment

```bash
cd kubeOP
cp samples/00-bootstrap/.env.example samples/00-bootstrap/.env
```

Edit `samples/00-bootstrap/.env` and set the following variables:

| Variable | Description |
| --- | --- |
| `AUTH_TOKEN` | Admin JWT used for authenticated requests. |
| `PROJECT_ID` | Project identifier used in example payloads. |
| `USER_EMAIL` | Email address used when previewing bootstrap payloads. |
| `CLUSTER_ID` | Cluster identifier returned from `/v1/clusters`. |

The scripts automatically source `samples/.env.samples`, which defaults to
`KUBEOP_BASE_URL=http://localhost:8080`, `SAMPLES_LOG_ROOT=./logs`, and a scratch
directory under `samples/.data`.

## 2. Preview bootstrap commands

```bash
cd samples/00-bootstrap
./curl.sh
```

Example output:

```text
2025-10-31T12:00:00Z [STEP] Generating bootstrap payloads
2025-10-31T12:00:00Z [INFO] POST /v1/users/bootstrap
2025-10-31T12:00:00Z [INFO] Payload: {"name":"Sample User","email":"alice@example.com","clusterId":"clu-123"}
2025-10-31T12:00:00Z [INFO] curl -sS -H 'Authorization: Bearer ***' -H 'Content-Type: application/json' \
     -d '{"name":"Sample User","email":"alice@example.com","clusterId":"clu-123"}' http://localhost:8080/v1/users/bootstrap
...
2025-10-31T12:00:00Z [WARN] The bootstrap sample is a dry run. Remove the log_info wrappers to execute the calls.
```

The script validates required environment variables, checks for `curl`, `jq`,
and `base64`, and prints the commands you can run manually. To execute the API
calls, replace `log_info` with `eval` in the script (or copy the command into a
terminal).

## 3. Validate health endpoints

```bash
./verify.sh
```

Expected output:

```text
2025-10-31T12:00:00Z [STEP] Checking /healthz
2025-10-31T12:00:00Z [INFO] Health check OK
2025-10-31T12:00:00Z [STEP] Checking /readyz
2025-10-31T12:00:00Z [INFO] Readiness check OK
```

If either endpoint fails, the script logs an error and exits with a non-zero
status so CI pipelines or wrappers can detect the failure.

## 4. Clean up

```bash
./cleanup.sh
```

The cleanup script removes the scratch directory defined by
`SAMPLES_DATA_DIR` (`./samples/.data` by default) and logs the action.

## 5. Next steps

- Extend the scripts with additional API calls (e.g., deploy apps, list releases).
- Store generated logs under `${SAMPLES_LOG_ROOT}` to keep audit trails of sample runs.
- Version-control sample `.env` files only when they contain non-secret defaults.

The samples suite will expand with end-to-end flows in future roadmap items.
