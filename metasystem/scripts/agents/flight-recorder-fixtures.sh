#!/usr/bin/env bash
set -euo pipefail

# The flight recorder is a WITNESS, never an authority (records/misc/flight-recorder.md
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
#    and a broken PATH each leave a set -e caller alive.
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
event_count=0
: >"$tmp/writer-seqs"
while IFS= read -r event_line || [[ -n "$event_line" ]]; do
  [[ -n "${event_line//[[:space:]]/}" ]] || continue
  "$root/bin/metasystem" util json-validate --value "$event_line" \
    || fail "torn line under concurrency"
  event_pid=$("$root/bin/metasystem" json get --value "$event_line" --field pid) \
    || fail "an event carries no writer pid"
  event_seq=$("$root/bin/metasystem" json get --value "$event_line" --field seq) \
    || fail "an event carries no seq"
  printf '%s %s\n' "$event_pid" "$event_seq" >>"$tmp/writer-seqs"
  event_count=$((event_count + 1))
done <"$stream"
[[ $event_count -eq 180 ]] || fail "$event_count events, want 180"
for writer_pid in $(awk '{print $1}' "$tmp/writer-seqs" | sort -u); do
  awk -v pid="$writer_pid" '$1 == pid {print $2}' "$tmp/writer-seqs" | sort -n >"$tmp/writer-got"
  seq 1 "$(($(wc -l <"$tmp/writer-got")))" >"$tmp/writer-want"
  cmp -s "$tmp/writer-got" "$tmp/writer-want" || fail "seq gap for writer $writer_pid"
done

# 3. A torn fragment cannot poison the next writer: simulate a short write
#    (raw fragment without framing), then emit normally -- the new event parses.
printf '\n{"torn": tru' >>"$stream"
bash -c "
source '$root/scripts/agents/emit-event.sh'
_metasystem_event_root='$checkout'
emit_event lease lease-renewed epoch=2 summary=after-torn"
renewed_seen=0
while IFS= read -r event_line || [[ -n "$event_line" ]]; do
  [[ -n "${event_line//[[:space:]]/}" ]] || continue
  [[ "$event_line" == '{"torn"'* ]] && continue
  "$root/bin/metasystem" util json-validate --value "$event_line" \
    || fail "a healthy line failed to parse after the torn fragment"
  if [[ "$("$root/bin/metasystem" json get --value "$event_line" --field event --default '')" == lease-renewed ]]; then
    renewed_seen=1
  fi
done <"$stream"
[[ $renewed_seen -eq 1 ]] || fail "event after torn fragment did not survive"

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
[[ "$("$root/bin/metasystem" json get --file "$lease_repo/artifacts/agents/mains/worktree-lease.json" --field claimEpoch)" == 1 ]] \
  || fail "the lease claim did not actually happen"

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
