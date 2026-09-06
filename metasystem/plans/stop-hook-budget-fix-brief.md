Working Mode: implement
Orchestrator Identity: m1d+main-1788683763-71870-f7f607 (dispatch delegate under goal stop-hook-budget-is-ours)
Date: 2026-09-06

# Goal

Follow-up round of slice 1 of goal stop-hook-budget-is-ours (chain
shbo-build1-20260906). Your round-1 return stopped on one real gap in
the brief: a Stop that starts and completes within the same epoch second
measures zero, zero was declared "unmeasured and omitted", and the
chat-line fixture asserts the field on every run. The gap is accepted;
the contract is corrected below. Your round-1 brief
(metasystem/plans/stop-hook-budget-build-brief.md) still governs
everything it says; where this brief differs, this brief wins. No
fixture-only delay wrapper: the fixture must hold for a fast Stop.

# Workspace

Your existing worktree, uncommitted as you left it. Do not stage or
commit; the seat lands the chain.

# The correction: absent means unmeasured, zero is a measurement

1. In metasystem/internal/steward/component_evidence.go the two new
   fields become pointers: `LastStopElapsedSec *int64` with json tag
   `lastStopElapsedSec,omitempty` on ComponentEvidence, and
   `StopElapsedSec *int64` with json tag `stopElapsedSec,omitempty` on
   ComponentAttemptHistory. A nil pointer is omitted from the JSON and
   means no measurement was supplied; a non-nil zero is written as `0`
   and means the Stop completed within the second it started.
2. `CompleteHookAttempt` takes `stopElapsedSec *int64`; nil when the
   caller has no measurement; a non-nil negative is refused with an
   error as today. Thread the pointer through `completeComponentAttempt`
   and `appendAttemptHistory`; `CompleteComponentAttempt` and the
   INTERRUPTED_BY_NEXT_TURN closure pass nil, so other components and
   interrupted turns never carry the field.
3. `loadComponentEvidence` treats a non-nil negative on the record or on
   a history entry as malformed; nil is valid.
4. In metasystem/cmd/metasystem/steward_verbs.go `steward hook-complete`
   keeps `--elapsed-sec` optional: when the flag is absent the verb
   passes nil; when present, a value below zero is a usage error (exit
   2) and any other value is passed as a measurement, zero included.
   Detect presence rather than relying on the default value (for
   example by visiting the parsed flags, or a string flag parsed when
   non-empty); say in the return which mechanism you chose.
5. The hook script does not change: it always passes the flag with the
   clamped whole-second value, so a fast Stop records `0`.
6. Unit tests in the steward package follow: a completion with nil
   writes no field; a completion with zero writes `"lastStopElapsedSec": 0`
   and a history entry with `"stopElapsedSec": 0`; a completion with 7
   writes 7 on both; a negative is refused; a record written without the
   field loads with a nil pointer.
7. The chat-line fixture assertion in
   metasystem/scripts/agents/supervision-hook-fixtures.sh stays as the
   round-1 brief asked: the component record carries
   `"lastStopElapsedSec": <digits>`, zero included, and the hooks log
   line carries `elapsed=<digits>s`. Keep the deadline-scenario
   assertions from round 1.

# Gate

`cd metasystem && go build ./... && go vet ./... && gofmt -l .` (empty);
`go test ./internal/steward/ -count=1` green;
`bash -n scripts/agents/supervision-hook.sh scripts/agents/supervision-hook-fixtures.sh`;
`bash scripts/agents/supervision-hook-fixtures.sh` green, run to the end
this time; it now takes a couple of minutes longer because of the
deadline scenario. Report its exit status and wall time in evidence.

# Constraints

Wall-clock budget: 45 minutes; return before it ends even if something
is red, naming it. Declare the boundary as every file that differs from
main. Gap rule: stop and report a gap with your proposed contract
written out.

# Expected Return

Version-2 implementer JSON as in round 1.

# Gap Rule

stop and report a gap; never fill it silently.
