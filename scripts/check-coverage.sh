#!/usr/bin/env sh
set -eu

threshold="${COVERAGE_THRESHOLD:-80.0}"
profile="${COVERAGE_PROFILE:-coverage.out}"

go test ./... -coverprofile="$profile"
coverage="$(go tool cover -func="$profile" | awk '/^total:/ {gsub(/%/, "", $3); print $3}')"

awk -v coverage="$coverage" -v threshold="$threshold" 'BEGIN {
  if (coverage + 0 < threshold + 0) {
    printf("coverage %.1f%% is below required %.1f%%\n", coverage, threshold)
    exit 1
  }
  printf("coverage %.1f%% meets required %.1f%%\n", coverage, threshold)
}'
