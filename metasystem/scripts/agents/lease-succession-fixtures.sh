#!/usr/bin/env bash
set -euo pipefail

# A mission runs as a CHAIN of processes (staging, the resume after the human
# signs, any re-arm). Each announces its own mainId, so before ownership
# lineages every succession looked like a foreign takeover: the epoch bumped and
# the predecessor's in-flight delegates were failed as stale-claim-epoch. bm-2's
# first live cohort lost two of three delegates that way.
#
# Both halves are proven here, because a fix that only preserves work would
# quietly disable the protection against an abandoned session's children:
# same-lineage succession RENEWS (epoch preserved, jobs survive) and a different
# lineage still TAKES OVER (epoch bumped, jobs swept).

root=$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)
helper="$root/scripts/agents/worktree-lease.py"
tmp=$(mktemp -d "${TMPDIR:-/tmp}/metasystem-lease-succession.XXXXXX")
trap 'rm -rf "$tmp"' EXIT

python3 - "$helper" "$tmp" <<'PY'
import importlib.util, json, os, sys
from pathlib import Path

helper_path, tmp = sys.argv[1], Path(sys.argv[2])
spec = importlib.util.spec_from_file_location("wl", helper_path)
wl = importlib.util.module_from_spec(spec)
spec.loader.exec_module(wl)

root = tmp / "checkout"
mains = root / "artifacts/agents/mains"
jobs = root / "artifacts/agents/jobs"
mains.mkdir(parents=True)
jobs.mkdir(parents=True)
lease = mains / "worktree-lease.json"
stamp = mains / "reaped-after-claim.json"

failures = []


def check(name, got, want):
    if got != want:
        failures.append(f"{name}: got {got!r}, want {want!r}")


def write_lease(lineage, *, epoch=5, pid=999999, start=100,
                holder="main-100-999-aaaaaa", revision=3):
    lease.write_text(json.dumps({
        "holderMainId": holder, "ownerLineage": lineage, "pid": pid,
        "pidStartedAt": start, "commandHash": "a" * 64,
        "claimedAt": "2026-01-01T00:00:00Z", "renewedAt": "2026-01-01T00:00:00Z",
        "takeovers": [], "revision": revision, "claimEpoch": epoch,
    }), encoding="utf-8")


def write_job(name, epoch):
    (jobs / f"{name}.json").write_text(json.dumps({
        "jobId": name, "status": "pending", "claimEpoch": epoch,
        "mainId": "main-100-999-aaaaaa", "role": "design-critic", "runtime": "fake",
    }), encoding="utf-8")


def job(name):
    return json.loads((jobs / f"{name}.json").read_text(encoding="utf-8"))


def current():
    return json.loads(lease.read_text(encoding="utf-8"))


me = os.getpid()
mystart = wl.started_at(root, me)
successor = {
    "sessionId": "mission-runner-bm-2", "mainId": "main-200-1234-bbbbbb",
    "pid": me, "pidStartedAt": mystart, "pgid": os.getpgid(0), "runtime": "fake",
    "instanceTag": "mission-runner.sh", "commandHash": "b" * 64,
    "announcedAt": "2026-01-01T00:00:00Z", "ownerLineage": "mission-abc",
}

# 1. The mission's own succession: renewal, epoch preserved, work survives.
write_lease("mission-abc"); write_job("survivor", 5)
wl.claim_for_announcement(root, successor)
value = current()
check("renewal preserves the epoch", value["claimEpoch"], 5)
check("renewal keeps in-flight work", job("survivor")["status"], "pending")
check("renewal is not a takeover", len(value["takeovers"]), 0)
check("renewal moves the holder", value["holderMainId"], "main-200-1234-bbbbbb")
# Liveness is the PAIR: a new pid beside the predecessor's start time would make
# the live successor test as dead and invite the takeover this prevents.
check("successor tests as live", wl.live(root, value["pid"], value["pidStartedAt"]), True)

# 2. A genuinely different writer over a dead holder still sweeps.
write_lease("mission-abc"); write_job("foreign", 5)
stranger = dict(successor, ownerLineage="mission-zzz")
wl.claim_for_announcement(root, stranger)
value = current()
check("foreign takeover bumps the epoch", value["claimEpoch"], 6)
check("foreign takeover sweeps", job("foreign")["status"], "failed")
check("foreign takeover is recorded", len(value["takeovers"]), 1)

# 3. No lineage anywhere behaves exactly as before this change.
write_lease("main-100-999-aaaaaa"); write_job("legacy", 5)
plain = {k: v for k, v in successor.items() if k != "ownerLineage"}
wl.claim_for_announcement(root, plain)
check("default lineage still takes over", current()["claimEpoch"], 6)
check("default lineage still sweeps", job("legacy")["status"], "failed")

# 4. A LIVE holder is never displaced -- not even by its own lineage.
write_lease("mission-abc", pid=me, start=mystart, holder="main-999-888-cccccc", revision=4)
before = lease.read_text(encoding="utf-8")
wl.claim_for_announcement(root, successor)
check("live holder keeps the lease", lease.read_text(encoding="utf-8"), before)

# 5. A predecessor's interrupted sweep is finished, without raising the epoch.
write_lease("mission-abc", epoch=7)
stamp.write_text(json.dumps({
    "holderMainId": "main-100-999-aaaaaa", "claimEpoch": 6, "reapedAt": "x",
}), encoding="utf-8")
write_job("abandoned", 6)
write_job("inflight", 7)
wl.claim_for_announcement(root, successor)
check("interrupted sweep does not raise the epoch", current()["claimEpoch"], 7)
check("interrupted sweep completes", job("abandoned")["status"], "failed")
check("interrupted sweep spares this epoch", job("inflight")["status"], "pending")

# 6. A lease written before lineages existed still loads and authorises.
legacy = json.loads(lease.read_text(encoding="utf-8"))
legacy.pop("ownerLineage")
lease.write_text(json.dumps(legacy), encoding="utf-8")
loaded = wl.load_lease(root)
check("a pre-change lease still loads", loaded["ownerLineage"], loaded["holderMainId"])

if failures:
    for line in failures:
        print(f"lease succession fixture failed: {line}", file=sys.stderr)
    raise SystemExit(1)
PY

# The lineage is DERIVED, not concatenated. A mission id has no length bound, so
# concatenation would overflow the 128-character lineage on a long id and the
# mission could not arm; truncation would let two missions share a lineage and
# silently turn a foreign takeover into a renewal.
python3 - <<'PY'
import hashlib, re, sys

def lineage(mission):
    return "mission-" + hashlib.sha256(mission.encode("utf-8")).hexdigest()[:32]

pattern = re.compile(r"[A-Za-z0-9._-]{1,128}")
failures = []
long_id = "bm-2-" + "x" * 109
if not pattern.fullmatch(lineage(long_id)):
    failures.append("a 114-character mission id must still produce a valid lineage")
if len("mission-runner-" + long_id) <= 128:
    failures.append("the long-id case no longer exercises the overflow it guards")
first = "cohort-" + "a" * 60 + "-rep1"
second = "cohort-" + "a" * 60 + "-rep2"
if lineage(first) == lineage(second):
    failures.append("mission ids sharing a long prefix must not share a lineage")
if lineage("bm-2") != lineage("bm-2"):
    failures.append("a lineage must be deterministic for one mission id")
for line in failures:
    print(f"lease succession fixture failed: {line}", file=sys.stderr)
raise SystemExit(1 if failures else 0)
PY

# A lineage is immutable once set for a process: absent-to-present is the only
# transition, and a conflicting one is refused rather than silently preferred.
checkout="$tmp/announce"
mkdir -p "$checkout"
git -C "$checkout" init -q .
start=$(python3 -c "
import importlib.util, pathlib, sys
spec = importlib.util.spec_from_file_location('wl', '$helper')
wl = importlib.util.module_from_spec(spec); spec.loader.exec_module(wl)
print(wl.started_at(pathlib.Path('$checkout'), $$))")

announce() { python3 "$helper" --root "$checkout" announce --session mission-runner-bm-2 \
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
start=\$(python3 -c "
import importlib.util, pathlib
spec = importlib.util.spec_from_file_location('wl', '$helper')
wl = importlib.util.module_from_spec(spec); spec.loader.exec_module(wl)
print(wl.started_at(pathlib.Path('$turns'), \$\$))")
python3 "$helper" --root "$turns" announce --session "host-\$\$" --pid \$\$ \
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

# The wiring itself: if launch_host stops exporting the lineage, host turns go
# back to taking the lease from each other and this whole fix is inert.
grep -Fq 'METASYSTEM_OWNER_LINEAGE": mission_lineage(mission)' "$root/scripts/agents/mission-runner.sh" || {
  echo "lease succession fixture failed: launch_host no longer exports the mission lineage" >&2
  exit 1
}

echo "lease succession fixtures: PASSED"
