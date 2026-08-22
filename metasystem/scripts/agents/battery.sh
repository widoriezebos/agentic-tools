#!/usr/bin/env bash
# The executable covenant's battery half: ONE entrypoint, a durable
# verdict file, and a mechanical verdict check — the ugrep -qv form
# once read red as green, so the check is =[1-9] on the codes file
# and nothing else. Evidence survives under artifacts/ either way.
set -euo pipefail

# The battery takes no arguments: anything passed is a mistake, and
# a mistake must not cost a forty-minute suite run (--help once did).
if (( $# )); then
  echo "usage: scripts/agents/battery.sh   (no arguments; runs the full validate suite, writes a durable verdict under artifacts/agents/battery/)" >&2
  [[ ${1:-} == --help || ${1:-} == -h ]] && exit 0
  exit 2
fi

root=$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)
stamp=$(date -u +%Y%m%dT%H%M%SZ)
out_dir="$root/artifacts/agents/battery"
mkdir -p "$out_dir"
log="$out_dir/$stamp.log"
codes="$out_dir/$stamp.codes"

# The verdict is recorded WHATEVER the suite returns: set -e dying
# on a red validate would leave no codes file — the exact silent
# outcome this entrypoint exists to prevent (its own first run did
# exactly that).
rc=0
"$root/scripts/validate-metasystem.sh" >"$log" 2>&1 || rc=$?
echo "validate-metasystem=$rc" >"$codes"

if grep -qE '=[1-9][0-9]*' "$codes"; then
  echo "BATTERY-RED: $(cat "$codes")"
  echo "log: $log"
  tail -15 "$log"
  exit 1
fi
echo "BATTERY-GREEN: $(cat "$codes")"
echo "log: $log"
