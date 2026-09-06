Working Mode: implement
Orchestrator Identity: m1c (lineage main-1788680061-17829-64951c, dispatch delegate under goal stop-hook-refusal-carries-verdict, tier 3, hazard DESIGN-BEARING)
Date: 2026-09-06

# Goal

The Stop hook's refusal must carry the turn verdict. Today the block a
person sees at a turn end reads "Work named in a plan is unblocked and
nothing is in flight ... Metasystem could not prove that stopping is
safe: supervision arming failed" and names neither the open step nor,
in its reason, anything a person can act on. The supervision fixture's
stop-hook-monitor scenario in
metasystem/scripts/agents/supervision-fixtures.sh pins the lawful shape
and is red. Observed on m1 today (2026-09-06, main at 36b6a55e, the
scenario run with a bash-3.2 form of its BASHPID line): the scenario's
first block is

    {"decision":"block","reason":"Work named in a plan is unblocked and nothing is in flight. Do it now, or record in the plan why it is blocked or waiting on the human. This refusal does not repeat for the same work.\n\nMetasystem could not prove that stopping is safe: supervision arming failed","systemMessage":"HEALTH unhealthy — steward-runner=dead (...)"}

and the scenario fails at "the block reason does not carry the verdict
display". The block came from the hook's early failed-stop path in
metasystem/scripts/agents/supervision-hook.sh (emit_failed_stop, taken
when arming, health, digest, lease, watchdog or evidence recorded a
stop failure), which exits before the turn verdict ever runs, so no
OPEN WORK display exists to carry. The health line does reach the
system message on that path.

When you are done, every block the hook emits carries the verdict
display first, then any rule sentence, the one-line HEALTH verdict is
in the system message, the deadline sentence appears only when the
deadline actually expired, and the stop-hook-monitor scenario passes on
a Mac with the stock bash 3.2.

# The law, and where each part lives

The envelope law is design point GOAL-05 in
metasystem/records/goal-system/goal-system-design.md (the block reason
is the verdict display byte-verbatim; watchdog and protocol text stay
in the non-blocking channel) and GOAL-04 (block-once; the verdict verb
owns the decision). The goal's intent adds: display first, then any
rule sentence; HEALTH in the system message; the deadline sentence only
on a real expiry.

1. Reason order, in the engine. `StopBlock` in
   metasystem/internal/report/stopblock.go composes the reason as the
   fixed guidance sentence followed by the caller's detail. Reverse it:
   the detail first, then one blank line, then the guidance; with an
   empty detail the reason is the guidance alone with no leading blank
   line. `StopRefusal` in the same file reuses `StopBlock` and needs no
   separate change. The verb wiring in
   metasystem/cmd/metasystem/report.go stays as it is. The two tests
   that pin the old order, `TestStopBlock` ("reason must append the
   caller detail") and `TestStopBlockEmptyDetail` in
   metasystem/internal/report/stopblock_test.go, change to pin the new
   order: detail first, guidance last, no leading separator when the
   detail is empty. That is the law changing under the design point,
   not a weakened test; every other assertion in that file stays.

2. The verdict runs before any block, in the hook. In
   metasystem/scripts/agents/supervision-hook.sh the Stop body records
   stop failures (record_stop_failure) through the arming, health,
   digest, lease, watchdog and evidence stages and then, at "if
   [[ -n "$stop_failure" ]]", leaves through emit_failed_stop before the
   turn verdict. Change the order so the turn verdict
   (`report turn-verdict`) runs first on every Stop that reaches the
   body, the advisor early exit (OWNED-ELSEWHERE) excepted, and the
   stop-failure path composes with the verdict's display:

   - If the verdict verb fails or returns an unreadable shape, the
     existing degraded path stands unchanged (the fixed
     "turn-verdict unavailable:" message; never an all-clear).
   - With a readable verdict and a recorded stop failure, the block is
     produced through the existing refusal record
     (`report stop-block --refusal-record ...`, block-once per cause)
     with the detail set to the verdict display, one blank line, then
     "Metasystem could not prove that stopping is safe: <cause>", and
     the system message set to the check-in tail (the HEALTH line and
     the narrator digest line) as today. When the refusal record says
     the cause is repeated, that verb returns a non-blocking message;
     then the verdict decides: if the verdict says block, emit the
     ordinary verdict block (stop_block_json with the display as its
     detail) and prepend the repeated-failure message to the system
     message; if the verdict does not block, surface the
     repeated-failure message, the display and the check-in tail as
     today's non-blocking envelope does.
   - With a readable verdict and no stop failure, nothing changes.

   Keep the evidence trail writes exactly where they are (one dated
   line per emitted response; that landed today in 1338f2a9). Keep the
   block-once semantics of both mechanisms: the verdict's open-work and
   goal signatures, and the refusal record's per-cause count.

3. The deadline parent preserves a published answer. The same file's
   deadline parent (the top block guarded by
   METASYSTEM_STOP_DEADLINE_PARENT) gives the worker four seconds and,
   when the worker is still running at the deadline, kills it and
   emits "Metasystem Stop deadline expired before a safe turn verdict"
   even when the worker had already written its complete response.
   Before killing, read the worker's stdout: if it holds a complete
   response of the shape the in-time path already validates (a block
   with a string reason and a string system message containing
   "HEALTH ", or a system-message-only allow), emit that response
   verbatim and clean up; emit the deadline sentence only when no such
   response was published. The preserved branch
   preserve/sse-build1-r3 on origin carries a working shape of this
   change (its deadline_published block); take the idea, not the
   branch: that branch also moved the evidence trail and rewrote the
   block composition in the hook, both of which are superseded by
   points 1 and 2 and by 1338f2a9.

4. The fixture runs on the stock Mac bash. Line 1746 of
   metasystem/scripts/agents/supervision-fixtures.sh reads
   `stop_main_pid=$BASHPID`; BASHPID does not exist in bash 3.2, the
   only bash on a stock Mac, and under set -u the scenario dies there
   before its first assertion. Replace it with a form that yields the
   subshell's own pid on bash 3.2 (for example the parent pid seen by a
   child `sh -c 'echo $PPID'`). It is the only bash-4 construct under
   the scripts directory; add no other.

5. Out of scope: the census-lifecycle scenario's failure ("announced
   main pid <empty> outside its scenario bed"). Run it; if points 2 or
   3 change its outcome, say so in whatWasDone with the evidence; do
   not fix it separately. Do not touch the steward, the census, the
   turn-verdict verb's decision logic, or any other scenario.

# Workspace

The job worktree the dispatcher creates for you, branched from main.
May touch: metasystem/internal/report/stopblock.go
May touch: metasystem/internal/report/stopblock_test.go
May touch: metasystem/scripts/agents/supervision-hook.sh
May touch: metasystem/scripts/agents/supervision-fixtures.sh
May touch: metasystem/scripts/agents/supervision-hook-fixtures.sh
Must not touch: anything else; nothing under plans; no other test file.
The fixture files may change only as points 4 and 5 allow and, in
supervision-hook-fixtures.sh, only where an assertion pins the old
reason order and must pin the new one (name each such change).

# Inputs

- metasystem/scripts/agents/supervision-hook.sh: the deadline parent
  (top, guarded by METASYSTEM_STOP_DEADLINE_PARENT), the Stop body's
  stages, record_stop_failure, emit_failed_stop, stop_block_json,
  external_stop_json, the verdict call and the envelope composition.
- metasystem/internal/report/stopblock.go and its test.
- metasystem/cmd/metasystem/report.go: the stop-block verb.
- metasystem/scripts/agents/supervision-fixtures.sh: the
  stop-hook-monitor scenario (S4-14, S4-15, S4-16) and its assertions;
  the census-lifecycle scenario for point 5.
- metasystem/records/goal-system/goal-system-design.md: GOAL-04 and
  GOAL-05.
- The branch preserve/sse-build1-r3 on origin: the earlier attempt's
  deadline_published shape (read only).
- Landing 1338f2a9 on main: the evidence trail on every emission.

# Constraints

- Never weaken a test beyond the two named order assertions, which
  change to the new law. No skips, no relaxed greps.
- The hook stays bash-3.2 clean; no new commands outside the engine
  and the POSIX tools it already uses.
- The engine binary the fixture pins is metasystem/bin/metasystem in
  your worktree; rebuild it with metasystem/scripts/agents/go-build.sh
  after the Go change, before running any fixture.
- Budget: one round, at most 120 minutes of wall clock.
- Hazard class DESIGN-BEARING: the hook envelope and an engine verb's
  contract change; an independent critique follows. Leave nothing for
  it to guess: name every composition choice in whatWasDone.
- The full validation suite is not yours to run; the orchestrator runs
  it. If the supervision fixture suite refuses to run in your sandbox
  (it arms beds and reads live processes), report the refusal verbatim
  as a gap and stop after the Go tests and the syntax checks.

# Expected Return

The implementer role's version-2 JSON. Evidence commands, replayable
from the worktree root (the metasystem directory):

- `go test -count=1 ./internal/report` (expected: ok)
- `go test -race -count=1 ./internal/report ./internal/goal` (expected: ok)
- `bash scripts/agents/go-build.sh` (expected: a rebuilt engine line)
- `bash -n ./scripts/agents/supervision-hook.sh` and `bash -n ./scripts/agents/supervision-fixtures.sh` (expected: exit 0)
- `grep -n 'BASHPID' ./scripts/agents/supervision-fixtures.sh` (expected: no output)
- `bash ./scripts/agents/supervision-hook-fixtures.sh` (expected: passes)
- `bash ./scripts/agents/supervision-fixtures.sh` (expected: stop-hook-monitor passes; report every scenario's line)
- `git diff --stat` (expected: only files under May touch)

Lead riskiestPart with the composition in point 2: the order of the
verdict and the failure stages, and what a Stop that both fails arming
and finds open work emits on its first and second firing.

# Acceptance Criteria

1. `report stop-block "X"` prints a reason that begins with X, then one
   blank line, then the guidance; `report stop-block ""` prints the
   guidance alone; the two named Go tests pin exactly that.
2. In the fixture's stop-hook-monitor scenario, the first block's
   reason begins with "OPEN WORK (1)" and its system message contains
   "HEALTH "; the second firing does not block; every later assertion
   of S4-14, S4-15 and S4-16 passes; the scenario passes under
   /bin/bash version 3.2.
3. The deadline parent emits a worker's complete published response
   when one exists at the deadline, and the sentence "Stop deadline
   expired" only when none does.
4. No file outside May touch changed; no assertion was removed or
   loosened except the two order assertions named in point 1.

# Gap Rule

stop and report a gap; never fill it silently.
