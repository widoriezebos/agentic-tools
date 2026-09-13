# stop-infrastructure-allows-the-seat-to-stop: design

The first member of stop-hook-never-forces-an-empty-turn (its page,
plans/stop-hook-never-forces-an-empty-turn-design.md, section 7). One
mechanism: an infrastructure read failure never refuses a stop. Written by
the m1b seat 2026-09-12; one rostered Codex read, then a Codex sol build in
a worktree, an Opus read, a human commit.

## 1. What refuses today

The Stop hook (`metasystem/scripts/agents/supervision-hook.sh`) reads its
own infrastructure before it renders the turn verdict: the runtime identity
(lines 514 to 549), the turn and attempt evidence (745 to 772), supervision
arming (836), health (842), the narrator digest (855 and 859), the holder
protocol and lease (1037 to 1072), the watchdog and the hook evidence (1082
to 1098). Each failure calls `record_stop_failure` (line 481), which keeps
the first cause in one slot; at line 943 that cause becomes an external
refusal through `report stop-block`. The engine's `report stop-block`
(`metasystem/internal/report/stopblock.go`, lines 121 to 141) keys a
per-session record by the cause text alone, blocks on the first occurrence
of a cause and lets later occurrences through with a notice. The deadline
path (lines 329 to 346) refuses on its own when the verdict worker did not
finish in time. The Claude launcher
(`metasystem/scripts/enforcement/claude-code-hooks.json`, line 25) turns
any nonzero exit of the hook into a raw block with no record at all.

The engine's turn verdict has the same shape inside: state-root, fence,
verdict-state and status-write failures return `failClosedTurnVerdict`
(`metasystem/internal/goal/turnverdict.go`, lines 231 to 250, 338 and 412),
a blocking verdict whose reason is "cannot prove that stopping is safe";
`metasystem/internal/goal/sessionstop.go` (lines 428 to 445, 484 to 501)
does the same for the session-stop marker's read and consume.

Five days of records: "the narrator digest could not be read" 184 times,
"stop deadline expired" 117, "supervision arming failed" about 95. None of
these is a condition the seat can act on. Each cost the seat a forced turn.

## 2. The mechanism

Every refusal source carries a class, decided in the engine and never from
a rendered string:

- **infrastructure**: a read or write of the hook's or the verdict's own
  state failed (every `record_stop_failure` site above, the deadline, every
  `failClosedTurnVerdict` producer, the session-stop marker's read and
  consume, the launcher's bootstrap). The stop is ALLOWED. The response
  carries a degraded notice naming the condition and its owner (the steward
  or a person), one line is appended to the hook log, and nothing else
  happens: no seen marker, no counter, no ALL CLEAR, no authorization.
  This holds on the first occurrence and every one after; the
  first-occurrence block of `report stop-block` goes.
- **seat-actionable**: the turn verdict's own findings (owned work with no
  waiter, an unsurfaced plan line, a changed queue, a stale goal-free
  declaration). Unchanged by this member: they block as today.
- **idle-with-backlog**: approved backlog with no job. Unchanged: it blocks
  the first and second stop and hands off on the third, exactly as
  `metasystem/records/goals/idle-with-backlog-alarm.md` says. When the
  verdict state that spends its count cannot be written, the observed
  condition still refuses, uncounted, and the refusal says the count could
  not be spent; the hook-log line records that refusal first.

Where it lives:

- `report stop-block` gains `--class` (`infrastructure` or
  `seat-actionable`); the infrastructure class returns the allowance with
  the notice and writes the log line; the seat class keeps today's block.
  The hook passes `--class infrastructure` at line 943 for every
  `record_stop_failure` cause and at the deadline path; the deadline's own
  record-failure branch already allows and is kept, and its notice is
  printed by the shell without the engine (the engine is what is most
  often missing when that branch runs).
- `failClosedTurnVerdict` becomes `infrastructureVerdict`: `ShouldBlock`
  false, `LedgerStatus` degraded, the detail kept in `Diagnostics` and
  `Display`, a new `Class` field set to `infrastructure` on the verdict so
  the hook renders it as a notice, not a block: a fixed first line
  (`turn-verdict degraded: stopping is allowed on degraded infrastructure;
  the steward owns repair`, the cause code and the component) over the
  detail, never an all-clear. The idle branch is
  evaluated before the verdict-state write, and a failed write returns the
  idle block with `CountSpent` false and the same class field set to
  `idle-with-backlog`.
- The arming-failure notice carries the failed components exactly as
  `metasystem up` prints them (its `Components` lines with outcome, detail
  and remedy, then the aggregate), never the bare "supervision arming
  failed"; the hook already holds `up_aggregate` at line 835 and gains the
  component lines from the same call.
- The hook log (line 878) gains one line per infrastructure condition and
  per uncounted idle refusal: `stop-condition <class> <cause code>
  <component> <turn generation> <deadline end> <outcome>`, in every
  outcome: the advisor's allowance and the deadline parent's own
  allowances (worker output unreadable, deadline expired) append theirs
  too. The append's exit is checked; a failed append is said in the notice.
- The launcher's fallback becomes a degraded allowance: on a nonzero hook
  exit it prints a systemMessage that names the failure as the hook's own
  and points at the steward, and never a block. The installed Stop line
  the seats run is not the template: `metasystem/internal/hooks/setup.go`
  renders it. The renderer carries the template's fallback tail through
  byte for byte instead of composing one, and recognizes the retired block
  form as this installation's, so `runtime setup` replaces it; the tracked
  `.claude/settings.json` is re-rendered in the landing.

## 3. What does not change

- Seat-actionable refusals and their seen-state limits.
- The idle-with-backlog refusal, its counter, its digest (every nonterminal
  job, as today) and its three-refusals handoff to steward continuation.
- The stop-refusal record's schema and path; only its first-occurrence
  block for the infrastructure class goes. The steward's incident record
  and drain are member stop-incidents-reach-the-steward, not this one.
- The human session-stop authorization: a failed read of its marker allows
  the stop without consuming or inventing an authorization.

## 4. Risks

- A telemetry failure could hide a real refusal if the work decision were
  read after the telemetry: the verdict's work decision is read first and
  the infrastructure reads after it, so a real refusal is never turned into
  an allowance by a later failure.
- An allowance on every infrastructure failure means a seat with broken
  supervision can stop freely; that is the intent (DONE clause 2), and the
  hook-log line plus the steward's notice keep it visible.

## 5. Fixtures

1. `TestStopBlockClassesInfrastructureAllows` in
   `metasystem/internal/report/stopblock_test.go`: `report stop-block` with
   the infrastructure class allows on the first and the fifth occurrence
   with the notice and no block; the seat class blocks as today.
2. `TestInfrastructureVerdictNeverBlocks` in
   `metasystem/internal/goal/turnverdict_test.go`: every former
   `failClosedTurnVerdict` producer (state root, fence, verdict state,
   status write) yields an allowing verdict with the infrastructure class
   and the detail; `metasystem/internal/goal/turnverdict_idle_test.go`
   `TestIdleRefusalSurvivesALostCounter`: approved backlog, no job, the
   verdict-state write faulted: the stop is refused with the idle reason,
   `CountSpent` false, no count recorded.
3. `TestSessionStopInfrastructurePreservesAuthority` in
   `metasystem/internal/goal/turnverdict_idle_test.go`: a marker read failure
   allows without consuming; a consume failure never reports consumption.
   `TestSessionStopConsumeErrorKeepsDecidedBlock` there: a consume failure
   under a decided refusal keeps the block and the marker unspent, and the
   authorization then covers exactly one later quiet stop.
4. `TestArmingDetailSurvivesStop` in `metasystem/cmd/metasystem/up_test.go`:
   an ENROLLMENT_DRIFT arming result reaches the stop notice with its
   component outcome, detail, remedy and aggregate byte for byte.
5. Hook bed legs in `metasystem/scripts/agents/supervision-hook-fixtures.sh`:
   the three recorded causes replayed (narrator read failure, deadline
   expiry, arming failure), each on its first occurrence: the response is an
   allowance with the notice, the hook log carries the line, no block; the
   killed-worker leg also finds the parent's stop-condition line; the
   launcher fallback leg: the shipped Stop launcher line runs where the
   hook cannot bootstrap and prints a degraded allowance, never a block.
   The installed line is proved in Go:
   `TestGeneratedShippedClaudeStopLauncherAllowsDegradedOutsideGit` in
   `metasystem/internal/hostsetup/setup_test.go` runs the line `runtime
   setup` renders from the shipped template, and
   `TestClaudeMergeReplacesTheBlockFallbackLauncher` in
   `metasystem/internal/hooks/setup_test.go` fails readiness on a live
   block-fallback launcher and replaces it with the shipped one.
6. The idle handoff regression: the existing idle tests in
   `metasystem/internal/goal/turnverdict_idle_test.go` keep passing
   unchanged.
7. The next day of stop verdicts on the seats shows no infrastructure
   refusal (DONE clause 5), recorded here by the seat. Field record:
   - 2026-09-12 21:39Z, m1b (the staged hook before the landing): a stop
     with the narrator-digest condition, `stop-condition infrastructure
     narrator-digest-could-not-be-read narrator 496 ... degraded-allow`,
     decision allow.
   - 2026-09-12 22:48:23Z, m1c (rebuilt at 529d8a649): the same condition
     recorded degraded-allow; the stop refused only on open work
     (seat-actionable, OPEN WORK 9), and the next stop at 22:48:48Z with
     the same signature was allowed. No infrastructure refusal.
   - 2026-09-13, midnight to 08:50 local, all three seats rebuilt at
     529d8a649 or later (m1b 00:47, m1e 00:52, m1c on its sync; then
     fed9f5d9e and 286efe41e): m1b 19 stops (15 allowed, 4 refused), m1c
     18 stops (16 allowed, 2 refused), m1e 14 stops (7 allowed, 7
     refused). Every `stop-condition` line on m1b (20) and m1c (26) reads
     `infrastructure narrator-digest-could-not-be-read ... degraded-allow`;
     m1e recorded no infrastructure condition. No deadline expiry and no
     unreadable-output outcome on any seat. The 13 refusals were verdict
     decisions (open work, idle backlog), none an infrastructure cause.
     DONE clause 5 holds for the first nine hours.

## 6. Landing

`metasystem/internal/report/stopblock.go`, `metasystem/internal/goal/turnverdict.go`,
`metasystem/internal/goal/sessionstop.go`, `metasystem/cmd/metasystem/report.go`
(the `--class` flag), `metasystem/scripts/agents/supervision-hook.sh`,
`metasystem/scripts/enforcement/claude-code-hooks.json`,
`metasystem/internal/hooks/setup.go` (the launcher renderer; the Opus read
of the build found that without it the installed Stop line never changes
and `runtime setup --check` fails on every seat), the re-rendered
`.claude/settings.json`, the tests above.
Every seat rebuilds and re-arms once by hand after it lands (the hook file
changes under the running seats; from then on runs re-arm themselves). The
umbrella's members two to eight follow from its page.
