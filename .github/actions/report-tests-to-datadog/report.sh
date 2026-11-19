#!/usr/bin/env bash
# Usage: report.sh <suite> <go-test-log> [coverprofile]
set -euo pipefail

suite="${1:?usage: report.sh <suite> <go-test-log> [coverprofile]}"
log="${2:?usage: report.sh <suite> <go-test-log> [coverprofile]}"
profile="${3:-}"
: "${DATADOG_CI:?DATADOG_CI must point at the datadog-ci binary}"
: "${DD_SERVICE:?DD_SERVICE must be set}"
: "${GO_JUNIT_REPORT_VERSION:?GO_JUNIT_REPORT_VERSION must be set}"

if [[ ! -s "$log" ]]; then
	echo "::error::Suite $suite has no test output at $log."
	exit 1
fi

junit="${log%.*}.junit.xml"
go run "github.com/jstemmer/go-junit-report/v2@${GO_JUNIT_REPORT_VERSION}" -in "$log" -out "$junit"

args=(--service "$DD_SERVICE" --env ci --tags "suite:$suite" --skip-git-metadata-upload)

pct=""
if [[ -n "$profile" && -s "$profile" ]]; then
	pct="$(go tool cover -func="$profile" | awk '$1 == "total:" { sub(/%$/, "", $NF); print $NF }')"
fi
if [[ -n "$pct" ]]; then
	args+=(--report-measures "test.code_coverage.lines_pct:$pct")
fi

echo "Uploading suite $suite${pct:+ (coverage $pct%)}"
"$DATADOG_CI" junit upload "${args[@]}" "$junit"
