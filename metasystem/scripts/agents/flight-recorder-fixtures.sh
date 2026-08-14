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
mkdir -p "$checkout/artifacts/agents" "$checkout/scripts/agents"
stream="$checkout/artifacts/agents/events.jsonl"
# The engine reads the registry from the EVENT ROOT (an absent registry must
# not silence the witness, so it admits everything). FRCC-001 below proves
# enforcement, which therefore needs the real registry inside this checkout.
cp "$root/scripts/agents/event-registry.json" "$checkout/scripts/agents/"

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
[[ "$(run_caller 'PATH=/nonexistent')" == SURVIVED ]] || fail "a broken PATH aborted the caller"
broken="$tmp/broken-root"; mkdir -p "$broken/scripts/agents"
[[ "$(bash -c "
set -euo pipefail
source '$root/scripts/agents/emit-event.sh'
_metasystem_event_root='$broken'
_metasystem_event_bin='$broken/bin/metasystem'
emit_event lease lease-claimed epoch=1 summary=probe
echo SURVIVED")" == SURVIVED ]] || fail "a missing engine binary aborted the caller"

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

# Sections 4 (oversize degradation), 5 (registry conformance) and the
# FRCC-001/FRCC-002 door-and-cap legs retired to the go gate
# (script-fixtures-011): emit_event is a thin wrapper over `event emit`,
# and internal/events/emit_test.go proves the same properties as
# TestEmitWritesRegisteredEvent, TestEmitDropsUnregisteredEventAndWrong-
# Emitter, TestEmitHonorsHardCap, and TestEmitShrinksOptionalFieldsUnder-
# Cap. What stays here needs real processes: caller harmlessness under
# set -e, concurrent writers, the torn fragment, and the two
# witness-not-authority lease legs below.

# 6. Witness, not authority: with the stream unwritable, a real lease claim in
#    a scratch checkout still succeeds and emits nothing.
lease_repo="$tmp/lease-repo"
mkdir -p "$lease_repo/artifacts/agents/mains" "$lease_repo/artifacts/agents/jobs"
git -C "$lease_repo" init -q -b main .
mkdir -p "$lease_repo/artifacts/agents"
touch "$lease_repo/artifacts/agents/events.jsonl"
chmod 000 "$lease_repo/artifacts/agents/events.jsonl"
start=$("$root/bin/metasystem" proc started-at --pid $$)
"$root/bin/metasystem" lease announce --root "$lease_repo" \
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
git -C "$lease_repo2" init -q -b main .
"$root/bin/metasystem" lease announce --root "$lease_repo2" \
  --session fr-fixture2 --pid $$ --start "$start" --tag metasystem-main-fr2 --runtime fake >/dev/null
grep -q '"event":"lease-claimed"' "$lease_repo2/artifacts/agents/events.jsonl" \
  || fail "a successful claim left no lease-claimed event"

# FRCC-011 (a live holder's refusal is witnessed) was vacuous here — both
# its command and its grep ended in || true — and is now a REAL assertion:
# internal/lease/refusals_test.go TestNonHolderAnnounceEmitsLeaseRefused-
# Witness (script-fixtures-010).

echo "flight recorder fixtures: PASSED"
