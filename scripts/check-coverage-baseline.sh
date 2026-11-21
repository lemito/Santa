#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/.."

baseline="${SANTA_COVERAGE_BASELINE:-coverage-baseline.txt}"
report="${SANTA_COVERAGE_REPORT:-artifacts/coverage/go.txt}"
[[ -f "$baseline" ]] || { echo "coverage baseline is missing: $baseline" >&2; exit 2; }
[[ -f "$report" ]] || { echo "coverage report is missing: $report" >&2; exit 2; }

awk -v baseline="$baseline" '
  BEGIN { while ((getline line < baseline) > 0) { split(line, a, "="); if (a[1] != "") minimum[a[1]] = a[2] } }
  /total:/ {
    for (i = 1; i <= NF; i++) if ($i ~ /[0-9]+\.[0-9]+%/) { value = $i; gsub("%", "", value) }
    if (minimum["total"] != "" && value + 0 < minimum["total"] + 0) { print "coverage below baseline: " value "% < " minimum["total"] "%"; failed = 1 }
  }
  END { exit failed }
' "$report"
