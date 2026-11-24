#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/.."
compose_file="compose.yaml"
if grep -Eq 'DATABASE_URL:.*\*\*\*' "$compose_file"; then
  echo "DATABASE_URL contains a placeholder password" >&2
  exit 1
fi
if ! awk '
  /^  santa-go:/ { in_go=1; next }
  /^  [A-Za-z0-9-]+:/ { in_go=0 }
  in_go && /healthcheck:/ { found=1 }
  END { exit(found ? 0 : 1) }
' "$compose_file"; then
  echo "santa-go service is missing healthcheck" >&2
  exit 1
fi
printf 'compose validation passed\n'
