#!/bin/sh
set -eu

css='go/static/app.css'
[ -f "$css" ] || { printf '%s\n' "missing: $css" >&2; exit 1; }

for selector in '.site-header' '.panel' '.button' '.game-card' '.auth-panel' '.role-tabs'; do
  grep -Fq "$selector" "$css" || { printf '%s\n' "missing selector: $selector" >&2; exit 1; }
done

grep -Fq '@media (prefers-reduced-motion: reduce)' "$css" || {
  printf '%s\n' 'missing reduced-motion media query' >&2
  exit 1
}

printf '%s\n' 'frontend validation: PASS'
