# kubeOP Samples

The samples suite provides copy-paste automation that demonstrates kubeOP workflows without requiring a bespoke CLI. Each sample directory contains:

- `README` — short pointers back to the documentation in this directory.
- `.env.example` — per-sample configuration to copy into `.env`.
- `curl.sh`, `verify.sh`, `cleanup.sh` — executable scripts that orchestrate API calls.
- Shared logging and validation helpers sourced from `lib/common.sh`.

All scripts default to `set -euo pipefail`, log steps with ISO-8601 timestamps, and rely on environment variables defined in `.env.samples` plus the sample `.env`.

### Quick automation via Makefile

The `samples/Makefile` wraps the bootstrap flows with explicit logging so teams can run smoke tests without changing directories:

```bash
make -C samples bootstrap-env
make -C samples bootstrap-dry-run
make -C samples bootstrap-verify
make -C samples bootstrap-clean
make -C samples onboard-env
make -C samples onboard-plan
make -C samples onboard-exec
make -C samples onboard-verify
make -C samples onboard-clean
```

Each target prints the step it is performing and exits early if the sample `.env` is missing.

## Catalog

| Directory | Docs | Summary |
|-----------|------|---------|
| `00-bootstrap` | [Bootstrap walkthrough](./00-bootstrap.md) | End-to-end tenant + project bootstrap with health verification and cleanup helpers. |
| `01-tenant-project` | [Tenant onboarding & project provisioning](./01-tenant-project.md) | Scripts that create a tenant via `/v1/users/bootstrap`, provision a project, and preview verification flows. |
| `jobs` | [Jobs & CronJobs](./02-jobs.md) | Kubernetes Job and CronJob manifests wired with kubeOP tenancy labels for batch automation experiments. |

## Usage

```bash
cd samples/00-bootstrap
cp .env.example .env
# edit .env with tokens and project identifiers
./curl.sh
./verify.sh
./cleanup.sh

cd ../01-tenant-project
cp .env.example .env
# edit .env with tenant details, project name, and cluster id
./curl.sh    # DRY_RUN=1 logs commands, DRY_RUN=0 executes them
./verify.sh  # previews project lookup and kubeconfig renewal steps
./cleanup.sh
```

The helpers exit early when required environment variables or commands are missing, making the scripts safe to run in CI or locally.
