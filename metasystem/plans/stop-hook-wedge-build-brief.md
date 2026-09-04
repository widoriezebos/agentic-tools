Working Mode: build
Orchestrator Identity: m2+main-1788441779-14484-82d6ed (dispatch delegate under goal stop-hook-wedge-on-enrollment-drift)
Date: 2026-09-04

# Goal

Goal stop-hook-wedge-on-enrollment-drift (tier 2, approved by Wido's
word "you are allowed to fix the hook problem now"). The harness Stop
hook, metasystem/scripts/agents/supervision-hook.sh, refuses every turn
end whenever it cannot prove within its deadline that stopping is
safe. Two causes are not the seat's to fix and today re-prompt the seat
forever: (a) `metasystem up` failing with a human-only remedy
(ENROLLMENT_DRIFT, or a steward runner that cannot complete a tick),
surfaced as "supervision arming failed"; (b) the hook's own 4-second
deadline expiring ("Stop deadline expired before a safe turn
verdict"), which on a loaded machine happens on every turn. Specimens:
m2 on 2026-09-03 and 2026-09-04, hundreds of refusals over ten hours,
Wido: "THIS NEEDS TO DIE NOW".

# The change

1. A refusal whose cause is NOT the seat's blocks AT MOST ONCE PER
   CAUSE PER SESSION. The hook keeps a durable per-session record
   `artifacts/agents/supervision/stop-refusals/<session slug>.json`
   with, per cause digest: the cause text, the count, first and last
   time. On a refusal it consults the record: first time for this
   cause, emit the block as today AND record it; a repeat, emit a
   systemMessage (not a block) that names the cause, the remedy and
   the count, so the turn ends. The record is runtime class (never
   landed). Causes covered by this rule: every `record_stop_failure`
   diagnostic whose remedy is outside the seat (the arming failure,
   the health engine returning no verdict, the watchdog state
   unreadable, the lease not renewable) and the deadline-parent path.
2. The deadline parent (the 4-second supervisor at the top of the
   script) applies the same rule: on overrun it records the cause
   "stop deadline expired" and blocks only the first time per session;
   afterwards it surfaces a systemMessage and lets the turn end. Keep
   the overrun handling otherwise unchanged.
3. The "work named in a plan is unblocked and nothing is in flight"
   refusal (the report stop-block verb's open-work case) is the seat's
   own and keeps blocking exactly as today; it already says it does not
   repeat for the same work.
4. `metasystem report stop-block` (cmd/metasystem/report.go,
   internal/report) gains what the hook needs if the JSON shape must
   carry a `surfaceOnly` decision; otherwise leave the engine alone.
5. Fixtures in metasystem/scripts/agents/supervision-hook-fixtures.sh:
   an arming failure blocks once and surfaces on the second stop of the
   same session; a deadline overrun blocks once and surfaces on the
   second; a different cause blocks again; the open-work refusal blocks
   on every stop; a new session starts fresh. Use the fake runtime and
   the existing fixture harness; no real steward.

# Gate

`bash metasystem/scripts/agents/supervision-hook-fixtures.sh` green;
`bash -n` on the hook; `cd metasystem && go build ./... && go test ./internal/report/ ./cmd/metasystem/ -run 'Stop|Report' -count=1`
green if the engine is touched.

# Constraints

Wall-clock budget: 60 minutes; return before it ends even if something
is red, naming it. MECHANICAL reach (tier 2: mechanical logic inside
the existing hook). Declare the boundary as every file that differs
from main, with the metasystem/ prefix. Gap rule: stop and report a
gap with your proposed contract written out, so the answer is one word.
