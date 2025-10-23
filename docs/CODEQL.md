# CodeQL Remediation Notes

This change set resolves the outstanding CodeQL findings for Helm chart downloads and Git delivery path handling.

## Helm SSRF hardening (alerts #12, #16)

* All chart URLs must now parse as canonical HTTPS addresses using `pkg/security.ParseAndValidateHTTPSURL`. The helper rejects
  userinfo, alternate ports, path traversal (`..` or encoded variants), fragments, and IP literals before any network activity.
* Hosts are validated against the `HELM_CHART_ALLOWED_HOSTS` allow-list and the request is issued with a dedicated HTTP client
  that enforces:
  * A 15s request timeout, 10s TLS handshake/response header limits, and a fixed `User-Agent` header.
  * A redirect policy (`security.ValidateRedirect`) that caps hops at three and re-validates each target against the allow-list.
  * A transport dialer that only connects to the pre-resolved global unicast addresses. Private, loopback, link-local, and
    multicast ranges are rejected both at resolution time and at dial time via `security.DenyPrivateNetworks`.
  * Response bodies larger than 32 MiB are discarded to bound memory use.
* Hostname resolution continues to support dependency injection for tests but is now layered with the shared network
  validation helpers under `pkg/security`.

## Git path traversal hardening (alerts #17, #18)

* The new `pkg/security` path helpers normalise and resolve repository inputs via `NormalizeRepoPath`, `CleanRoot`, and
  `WithinRepo`, combining `filepath.Clean`, `EvalSymlinks`, and `filepath.Rel` checks. They reject encoded traversal, mixed
  separators, drive letters, and control characters up-front.
* `internal/delivery/git.go` now uses these helpers when validating checkout paths, selecting the base directory, and walking
  manifests. Every filesystem access is preceded by `WithinRepo` to guarantee targets remain inside the cloned repository even
  when symlinks are present.
* Device files, FIFOs, and sockets are rejected by `security.EnsureRegularFile` before reading, and directory walks leverage
  `fs.WalkDir` to skip symlinks that attempt to escape the repo root.

## Test coverage

* Unit tests exercise URL sanitisation edge cases (bad schemes, userinfo, IP literals, private DNS responses, redirect policy)
  and Git path handling (relative escapes, encoded traversals, symlink handling).
* Go fuzzers (`FuzzParseAndValidateHTTPSURL`, `FuzzWithinRepo`) live in `pkg/security` to continually stress the sanitizers.
