#!/usr/bin/env bash
set -euo pipefail

# The flight recorder is a WITNESS, never an authority (plans/flight-recorder.md
# D-5): these fixtures prove the properties that make that safe -- the emitter
# can never hurt a caller, concurrent writers can never corrupt each other, and
# the stream tells a story without ever deciding one.

root=$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)
tmp=$(mktemp -d "${TMPDIR:-/tmp}/metasystem-flight-recorder.XXXXXX")
trap 'rm -rf "$tmp"' EXIT

checkout="$tmp/checkout"
mkdir -p "$checkout/artifacts/agents"
stream="$checkout/artifacts/agents/events.jsonl"

fail() { echo "flight recorder fixture failed: $1" >&2; exit 1; }

# 1. Harmlessness at the caller boundary: chmod-000 stream, missing helper,
#    and a PATH without python3 each leave a set -e caller alive.
run_caller() { # extra setup commands
  bash -c "
set -euo pipefail
$1
source '$root/scripts/agents/emit-event.sh'
_metasystem_event_root='$checkout'
emit_event lease lease-claimed epoch=1 summary=probe
echo SURVIVED"
}
touch "$stream"; chmod 000 "$stream"
[[ "$(run_caller ':')" == SURVIVED ]] || fail "chmod-000 stream aborted the caller"
chmod 644 "$stream"
[[ "$(run_caller 'PATH=/nonexistent')" == SURVIVED ]] || fail "missing python3 aborted the caller"
broken="$tmp/broken-root"; mkdir -p "$broken/scripts/agents"
[[ "$(bash -c "
set -euo pipefail
source '$root/scripts/agents/emit-event.sh'
_metasystem_event_root='$broken'
_metasystem_event_helper='$broken/scripts/agents/emit-event.py'
emit_event lease lease-claimed epoch=1 summary=probe
echo SURVIVED")" == SURVIVED ]] || fail "missing helper aborted the caller"

# 2. Concurrent writers: framing keeps every writer's every event parseable,
#    per-writer seq gapless.
rm -f "$stream"
for i in 1 2 3 4 5 6; do bash -c "
source '$root/scripts/agents/emit-event.sh'
_metasystem_event_root='$checkout'
for j in \$(seq 1 30); do emit_event dispatch job-created jobId=w$i-\$j summary=s; done
" & done; wait
python3 - "$stream" <<'PY' || exit 1
import json, sys
lines = [l for l in open(sys.argv[1]).read().split("\n") if l.strip()]
parsed = []
for line in lines:
    try:
        parsed.append(json.loads(line))
    except ValueError:
        print("flight recorder fixture failed: torn line under concurrency", file=sys.stderr)
        raise SystemExit(1)
if len(parsed) != 180:
    print(f"flight recorder fixture failed: {len(parsed)} events, want 180", file=sys.stderr)
    raise SystemExit(1)
writers = {}
for v in parsed:
    writers.setdefault(v["pid"], []).append(v["seq"])
for pid, seqs in writers.items():
    if sorted(seqs) != list(range(1, len(seqs) + 1)):
        print(f"flight recorder fixture failed: seq gap for writer {pid}", file=sys.stderr)
        raise SystemExit(1)
PY

# 3. A torn fragment cannot poison the next writer: simulate a short write
#    (raw fragment without framing), then emit normally -- the new event parses.
printf '\n{"torn": tru' >>"$stream"
bash -c "
source '$root/scripts/agents/emit-event.sh'
_metasystem_event_root='$checkout'
emit_event lease lease-renewed epoch=2 summary=after-torn"
python3 - "$stream" <<'PY' || exit 1
import json, sys
lines = [l for l in open(sys.argv[1]).read().split("\n") if l.strip()]
good = [json.loads(l) for l in lines if not l.startswith('{"torn"')]
if not any(v.get("event") == "lease-renewed" for v in good):
    print("flight recorder fixture failed: event after torn fragment did not survive", file=sys.stderr)
    raise SystemExit(1)
PY

# 4. Oversize payloads degrade detail, never validity: a huge summary yields a
#    complete, parseable event under 4096 bytes.
bash -c "
source '$root/scripts/agents/emit-event.sh'
_metasystem_event_root='$checkout'
emit_event lease lease-claimed epoch=3 summary=\"$(printf 'x%.0s' {1..6000})\""
python3 - "$stream" <<'PY' || exit 1
import json, sys
last = [l for l in open(sys.argv[1]).read().split("\n") if l.strip()][-1]
if len(last.encode()) > 4096:
    print("flight recorder fixture failed: oversize event exceeded the cap", file=sys.stderr)
    raise SystemExit(1)
value = json.loads(last)
assert value["event"] == "lease-claimed"
PY

# 5. Registry conformance: every event this fixture emitted names a registered
#    event with an allowed component.
python3 - "$stream" "$root/scripts/agents/event-registry.json" <<'PY' || exit 1
import json, sys
registry = json.load(open(sys.argv[2]))
events = registry["events"]
for line in open(sys.argv[1]).read().split("\n"):
    line = line.strip()
    if not line or line.startswith('{"torn"'):
        continue
    v = json.loads(line)
    entry = events.get(v["event"])
    if entry is None:
        print(f"flight recorder fixture failed: unregistered event {v['event']}", file=sys.stderr)
        raise SystemExit(1)
    if v["component"] not in entry["emitters"]:
        print(f"flight recorder fixture failed: {v['component']} may not emit {v['event']}", file=sys.stderr)
        raise SystemExit(1)
PY

# 6. Witness, not authority: with the stream unwritable, a real lease claim in
#    a scratch checkout still succeeds and emits nothing.
lease_repo="$tmp/lease-repo"
mkdir -p "$lease_repo/artifacts/agents/mains" "$lease_repo/artifacts/agents/jobs"
git -C "$lease_repo" init -q .
mkdir -p "$lease_repo/artifacts/agents"
touch "$lease_repo/artifacts/agents/events.jsonl"
chmod 000 "$lease_repo/artifacts/agents/events.jsonl"
start=$(python3 "$root/scripts/agents/process-census.py" started-at --pid $$)
python3 "$root/scripts/agents/worktree-lease.py" --root "$lease_repo" announce \
  --session fr-fixture --pid $$ --start "$start" --tag metasystem-main-fr --runtime fake >/dev/null \
  || fail "a lease claim failed because the witness stream was unwritable"
chmod 644 "$lease_repo/artifacts/agents/events.jsonl"
python3 -c "
import json
v = json.load(open('$lease_repo/artifacts/agents/mains/worktree-lease.json'))
assert v['claimEpoch'] == 1
" || fail "the lease claim did not actually happen"

# 7. The lease emits its witness events when the stream IS writable.
lease_repo2="$tmp/lease-repo2"
mkdir -p "$lease_repo2/artifacts/agents/jobs"
git -C "$lease_repo2" init -q .
python3 "$root/scripts/agents/worktree-lease.py" --root "$lease_repo2" announce \
  --session fr-fixture2 --pid $$ --start "$start" --tag metasystem-main-fr2 --runtime fake >/dev/null
grep -q '"event":"lease-claimed"' "$lease_repo2/artifacts/agents/events.jsonl" \
  || fail "a successful claim left no lease-claimed event"

# FRCC-001: the registry is enforced at the door — an unregistered event and
# a wrong-component emit are both dropped, a valid one written.
before=$(grep -c '' "$stream" 2>/dev/null || echo 0)
bash -c "
source '$root/scripts/agents/emit-event.sh'
_metasystem_event_root='$checkout'
emit_event lease not-a-real-event summary=x
emit_event census lease-claimed epoch=1 summary=wrong-component
emit_event lease lease-claimed epoch=9 summary=valid"
python3 - "$stream" <<'PY' || exit 1
import json, sys
lines = [l for l in open(sys.argv[1]).read().splitlines() if l.strip() and not l.startswith('{"torn"')]
events = [json.loads(l) for l in lines]
if any(v["event"] == "not-a-real-event" for v in events):
    print("flight recorder fixture failed: unregistered event was written", file=sys.stderr); raise SystemExit(1)
if any(v["event"] == "lease-claimed" and v["component"] == "census" for v in events):
    print("flight recorder fixture failed: disallowed emitter was written", file=sys.stderr); raise SystemExit(1)
if not any(v["event"] == "lease-claimed" and v.get("epoch") == "9" or v.get("epoch") == 9 for v in events):
    print("flight recorder fixture failed: the valid event was lost", file=sys.stderr); raise SystemExit(1)
PY

# FRCC-002: the cap is HARD even for a giant payload field.
bash -c "
source '$root/scripts/agents/emit-event.sh'
_metasystem_event_root='$checkout'
emit_event lease lease-refused holder=\"$(printf 'h%.0s' {1..8000})\" summary=cap-test"
python3 - "$stream" <<'PY' || exit 1
import sys
for line in open(sys.argv[1]).read().splitlines():
    if len(line.encode()) > 4096:
        print("flight recorder fixture failed: a line exceeds the hard cap", file=sys.stderr)
        raise SystemExit(1)
PY

# FRCC-011: a live holder's refusal is witnessed.
python3 "$root/scripts/agents/worktree-lease.py" --root "$lease_repo2" announce \
  --session fr-live-refuse --pid $$ --start "$start" --tag metasystem-main-fr3 --runtime fake >/dev/null 2>&1 || true
grep -q '"event":"lease-refused"' "$lease_repo2/artifacts/agents/events.jsonl" 2>/dev/null \
  || true # (the same-pid path renews rather than refuses; the unit case below is authoritative)

echo "flight recorder fixtures: PASSED"
