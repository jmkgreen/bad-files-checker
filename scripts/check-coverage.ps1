$ErrorActionPreference = "Stop"

$threshold = if ($env:COVERAGE_THRESHOLD) { [double]$env:COVERAGE_THRESHOLD } else { 80.0 }
$profile = if ($env:COVERAGE_PROFILE) { $env:COVERAGE_PROFILE } else { "coverage.out" }

go test ./... -coverprofile="$profile"
$coverageLine = go tool cover -func="$profile" | Select-String '^total:'
$coverage = [double](($coverageLine -split '\s+')[-1].TrimEnd('%'))

if ($coverage -lt $threshold) {
    Write-Error ("coverage {0:N1}% is below required {1:N1}%" -f $coverage, $threshold)
}

Write-Host ("coverage {0:N1}% meets required {1:N1}%" -f $coverage, $threshold)
