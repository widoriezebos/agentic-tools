# stop-decisions-record-deadline-evidence: design

The second member of stop-hook-never-forces-an-empty-turn (its page,
plans/stop-hook-never-forces-an-empty-turn-design.md, sections 3 and 7).
One mechanism: a stop decision is a durable record keyed by the stop's own
deadline, written before anything is emitted, and completed exactly once.
Written by the m1b seat 2026-09-13; one rostered Codex sol design read,
then a Codex sol build in a worktree, an Opus read, a human commit.

## 1. What the record does not hold today

A Stop has two processes. The deadline parent
(`metasystem/scripts/agents/supervision-hook.sh`, lines 38 to 375) stages
the payload, starts the worker with the deadline's start in
`METASYSTEM_STOP_DEADLINE_STARTED`, and after fifty-seven seconds emits on
its own: an infrastructure allowance for the expired deadline, or the
unreadable-output allowance. The worker allocates the turn generation and
attempt sequence through `steward hook-attempt`
(`metasystem/internal/steward/component_evidence.go`, `BeginHookAttempt`:
a new generation after every emitted turn, the same generation and a
higher attempt sequence when a failed turn is retried), arms supervision
through `up`, reads health and the narrator digest, and asks `report
turn-verdict` for the decision; that verb spends the seen markers and the
idle count and saves the verdict state before it answers
(`metasystem/internal/goal/turnverdict.go`, `saveVerdictState`).

Three things are not recorded. The decision the seat sees has no record of
its own that names the generation, the deadline end, the class and the up
result it was made with: the verdict state holds the spent markers, the
per-session verdict file under artifacts/agents/supervision/stop-verdicts
is replaced at every verdict and is display, and the stop-refusal record
(`metasystem/internal/report/stopblock.go`, version 1) counts causes by
their text per session, without a generation or a deadline. The deadline
parent and the worker can both decide the same stop: a worker whose
verdict returned a block after the parent already emitted the deadline
allowance has spent the seen marker for work the seat never saw refused,
so the next stop allows it (the lost refusal member 1's read noted, now
with its cause). And an infrastructure condition observed twice inside one
sixty-second deadline counts as two occurrences, while the same condition
on the next stop is one more occurrence of a rolling count instead of a new
episode.

## 2. The mechanism

The stop-refusal record becomes version 2: per session as today, at the
same path, resolved from the canonical checkout state root; the version-1
`causes` map is kept as history and never read as evidence that a stop
was processed. It gains `episodes`, keyed by the episode identity
`<turn generation>/<attempt sequence>/<deadline end>` (the generation and
attempt sequence of `steward hook-attempt`, the deadline end as the parent
computed it: start plus sixty seconds). An episode holds:

- `preparedAt`, `holderSession`, `leaseEpoch`, `checkout`;
- `decision`: `pending` until completed, then `class`
  (infrastructure, seat-actionable, idle-with-backlog), `outcome` (block
  or allow), `reason`, `command`, `countState` (spent, not-spent, none),
  `completedBy` (worker or deadline), `completedAt`;
- `arming`: `complete` false until the worker attaches up's result, then
  `components` (outcome, detail, remedy per line, as `up` printed them)
  and `aggregate`, byte for byte (`metasystem/internal/up/up.go`'s
  renderer is the source of both the command's output and this field);
  a deadline that completes before arming finished leaves `complete`
  false and an aggregate that says arming was incomplete, never an armed
  result;
- `incidents`: one per condition fingerprint (cause code, component,
  stable error kind, engine or fence identity; never elapsed time or
  wording) with `count`, `firstAt`, `lastAt`, `delivery` (undelivered
  until member stop-incidents-reach-the-steward acknowledges it).

Three verbs on the record, all under its bounded lock
(`stopRefusalLockWait`), in `metasystem/internal/report/stopblock.go`
and exposed as `report stop-decision` in
`metasystem/cmd/metasystem/report.go`:

1. **prepare** (`--session --generation --attempt --deadline-end`): writes
   the episode with `decision` pending. The worker calls it right after
   `steward hook-attempt` succeeds and before any optional collection
   (`up`, health, narrator, watchdog). Called twice for the same identity
   it changes nothing. When `hook-attempt` failed there is no generation:
   the worker prepares with generation 0 and the deadline end alone, and
   the record says the coordinates are uncertain; such an episode never
   borrows an earlier one.
2. **complete** (`--by worker|deadline`, the decision fields, `--arming
   <file with up's output>`): compare-and-set on `decision == pending`.
   The first completion wins and is returned; a second completion for the
   same identity returns the committed decision with `alreadyCompletedBy`
   and changes nothing, so the caller relays the committed decision
   instead of its own. The record is written before the caller emits.
3. **incident** (`--cause --component --fingerprint`): adds or counts an
   incident on the episode. `report stop-block --class infrastructure`
   gains `--episode <identity>` and records through this path instead of
   the version-1 count; its notice names the episode and the count within
   it (`occurrence 2 in this stop`), not a rolling per-session number.

The verdict verb owns the worker's completion. `report turn-verdict` gains
`--episode <record path>:<identity>` and `--arming <file>`; under the
decision record's lock, before it reads the verdict state, it reads the
episode: completed already (by the deadline) means it answers with the
committed decision as a replay, spends no seen marker and no idle count,
and its display says `decided by the deadline at <end>`; pending means it
decides as today, spends, saves the verdict state, and completes the
episode with its class, outcome, reason, command, count state and the
arming file, all inside the same lock. The lock order is fixed: the
decision record's lock first, the verdict-state lock inside it; the
deadline parent takes only the decision record's lock. When the verdict
state cannot be written, the completion still records the observed
decision with `countState` not-spent, as member 1 requires.

The deadline parent completes the other way. It learns the episode
identity from a file the worker writes beside the payload
(`$deadline_dir/episode`, the identity on one line) as soon as prepare
returned, and on expiry calls `complete --by deadline --class
infrastructure --cause stop-deadline-expired`. A committed worker
decision comes back instead: the parent relays it (a block stays a block,
with the deadline noted in the system message); a pending episode is
completed as the deadline allowance, and the record now says the worker
never decided. The unreadable-output branch does the same with cause
`stop-hook-output-was-unreadable`. With no episode file (the worker died
before prepare) the parent prepares and completes an episode with
generation 0 and its own deadline end, so even that stop has a record.

Before emission, both processes append one decision line to the hook log:
`stop-decision <json>` with `checkout`, `session`, `leaseEpoch`,
`generation`, `attempt`, `deadlineEnd`, `class`, `outcome`, `reason`,
`command`, `countState`, `completedBy`, `aggregate`, `recorded` (true, or
false with the record write's error). The append's exit is checked; a
failed append is said in the emitted text, and an emission whose record
could not be written says `unrecorded` in its notice. This is the decision
half of the line protocol member stop-incidents-reach-the-steward reads;
member 1's `stop-condition` lines stay as they are.

## 3. What does not change

- Which stops block and which allow: the classes, the seen-marker limits,
  the idle counter and its three-refusals handoff are member 1's and the
  umbrella's; this member records them and keeps a late completion from
  spending them twice.
- `steward hook-attempt` and `hook-complete`: the generation and attempt
  sequence come from there; the episode identity reuses them.
- The verdict file under stop-verdicts stays display.
- The steward does not read the record yet: member
  stop-incidents-reach-the-steward drains `incidents` and acknowledges
  deliveries; this member only retains them undelivered. Delivered
  history is pruned with the session retention the record has today.
- The Codex adapter boundary and `host stop-gate`: member
  stop-hosts-enforce-the-common-gate.

## 4. Risks

- A record lock held across the verdict's own work lengthens the window
  in which the deadline parent waits for the lock: the bounded wait
  (`stopRefusalLockWait`) is shorter than the parent's three reserved
  seconds, and a parent that cannot take the lock completes nothing,
  emits the deadline allowance and says the record was busy; the worker's
  completion then stands alone, and the next stop finds it as history.
- The episode file is the only channel from worker to parent for the
  identity; a worker that dies between `hook-attempt` and prepare leaves
  the parent with generation 0. That is recorded as uncertain, never
  merged into the neighbouring generation.
- Two stops within one minute have different deadline ends, so they are
  different episodes even when the generation did not advance (a retried
  failed turn): the umbrella's rule, and the reason the count is per
  episode.
- Replaying a completed decision to a late worker returns a block the
  seat may never see (the parent already allowed): the seen marker is not
  spent, so the same work refuses the next stop. That is the point.

## 5. Fixtures

1. `TestInfrastructureDeadlineIdentity` in
   `metasystem/internal/report/stopblock_test.go`: with a fake clock,
   identical retries of one condition in one episode count once with a
   count of two; two concurrent writers leave one episode; a changed
   fingerprint is a second incident of the same episode; a different
   generation or a different deadline end within sixty seconds is a new
   episode; missing coordinates make an uncertain episode that borrows
   nothing; a version-1 record is read with its causes as history and
   gains `episodes` without losing them.
2. `TestStopDecisionPersistsBeforeEmission` in
   `metasystem/cmd/metasystem/report_test.go` (a new file): `report stop-decision
   complete` writes before it answers, with the full up result and the
   generation; a faulted arming file leaves `arming.complete` false with
   the incomplete aggregate; a faulted incident write leaves the decision
   intact; a faulted record write returns the decision with `recorded`
   false and the write error.
3. `TestStopDeadlineCompletionIsSingleUse` in
   `metasystem/internal/goal/turnverdict_test.go`: an episode completed by
   the deadline, then the verdict verb on the same episode with open
   work: the answer is the committed allowance as a replay, no seen
   marker or idle count is spent, and the next episode blocks on that
   work; the reverse order: the worker's block is committed and a
   deadline completion returns it with `alreadyCompletedBy` worker.
4. `TestArmingDetailSurvivesStop` in `metasystem/cmd/metasystem/up_test.go`
   (member 1's test, extended): the episode's `arming` block, the notice
   and `up`'s own rendering agree byte for byte on outcome, detail,
   remedy and aggregate.
5. Hook bed legs in `metasystem/scripts/agents/supervision-hook-fixtures.sh`:
   the delayed-worker leg (the existing deadline replay) shows the record
   completed by the deadline, the late worker's completion refused, and
   the same open work refused on the next stop; a worker that completes
   in time and a parent that then expires relays the worker's block; the
   narrator condition observed in two stops makes two episodes with one
   incident each, and both stops' `stop-decision` lines are in the log.
6. The existing idle and session-stop tests in
   `metasystem/internal/goal/turnverdict_idle_test.go` keep passing
   unchanged.

## 6. Landing

`metasystem/internal/report/stopblock.go`, `metasystem/cmd/metasystem/report.go`
(`report stop-decision`, `--episode` on `stop-block`),
`metasystem/internal/goal/turnverdict.go` (`--episode`, `--arming`, the
replay and the completion under the record lock),
`metasystem/scripts/agents/supervision-hook.sh` (prepare after
`hook-attempt`, the episode file, the deadline parent's completion and
relay, the decision line), the tests above. No seat rebuild is needed
beyond the landing's own re-arm. Members three to eight follow from the
umbrella's page.
