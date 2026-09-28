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

# Sections 1-3 (the shell emitter's caller harmlessness, concurrent writers,
# the torn fragment) retired with scripts/agents/emit-event.sh
# (verbs-object-action U6b): every emitter is the Go one, and
# internal/events/emit_writers_test.go proves the same properties as
# TestEmitNeverFailsItsCallerOnAnUnwritableStream,
# TestConcurrentWritersKeepEveryEventParseableAndEachSequenceGapless and
# TestATornFragmentDoesNotPoisonTheNextEvent.

# Sections 4 (oversize degradation), 5 (registry conformance) and the
# FRCC-001/FRCC-002 door-and-cap legs retired to the go gate
# (script-fixtures-011): emit_event is a thin wrapper over `event emit`,
# and internal/events/emit_test.go proves the same properties as
# TestEmitWritesRegisteredEvent, TestEmitDropsUnregisteredEventAndWrong-
# Emitter, TestEmitHonorsHardCap, and TestEmitShrinksOptionalFieldsUnder-
# Cap. What stays here needs a real lease: the two witness-not-authority
# legs below.

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
