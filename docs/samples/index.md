# kubeOP Samples

The samples suite provides copy-paste automation that demonstrates kubeOP workflows without requiring a bespoke CLI. Each sample directory contains:

- `README` — short pointers back to the documentation in this directory.
- `.env.example` — per-sample configuration to copy into `.env`.
- `curl.sh`, `verify.sh`, `cleanup.sh` — executable scripts that orchestrate API calls.
- Shared logging and validation helpers sourced from `lib/common.sh`.

All scripts default to `set -euo pipefail`, log steps with ISO-8601 timestamps, and rely on environment variables defined in `.env.samples` plus the sample `.env`.

## Usage

```bash
cd samples/00-bootstrap
cp .env.example .env
# edit .env with tokens and project identifiers
./curl.sh
./verify.sh
./cleanup.sh
```

The helpers exit early when required environment variables or commands are missing, making the scripts safe to run in CI or locally.
