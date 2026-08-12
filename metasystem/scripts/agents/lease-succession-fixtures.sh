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
git -C "$checkout" init -q .
start=$("$ms" proc started-at --pid $$)

announce() { "$ms" lease announce --root "$checkout" --session mission-runner-bm-2 \
  --pid $$ --start "$start" --tag mission-runner.sh --runtime fake "$@"; }

announce >/dev/null
announce --owner-lineage mission-deadbeef >/dev/null
python3 - "$checkout" <<'PY'
import glob, json, sys
from pathlib import Path
root = Path(sys.argv[1])
lease = json.loads((root / "artifacts/agents/mains/worktree-lease.json").read_text())
records = [
    p for p in glob.glob(str(root / "artifacts/agents/mains/*.json"))
    if not any(name in p for name in ("worktree-lease", "reaped-after-claim", "protocol-cursor"))
]
announcement = json.loads(Path(records[0]).read_text())
failures = []
if announcement.get("ownerLineage") != "mission-deadbeef":
    failures.append("supplying a lineage where none was stored must fill it in")
if lease.get("ownerLineage") != "mission-deadbeef":
    failures.append("the fill must reach the lease, which is what a claim reads")
if lease.get("claimEpoch") != 1:
    failures.append("filling in a lineage must not bump the epoch")
for line in failures:
    print(f"lease succession fixture failed: {line}", file=sys.stderr)
raise SystemExit(1 if failures else 0)
PY

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
git -C "$turns" init -q .
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
python3 - "$turns" <<'PY'
import json, sys
from pathlib import Path
root = Path(sys.argv[1])
lease = json.loads((root / "artifacts/agents/mains/worktree-lease.json").read_text())
job = json.loads((root / "artifacts/agents/jobs/inflight.json").read_text())
failures = []
if lease["claimEpoch"] != 1:
    failures.append("a second host turn must renew, not bump the epoch")
if lease["takeovers"]:
    failures.append("a second host turn of the same mission is not a takeover")
if job["status"] != "pending":
    failures.append("a delegate left in flight across a turn boundary must survive")
for line in failures:
    print(f"lease succession fixture failed: {line}", file=sys.stderr)
raise SystemExit(1 if failures else 0)
PY

# The wiring itself: if the host launcher stops exporting the lineage, host
# turns go back to taking the lease from each other and this whole fix is
# inert. The launcher lives in the engine now; pin it at its Go source when
# that is present (adopted repositories carry only the built binary).
if grep -qs '^module github.com/widoriezebos/agentic-tools/metasystem$' "$root/go.mod"; then
  grep -Fq '"METASYSTEM_OWNER_LINEAGE="+MissionLineage(e.Mission)' \
    "$root/internal/missionrunner/host.go" || {
    echo "lease succession fixture failed: the host launcher no longer exports the mission lineage" >&2
    exit 1
  }
fi

echo "lease succession fixtures: PASSED"
