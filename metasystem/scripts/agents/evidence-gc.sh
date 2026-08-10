#!/usr/bin/env bash
# Collect chain evidence that is already durable elsewhere.
#
# The standing rule has two halves: raw run evidence under gitignored
# artifacts/ is mirrored to the durable evidence root before it counts as
# disposable — and disposable evidence eventually gets disposed of. The second
# half never ran anywhere until a human browsing the template found months of
# closed critique chains living beside live state. artifacts/ holds LIVE
# state; history belongs to the evidence root, outside the repository.
#
# Safety: a chain is collected only when every job of it is terminal, and
# every file under its payload directory is listed in the mirror's manifest
# with a matching hash. Anything else is refused loudly, never skipped
# silently. Job records (jobs/*.json) always stay: they are the registry.
set -euo pipefail
metasystem_gc_root=$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)
ms="${METASYSTEM_BIN:-$metasystem_gc_root/bin/metasystem}"
cd "$metasystem_gc_root"

if [[ ${1:-} != __lease-held ]]; then
  lease_result=$("$ms" lease require-holder --root "$metasystem_gc_root" \
    --caller-pid "$$") || exit $?
  lease_epoch=$("$ms" json get --value "$lease_result" --field claimEpoch --default "")
  if [[ -n "$lease_epoch" ]]; then
    exec "$ms" lease run-held --root "$metasystem_gc_root" \
      --caller-pid "$$" --expected-epoch "$lease_epoch" -- "$0" __lease-held "$lease_epoch"
  fi
  exec "$ms" lease run-held --root "$metasystem_gc_root" \
    --caller-pid "$$" -- "$0" __lease-held human
fi
shift
expected_epoch=${1:-}
[[ -n "$expected_epoch" ]] || exit 2
shift
if [[ "$expected_epoch" =~ ^[1-9][0-9]*$ ]]; then
  "$ms" lease require-holder --root "$metasystem_gc_root" \
    --caller-pid "$$" --expected-epoch "$expected_epoch" >/dev/null
else
  [[ "$expected_epoch" == human ]] || exit 2
  "$ms" lease require-holder --root "$metasystem_gc_root" \
    --caller-pid "$$" >/dev/null
fi

evidence=$(scripts/metasystem-config.sh get --key evidence.root --default '' 2>/dev/null || true)
[[ "$evidence" == /* ]] || { echo "evidence-gc refused: evidence.root is not configured" >&2; exit 1; }

exec "$ms" evidence gc --root "$metasystem_gc_root" --evidence "$evidence" \
  --grace-seconds "${METASYSTEM_CHAIN_GRACE_SECONDS:-5400}"
