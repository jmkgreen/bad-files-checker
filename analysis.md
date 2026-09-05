# Bad Files Checker Analysis

Updated review recorded on 2026-09-05 against `REQUIREMENTS.md`. Earlier findings were rechecked and removed when addressed by the current code or tests.

## Remaining Gaps

### Medium: External archive validation is not tested against real tools or archives

`internal/scanner/archive_test.go` uses a fake command runner for `.7z`, `.rar`, and `.tar.xz`, while the Docker image supplies `7z`, `unrar-free`, and `tar`. There is no integration test proving that valid and corrupt files for those formats produce the expected result in the container. In particular, RAR support can vary by archive version and by the capabilities of `unrar-free`.

Recommended follow-up: add a Linux/container integration test with representative valid and corrupt `.7z`, `.rar`, `.tar.xz`, and `.txz` fixtures, and document the supported RAR variants.

### Medium: Non-archive read failures are not covered by an executable test

The implementation attempts a full read for regular non-archive files, but no test forces `readFully` or `io.Copy` to fail. This leaves the required CRC/readability behavior unverified. A portable unit test would require injecting the file-opening/read operation; alternatively, use a Linux filesystem integration test that reliably produces a read error.

### Low: Permission integration coverage skips the container's default root user

`internal/scanner/permission_linux_test.go` skips when running as root because `chmod 000` is ineffective for root. The test therefore verifies the corrected attribution only under a non-root Linux user. The scanner behavior is covered by the helper tests, but the deployed Docker execution path is not exercised for actual permission-denied traversal.

Recommended follow-up: run the integration test as an unprivileged container user, or add a fixture that produces a permission failure without relying on directory mode bits.

### Low: Terminal output write errors are ignored

`internal/app/app.go` still ignores errors from the final `fmt.Fprintf`/`fmt.Fprintln` calls to `stdout`. This does not affect the log file, but a broken pipe or unavailable output stream can still result in a successful exit code. The log write and close errors are now handled.

## Validation Status

- `go test ./...` passes in a Go 1.22 container.
- Measured total coverage is 81.1%, above the documented 80% threshold.
- CI runs the repository coverage script on Ubuntu.
- The host environment still does not have `go` on `PATH`; validation was performed through Docker.

## Current Assessment

No previously reported functional defect remains confirmed in the current implementation. The remaining risk is primarily integration coverage for external archive tools and filesystem failures that are difficult to reproduce portably.
