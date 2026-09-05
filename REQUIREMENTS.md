# Bad Files Checker Requirements

## Goal

Build a small command-line utility that recursively checks a supplied filesystem path and records folders that appear incomplete, damaged, or suspicious.

The primary use case is scanning a hierarchy of image-set folders mounted into a Linux Docker container, including TrueNAS SCALE deployments.

## Launch Contract

The application should be runnable with:

```bash
bad-files-checker --scan-path /data/images --log-file /logs/bad-files.log
```

Required inputs:

- `--scan-path`: root directory to scan recursively.
- `--log-file`: path to write the scan results.

Optional inputs may be added later, but the first implementation should keep the interface simple and predictable.

## Folder Conditions To Report

The utility should report the path to any folder that meets one or more of these conditions:

- The folder is empty.
- The folder contains only archive files.
- The folder contains one or more zero-size files.
- The folder contains one or more files that fail a CRC/integrity check.
- The folder contains one or more archive files that fail archive validation.

## Archive Handling

When an archive file is found, the utility should validate the archive instead of merely noting that it exists.

Archive validation should detect corruption when supported by the archive format and available tooling. Initial archive formats should reasonably include:

- `.zip`
- `.7z`
- `.rar`
- `.tar`
- `.tar.gz`
- `.tgz`
- `.tar.bz2`
- `.tbz2`
- `.tar.xz`
- `.txz`

If an archive type cannot be validated because the format or required tool is unsupported, the log should record that clearly.

For `.tar.xz` and `.txz` files, validation must verify the tar archive structure, not only the outer XZ stream.

## CRC / Integrity Checks

For non-archive files, the phrase "CRC check" should be interpreted as a practical file integrity/readability pass unless a sidecar checksum file or embedded checksum format is available.

The first implementation should at minimum attempt to read each regular file fully so that filesystem-level read errors are surfaced. Future versions may add support for checksum sidecar files such as `.sfv`, `.md5`, or `.sha256`.

## Log Output

The log file should be human-readable and suitable for later searching.

Each reported folder should include:

- Folder path.
- One or more issue types.
- Relevant file paths within that folder.
- Validation or read error details when available.
- Timestamp for the scan.

The process should create the log file path if possible, including parent directories.

## Exit Behavior

Suggested exit codes:

- `0`: scan completed and no bad folders were found.
- `1`: scan completed and one or more bad folders were found.
- `2`: scan could not complete because of invalid arguments, missing paths, permission errors on required paths, or another fatal error.

Non-fatal read or validation failures inside the scanned tree should be logged as findings rather than immediately aborting the entire scan.

## Docker Requirements

The application should run inside a Linux container.

The container image should include any archive validation tools required by the implementation, such as:

- `unzip` or equivalent ZIP validation support.
- `p7zip` / `7z` for broad archive testing.
- `unrar` or a compatible RAR testing utility where licensing and distribution allow.
- `tar`, `gzip`, `bzip2`, and `xz` tooling for tar-based archives.

The image should be usable with mounted host volumes:

```bash
docker run --rm \
  -v /mnt/tank/images:/data/images:ro \
  -v /mnt/tank/checker-logs:/logs \
  bad-files-checker \
  --scan-path /data/images \
  --log-file /logs/bad-files.log
```

## Docker Compose Example

The project should include an example similar to:

```yaml
services:
  bad-files-checker:
    image: bad-files-checker:latest
    container_name: bad-files-checker
    volumes:
      - /mnt/tank/images:/data/images:ro
      - /mnt/tank/checker-logs:/logs
    command:
      - --scan-path
      - /data/images
      - --log-file
      - /logs/bad-files.log
```

## Implementation Notes

- Prefer a small, portable implementation with no unnecessary runtime services.
- The scanner should avoid modifying the scanned tree.
- Directory symlinks should not be followed by default.
- Permission-denied paths should be logged and should not crash the scan unless the root scan path itself is inaccessible.
- Paths in logs should be the container-visible paths, because those are the paths the process can reliably observe.
- "Archive only" should mean a folder contains one or more regular archive files and no regular non-archive files. Subdirectories, symlinks, and special filesystem entries do not currently prevent this classification.
- Unsupported or unavailable archive validation tools should be logged as findings.
- RAR validation depends on the validation tool available in the container. The initial Docker image uses `unrar-free`, which can validate older RAR formats but is not expected to support every RAR variant, including many RAR5 archives. If the tool cannot validate a RAR variant, the result may be reported as an archive validation failure.

## Open Questions Before Implementation

- Should the final log format be plain text, JSON Lines, CSV, or both plain text and structured output?
- Should "only an archive file" mean exactly one archive file, or one or more archive files and no non-archive files?
- Should hidden files such as `.DS_Store`, `Thumbs.db`, or metadata files be ignored when deciding whether a folder is empty or archive-only?
- Should directory symlinks be skipped, followed, or reported?
- Should unsupported archive formats be treated as findings, warnings, or ignored?
