# Sample 00 — Bootstrap smoke test

This introductory sample verifies that kubeOP is reachable and that the logging helpers work as expected. The scripts print example payloads for bootstrapping a user and project but do not mutate the API by default.

## Prerequisites

- kubeOP API reachable at the URL defined by `KUBEOP_BASE_URL`.
- Admin JWT stored in the sample `.env` (`AUTH_TOKEN`).
- `curl`, `jq`, and `base64` installed locally.

## Steps

1. Copy `.env.example` to `.env` and fill the required variables.
2. Run `./curl.sh` to preview bootstrap requests (dry-run output only).
3. Run `./verify.sh` to check that health endpoints respond with `200`.
4. Run `./cleanup.sh` to remove temporary files created by the helpers.

All scripts emit structured logs with timestamps, and `verify.sh` exits non-zero if the health endpoints are unavailable.
