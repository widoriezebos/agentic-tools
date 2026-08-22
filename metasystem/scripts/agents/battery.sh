#!/usr/bin/env bash
# The executable covenant's battery half: ONE entrypoint, a durable
# verdict file, and a mechanical verdict check — the ugrep -qv form
# once read red as green, so the check is =[1-9] on the codes file
# and nothing else. Evidence survives under artifacts/ either way.
set -euo pipefail

root=$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)
stamp=$(date -u +%Y%m%dT%H%M%SZ)
out_dir="$root/artifacts/agents/battery"
mkdir -p "$out_dir"
log="$out_dir/$stamp.log"
codes="$out_dir/$stamp.codes"

"$root/scripts/validate-metasystem.sh" >"$log" 2>&1
echo "validate-metasystem=$?" >"$codes"

if grep -qE '=[1-9][0-9]*' "$codes"; then
  echo "BATTERY-RED: $(cat "$codes")"
  echo "log: $log"
  tail -15 "$log"
  exit 1
fi
echo "BATTERY-GREEN: $(cat "$codes")"
echo "log: $log"
