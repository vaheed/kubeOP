# Version compatibility quick check

This tutorial demonstrates how to validate kubeOP build compatibility from a fresh environment before running automation.

## Prerequisites

- kubeOP API running locally via Docker Compose or `make run`.
- `.env.example` copied to `.env` if you need local overrides.
- `jq` installed for JSON parsing.

## 1. Fetch the server metadata

```bash
#!/usr/bin/env bash
set -euo pipefail
API_URL="${API_URL:-http://localhost:8080}"
LOGS_ROOT="${LOGS_ROOT:-./logs}"
mkdir -p "$LOGS_ROOT"

curl -sS "$API_URL/v1/version" | tee "$LOGS_ROOT/version.json"
```

The response includes the server version, Git commit, build date, and compatibility matrix.

## 2. Compare with your client/tooling version

```bash
#!/usr/bin/env bash
set -euo pipefail
CLIENT_VERSION="${CLIENT_VERSION:-0.8.19}"
SERVER_JSON="${SERVER_JSON:-./logs/version.json}"

REQUIRED=$(jq -r '.compatibility.minClientVersion' "$SERVER_JSON")
if [[ -z "$REQUIRED" ]]; then
  echo "Server does not publish a minimum client version; continuing"
  exit 0
fi

if go run golang.org/x/mod/semver@latest cmp "v$CLIENT_VERSION" "v$REQUIRED" | grep -q '^-'; then
  echo "Client version $CLIENT_VERSION is older than required $REQUIRED" >&2
  exit 1
fi

echo "Client version $CLIENT_VERSION satisfies minimum requirement $REQUIRED"
```

Replace `CLIENT_VERSION` with the kubeOP CLI version your automation uses. The script exits non-zero when the client is too old.

## 3. Watch for deprecation warnings

Deprecated builds log warnings once the deadline passes. Tail the logs to confirm whether an upgrade is required:

```bash
#!/usr/bin/env bash
set -euo pipefail
LOGS_ROOT="${LOGS_ROOT:-./logs}"

tail -n0 -F "$LOGS_ROOT/app.log" | grep --line-buffered 'deprecated kubeOP build'
```

If the server is out of support, the message `running deprecated kubeOP build` appears during startup and when `/v1/version` is queried.

## Result

You now have a repeatable check that blocks automation when it targets an unsupported kubeOP release, and operational insight for when the control plane is past its deprecation deadline.
