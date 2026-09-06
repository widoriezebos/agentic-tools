Working Mode: build
Orchestrator Identity: m1d+main-1788683763-71870-f7f607 (dispatch delegate under goal stop-hook-budget-is-ours)
Date: 2026-09-06

# Goal

Goal stop-hook-budget-is-ours, slice 1 of 2 (tier 3, approved by Wido at
his terminal on 2026-09-06). Its record,
metasystem/plans/goals/stop-hook-budget-is-ours.md, is the contract. In
short: the Stop hook's budget is our own number. The five seconds the hook
comment attributes to the provider is the timeout we ship in the runtime
registration; the runtime's own default is sixty. Wido's decision: the
budget is sixty seconds. The fail-closed refusal on a genuine hang stays
and simply comes at sixty. And because nothing records how long a Stop
takes, every Stop measures itself from now on: the elapsed seconds since
the deadline parent started land in the hook evidence trail and in the
steward component record, so slice 2 (a later chain, not this one) can
make the steward flag a slowing hook before it costs turns.

# The change

1. Registration templates. In
   metasystem/scripts/enforcement/claude-code-hooks.json the Stop entry
   that runs the supervision hook changes `"timeout": 5` to
   `"timeout": 60`. The same change in
   metasystem/scripts/enforcement/codex-hooks.json and
   metasystem/scripts/enforcement/devin-hooks.json: the deadline parent
   below is shared by every registered runtime, so the three Stop
   registrations move together. The SessionStart (15) and SessionEnd (3)
   entries stay as they are. The `_comment` text stays.

2. The deadline parent in metasystem/scripts/agents/supervision-hook.sh
   (the block guarded by `"$event" == stop` and
   METASYSTEM_STOP_DEADLINE_PARENT):
   - The comment above it says the truth: sixty seconds is the Stop
     timeout we ship in the registration templates under
     metasystem/scripts/enforcement, installed into the runtime's live
     settings by adopt.sh; it is also the runtime's own default; the
     parent gives the worker fifty-seven of those seconds and keeps three
     to emit the refusal. The number is ours and this is where it lives.
   - Two named variables at the top of the block replace the bare `4`:
     `deadline_budget_sec=60` and `deadline_worker_sec=$((deadline_budget_sec - 3))`;
     `deadline_expires` is computed from `deadline_worker_sec`.
   - The parent records its own start as whole epoch seconds
     (`date -u +%s`; macOS ships bash 3.2, which has no EPOCHREALTIME) in
     `deadline_started_epoch`, and hands it to the worker on the worker's
     command line as `METASYSTEM_STOP_DEADLINE_STARTED=<epoch seconds>`,
     next to the existing METASYSTEM_STOP_DEADLINE_PARENT assignment.
   - `deadline_log_stop_outcome` appends ` elapsed=<n>s` to its
     `stop response outcome=` line, where n is now minus
     `deadline_started_epoch`, never negative.

3. The worker (the same script, everything after the deadline-parent
   block):
   - Right after the deadline-parent block, before the engine is resolved,
     set `stop_started_epoch` from METASYSTEM_STOP_DEADLINE_STARTED when
     that variable is all digits, else from `date -u +%s` (a worker run
     without a parent measures itself from its own start).
   - `emit_stop_payload` computes `stop_elapsed_sec` (now minus
     `stop_started_epoch`, clamped at zero) once, and appends
     ` elapsed=${stop_elapsed_sec}s` to the `stop response decision=` line
     it writes to the hooks log, so the line reads
     `<timestamp> stop response decision=allow elapsed=3s`.
   - Every `steward hook-complete` call inside `emit_stop_payload` (four
     call sites: two PAYLOAD_STAGE_FAILED, one EMISSION_FAILED, one
     EMITTED) passes `--elapsed-sec "$stop_elapsed_sec"`.

4. The engine, metasystem/internal/steward/component_evidence.go and
   metasystem/cmd/metasystem/steward_verbs.go:
   - `ComponentEvidence` gains `LastStopElapsedSec int64` with json tag
     `lastStopElapsedSec,omitempty`; `ComponentAttemptHistory` gains
     `StopElapsedSec int64` with json tag `stopElapsedSec,omitempty`.
   - `CompleteHookAttempt` takes one more parameter, `stopElapsedSec int64`,
     refuses a negative value with an error, and the completion writes it
     to the record's `LastStopElapsedSec` and to the history entry the
     completion appends. Thread it through `completeComponentAttempt` and
     `appendAttemptHistory` in whatever way keeps the other components'
     callers passing zero; zero means unmeasured and is omitted from the
     JSON.
   - `loadComponentEvidence` treats a negative `LastStopElapsedSec` or a
     negative history `StopElapsedSec` as malformed, like it does the
     other numeric fields.
   - `steward hook-complete` gains an OPTIONAL integer flag `--elapsed-sec`
     (default 0). Optional, not required: a seat can run a freshly pulled
     hook script against a not yet rebuilt engine, or the reverse, and
     neither pairing may break hook completion. A negative value is a
     usage error (exit 2).
   - Unit tests in the steward package: a hook completion with elapsed 7
     stores 7 on the record and on its history entry; a negative elapsed is
     refused; a record written without the field still loads; the
     existing tests stay green.

5. The fixtures, metasystem/scripts/agents/supervision-hook-fixtures.sh:
   - The deadline scenario (the delaying engine wrapper whose
     `runtime list` sleeps 4.5 seconds): the sleep becomes 57.5, over the
     worker's fifty-seven by the same half second it is over four today
     and under the provider's sixty; the elapsed assertion `< 5` becomes
     `< 60` and its message names the provider's sixty-second budget; the
     block, cause, occurrence and remedy assertions stay. Add one
     assertion: the hooks log under the scenario's stop root carries a
     `stop response outcome=deadline-expired-block elapsed=<digits>s` line.
     This scenario now takes about two minutes (two hook runs of about
     fifty-eight seconds each); that is the price of the real numbers and
     is accepted.
   - The chat-line scenario (the one whose payload names line_root and
     that reads the supervision-hook component record): after the hook
     completes, assert that the hooks log carries a line matching
     `stop response decision=[a-z]+ elapsed=[0-9]+s`, and that the
     component record carries `"lastStopElapsedSec": <digits>`. That run
     goes through the deadline parent (the fixture invokes the hook
     without METASYSTEM_STOP_DEADLINE_PARENT), so the number is measured
     from the parent's start.

Nothing else changes. In particular the deadline refusal text, the
stop-refusal record, the evidence-gc step and the health preview are
out of scope (goal stop-hook-health-cost owns the hook's cost).

# Why the template change alone does not switch the budget on

The live settings file the runtime actually reads sits at the repository
root, generated from the claude template by adopt.sh, and is outside this
chain: the conformance review refuses any path outside metasystem. Until
that live file carries 60 on a host, a worker allowed fifty-seven seconds
is killed by the runtime at five, and a timed-out Stop hook is
non-blocking, so the fail-closed refusal would be lost. The orchestrator
lands this chain only after the live file is at sixty on main. You do not
touch the live file; you build the change above.

# Gate

`cd metasystem && go build ./... && go vet ./... && gofmt -l .` (empty);
`go test ./internal/steward/ -count=1` green;
`bash -n scripts/agents/supervision-hook.sh scripts/agents/supervision-hook-fixtures.sh`;
`bash scripts/agents/supervision-hook-fixtures.sh` green (say so if it
cannot run in the sandbox; the orchestrator reruns it at the landing).

# Constraints

Wall-clock budget: 60 minutes; return before it ends even if something is
red, naming it. Declare the boundary as every file that differs from
main. Never touch plans. Gap rule: stop and report a gap with your
proposed contract written out.

# Expected Return

The implementer return per the role schema: `riskiestPart` first,
`diffBoundary` listing every touched path relative to the repository root
(so each starts with `metasystem/`), `whatWasDone`, `gaps`, and `evidence`
entries in the settled `{command, observed, level}` shape, one replayable
command each from the declared workspace: the go build, vet and gofmt
line, the steward package test, and the fixture suite run with its exit
status and wall time.

# Acceptance Criteria

- The three Stop registrations under metasystem/scripts/enforcement carry
  `"timeout": 60`; SessionStart and SessionEnd are unchanged.
- The deadline parent expires the worker at fifty-seven seconds after its
  own start and says so in its comment; the bare `4` is gone.
- A Stop that passes through the parent leaves a
  `stop response decision=<decision> elapsed=<n>s` line in the hooks log
  and `lastStopElapsedSec` on the supervision-hook component record; an
  expired Stop leaves a `stop response outcome=deadline-expired-block elapsed=<n>s` line.
- `steward hook-complete` accepts `--elapsed-sec`, works without it, and
  refuses a negative.
- The supervision-hook fixture suite passes with the deadline scenario at
  the new numbers and the two new assertions.

# Gap Rule

stop and report a gap; never fill it silently.
