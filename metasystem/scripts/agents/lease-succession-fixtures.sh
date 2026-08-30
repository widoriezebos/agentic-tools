#!/usr/bin/env bash
set -euo pipefail

# A mission runs as a CHAIN of processes (staging, the resume after the human
# signs, any re-arm). Each announces its own mainId, so before ownership
# lineages every succession looked like a foreign takeover: the epoch bumped and
# the predecessor's in-flight delegates were failed as stale-claim-epoch. bm-2's
# first live cohort lost two of three delegates that way.
#
# Two legs retired with the python lease helper, and WHY:
#  - the claim-matrix scenarios (renewal preserves epoch/work, foreign takeover
#    sweeps, live holder never displaced, interrupted-sweep completion, legacy
#    lease loading) drove module internals of worktree-lease.py; they are owned
#    by internal/lease's unit tests (claim_test.go, recovery_test.go,
#    sweep_test.go) under the go gate.
#  - the lineage DERIVATION properties (hash-derived, bounded, no shared-prefix
#    aliasing, deterministic) are owned by TestMissionLineage in
#    internal/missionrunner.
# What stays HERE is what only real processes can prove: the announce CLI's
# lineage immutability across calls, and the bm-2 scenario of consecutive
# host-turn processes renewing (not seizing) the lease around a live delegate.

root=$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)
ms="${METASYSTEM_BIN:-$root/bin/metasystem}"
[[ -x "$ms" ]] || { echo "lease succession fixtures: binary absent; run the go gate first" >&2; exit 1; }
tmp=$(mktemp -d "${TMPDIR:-/tmp}/metasystem-lease-succession.XXXXXX")
trap 'rm -rf "$tmp"' EXIT

# A lineage is immutable once set for a process: absent-to-present is the only
# transition, and a conflicting one is refused rather than silently preferred.
checkout="$tmp/announce"
mkdir -p "$checkout"
git -C "$checkout" init -q -b main .
start=$("$ms" proc started-at --pid $$)

announce() { "$ms" lease announce --root "$checkout" --session mission-runner-bm-2 \
  --pid $$ --start "$start" --tag metasystem-mission-runner --runtime fake "$@"; }

announce >/dev/null
announce --owner-lineage mission-deadbeef >/dev/null
lease_file="$checkout/artifacts/agents/mains/worktree-lease.json"
announcement=
for main_record in "$checkout"/artifacts/agents/mains/*.json; do
  case $main_record in
    *worktree-lease*|*reaped-after-claim*|*protocol-cursor*) continue ;;
  esac
  announcement=$main_record
  break
done
[[ -n "$announcement" ]] \
  || { echo "lease succession fixture failed: the announce wrote no main record" >&2; exit 1; }
succession_failed=0
succession_failure() { echo "lease succession fixture failed: $1" >&2; succession_failed=1; }
[[ "$("$ms" json get --file "$announcement" --field ownerLineage --default null)" == mission-deadbeef ]] \
  || succession_failure "supplying a lineage where none was stored must fill it in"
[[ "$("$ms" json get --file "$lease_file" --field ownerLineage --default null)" == mission-deadbeef ]] \
  || succession_failure "the fill must reach the lease, which is what a claim reads"
[[ "$("$ms" json get --file "$lease_file" --field claimEpoch --default null)" == 1 ]] \
  || succession_failure "filling in a lineage must not bump the epoch"
(( succession_failed == 0 )) || exit 1

announce --owner-lineage mission-deadbeef >/dev/null

if announce --owner-lineage mission-different >/dev/null 2>"$tmp/conflict.err"; then
  echo "lease succession fixture failed: a conflicting lineage must be refused" >&2
  exit 1
fi
grep -Fq "refusing to replace it" "$tmp/conflict.err" || {
  echo "lease succession fixture failed: the refusal must say what it refused" >&2
  cat "$tmp/conflict.err" >&2
  exit 1
}

# The bm-2 scenario itself: consecutive HOST TURNS are separate processes that
# each arm and hold the lease. A turn that ends with a delegate still running
# must not have it swept by the next turn's host. The lineage reaches those
# processes through the environment, because a host's arming is a session hook
# rather than a call this code makes.
turns="$tmp/turns"
mkdir -p "$turns"
git -C "$turns" init -q -b main .
mkdir -p "$turns/artifacts/agents/jobs"
cat >"$tmp/turn.sh" <<EOS
start=\$("$ms" proc started-at --pid \$\$)
"$ms" lease announce --root "$turns" --session "host-\$\$" --pid \$\$ \
  --start "\$start" --tag metasystem-host-turn --runtime claude \
  \${METASYSTEM_OWNER_LINEAGE:+--owner-lineage "\$METASYSTEM_OWNER_LINEAGE"} >/dev/null
EOS
export METASYSTEM_OWNER_LINEAGE=mission-fixture-lineage
bash "$tmp/turn.sh"
printf '%s\n' '{"jobId":"inflight","status":"pending","claimEpoch":1,"mainId":"m","role":"r","runtime":"devin"}' \
  >"$turns/artifacts/agents/jobs/inflight.json"
bash "$tmp/turn.sh"
unset METASYSTEM_OWNER_LINEAGE
turns_lease="$turns/artifacts/agents/mains/worktree-lease.json"
turns_job="$turns/artifacts/agents/jobs/inflight.json"
turn_failed=0
turn_failure() { echo "lease succession fixture failed: $1" >&2; turn_failed=1; }
[[ "$("$ms" json get --file "$turns_lease" --field claimEpoch)" == 1 ]] \
  || turn_failure "a second host turn must renew, not bump the epoch"
# An empty takeover list renders as [] (or null when never seized).
turns_takeovers=$("$ms" json get --file "$turns_lease" --field takeovers) \
  || { echo "lease succession fixture failed: the lease has no takeovers field" >&2; exit 1; }
[[ "$turns_takeovers" == "[]" || "$turns_takeovers" == null ]] \
  || turn_failure "a second host turn of the same mission is not a takeover"
[[ "$("$ms" json get --file "$turns_job" --field status)" == pending ]] \
  || turn_failure "a delegate left in flight across a turn boundary must survive"
(( turn_failed == 0 )) || exit 1

# The lineage-export wiring is pinned in Go, not by grepping source text
# from shell (script-fixtures-019): internal/missionrunner's
# TestAssembleHostCommandExportsMissionLineage asserts the constructed
# host command's environment carries METASYSTEM_OWNER_LINEAGE.

echo "lease succession fixtures: PASSED"
