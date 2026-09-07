# Bad Files Checker

A small command-line utility that recursively scans a filesystem path and writes a human-readable log of folders that look incomplete, damaged, or suspicious.

## Build

```bash
go build ./cmd/bad-files-checker
```

## Run

```bash
bad-files-checker --scan-path /data/images --log-file /logs/bad-files.log
```

The scanner defaults to low-impact runtime settings for shared filesystems:

- `--nice 10`: lowers CPU scheduling priority on Linux.
- `--ionice-class best-effort --ionice-level 7`: uses the lowest best-effort Linux I/O priority.
- `--scan-delay 0s`: no artificial pause by default; set values such as `10ms` if the storage pool needs extra breathing room.

On non-Linux systems, niceness and ionice settings are accepted for config portability but do not change process priority.

Exit codes:

- `0`: scan completed and no bad folders were found.
- `1`: scan completed and one or more bad folders were found.
- `2`: invalid arguments or fatal setup failure.

## Test Coverage

Coverage is required to stay at or above 80%. The CI workflow and local scripts fail the build when coverage drops below that threshold.

Linux/macOS:

```bash
sh scripts/check-coverage.sh
```

Windows PowerShell:

```powershell
.\scripts\check-coverage.ps1
```

## Docker

```bash
docker build -t bad-files-checker:latest .
docker run --rm \
  -v /mnt/tank/images:/data/images:ro \
  -v /mnt/tank/checker-logs:/logs \
  bad-files-checker:latest \
  --scan-path /data/images \
  --log-file /logs/bad-files.log \
  --nice 10 \
  --ionice-class best-effort \
  --ionice-level 7
```

The Dockerfile also includes an integration-test target. It installs the external archive tools and runs the Go test suite as an unprivileged Linux user:

```bash
docker build --target test .
```

Pushes to `main` publish the runtime image to GitHub Container Registry:

```bash
docker pull ghcr.io/jmkgreen/bad-files-checker:latest
docker pull ghcr.io/jmkgreen/bad-files-checker:<commit-sha>
```

## Current Behavior

- Directory symlinks are skipped.
- Hidden files are treated like ordinary files.
- "Archive only" means one or more regular archive files and no regular non-archive files.
- Subdirectories, symlinks, and special filesystem entries do not prevent a folder from being classified as archive-only.
- Unsupported or unavailable archive validation tools are logged as findings.
- Permission-denied paths inside the scanned tree are logged as findings where possible.
- `.tar.xz` and `.txz` validation requires tar validation. The checker does not accept an XZ-only integrity check as proof that the tar archive is valid.
- RAR validation depends on the installed RAR tool. The provided Docker image uses `unrar-free`, which can validate older RAR formats but is not expected to support every RAR variant, including many RAR5 archives.

## Documentation

See `REQUIREMENTS.md` for high level requirements.
