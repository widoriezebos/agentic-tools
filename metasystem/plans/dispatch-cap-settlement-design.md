# Design: reservation settlement for delegated jobs (goal dispatch-cap-necessity)

Date: 2026-09-02. Author: Fable design delegate (job cap-settle-design) for
orchestrator m1b+main-1788333346-60696-6a3256. Status: draft for critique.
All line numbers are from this worktree at commit 4142106d.

## 0. The defect in one paragraph

`ProjectBudget` (internal/dispatch/budget.go:237) walks every job record
bound to the claimed goal revision and, at budget.go:366-369, adds the
record's `capMin` to `projection.ReservedJobMinutes` regardless of status.
A record that ended (`completed`, `failed`, `cancelled`, `timeout`;
`TerminalStatus`, record.go:45-52) therefore keeps charging the minutes it
was ALLOWED to run, not the minutes it ran. On the m1b checkout today the
eight records bound to two-bars-for-changes (revisions 26 and 28) carry
`capMin` 120 each and ran between 12 seconds and 13 minutes 42 seconds:
old rule 960 minutes, observed rule 70 minutes (section 5, case T9). The
governed-run path in the same function already settles ended attempts to
`ObservedCostMinutes` (budget.go:483-487); the delegated-job path never
got that settlement. Wido's word (R-49-m1b): "Is just a bug ... so far
from intent". This design is the fix.

DONE (goal record, plans/goals/dispatch-cap-necessity.md line 4): a goal's
reserved job-minutes equal the minutes its ended jobs ran plus the caps of
its open jobs, and every budget refusal names those two numbers.

## 1. The charge rule (what the record loop adds per record)

Insert the rule at the one charging site, budget.go:366-369, after the
status vocabulary check (346-349), the revision filter (350-352) and the
post-discharge filter (353-361), so a record that is skipped today is never
parsed for timestamps (this keeps every skipped-record fixture valid).

```
charge(record) :=
  status in {pending-setup, pending, running}      -> capMin          (open: the ceiling it may still consume)
  TerminalStatus(status) and process never published -> 0              (never launched: no delegate ran)
  TerminalStatus(status) and process published       -> min(observed, capMin)
    observed := ceil((endedAt - startedAt) in seconds / 60), floor 1
    startedAt or endedAt absent/unparseable, or endedAt before startedAt -> unknownBudget (fail closed)
```

Definitions, each grounded in the record's own writers:

- **Open record charges its cap.** Unchanged from today for `pending-setup`,
  `pending`, `running` (budget.go:347 names exactly these three plus the
  terminal set as the lawful vocabulary). No timestamp is read for an open
  record, so open fixtures without timestamps stay valid.
- **"Process never published" is the proof of "never started".** The
  record's `startedAt` is NOT the launch instant: `BuildRecord` and
  `BuildFollowRecord` stamp `startedAt: nowISO()` when the pending record
  is assembled (build.go:423, build.go:635), and the pending-setup husk
  from `BuildSetup` carries only `createdAt` and no `startedAt`, `endedAt`
  or `pid` at all (build.go:148-164). The process identity is what proves a
  delegate ran: every built record starts with `"pid": nil,
  "pidStartedAt": nil, "pgid": nil` (build.go:400-402) and the launch's
  ownership write fills `pid` (record.go:494-498 describes that pending to
  pending write; `validateOwnershipPatch` at record.go:519 guards it); no
  writer nulls `pid` afterwards (grep `["pid"] =` in internal: only the
  claim-verification result object at claim.go:625 and the supervision
  ledger at supervise/ledger.go:74, neither a job-record clear). The
  predicate is the one the reaper already uses for "no process identity":
  `pid, hasPID := numInt(record["pid"]); !hasPID || pid < 1`
  (reapfacts.go:121-122). Extract it as `recordHasProcessIdentity(record)`
  and call it from both `fingerprintedIdentitylessReservation` and the
  charge rule so the vocabulary has one owner. Records this covers: a husk
  that went `pending-setup -> failed|cancelled` (statusTransitions,
  record.go:39; creator-abandoned at adoption.go:137-142), and a `pending
  -> failed|cancelled` record whose launch never published ownership. Both
  charge 0 and are still counted as an attempt (budget.go:362-365 is
  untouched), so the split guard (section 4) still sees work.
- **Launched terminal record charges observed minutes, clamped to its
  cap.** Parse `startedAt` and `endedAt` with `time.Parse(time.RFC3339, ...)`,
  the parser budget.go:354 already applies to `startedAt`; every writer
  produces that shape (`nowISO()` record.go:709-711 via RecordCAS
  record.go:541-542 and RecordProtocolError record.go:460-461;
  `now.Format(time.RFC3339)` adoption.go:141; the reaper's timeout goes
  through `cfg.transition` at supervise/reaper.go:196-224, whose `Apply`
  is `recordCASApplier` (cmd/metasystem/supervise_component.go:305, 338)
  for the supervision reaper and `applyContinuationReap`
  (steward/reap.go:100-109) for the steward's reap, both a `RecordCAS`
  patch, so the same endedAt stamp applies).
  Seconds are whole (`ended.Sub(started) / time.Second`), minutes are
  `(seconds + 59) / 60`, and a result of 0 becomes 1: byte-for-byte the
  rounding the governed path uses at run/conclude.go:302-309, so a
  12-second handshake failure charges 1 and a 9-minute-4-second critique
  charges 10. The clamp `min(observed, capMin)` is what makes "a job
  killed at its cap settles to its cap or less, never more" true: the
  reaper stamps `endedAt` when it acts, one poll interval after
  `capDeadline` (reapfacts.go:134-152 is the verdict; the reaper acts on
  it), so an unclamped timeout would read 121 against a cap of 120. The
  invariant the clamp gives is: for every record, new charge <= old
  charge, so this change can only lower a goal's projected spend.
- **Fail closed on unreadable timestamps for a launched terminal record.**
  Return `unknownBudget(file.Id, revision, logicalPath, reason)` (budget.go:195-198),
  reasons: `"the terminal reservation has no readable startedAt"`,
  `"the terminal reservation has no readable endedAt"`,
  `"CLOCK_REGRESSED: the terminal reservation ended before it started"`
  (the last mirrors budget.go:267). This is the file's existing shape and
  keeps the projection typed-unknown rather than all-zero (budget.go:20-22).
- **No new configuration key.** The cap stays the per-job runtime ceiling
  fixed at reservation (R-17; `capMin` is immutable, record.go:64) and the
  reaper's kill line (reapfacts.go:134-138). The four structured limits
  (R-13) keep their fields, boundaries and refusal exits.

## 2. Settlement shape: compute from the record, do not mirror the governed shape

The governed path writes a settled figure once at terminalization
(`ObservedCostMinutes`, run/conclude.go:306-310), publishes it a second
time into durable obligation state (run/conclude.go:329-337), and then
`terminalStateContradiction` (budget.go:492-507) refuses the projection
when the two copies disagree. That reconciliation exists because two
records hold the number.

The delegated-job path has one authoritative record and the settled
figure is already on it: `startedAt` is immutable for the record's life
(record.go:63) and `endedAt` is stamped exactly once by the terminal
transition (record.go:541-542), after which a terminal record accepts only
`mirror`, `chainClosed`, `chainUsage`, `runnerClosed` (record.go:92-95,
530-536). Therefore:

- **Decision: compute observed minutes from the record's own timestamps on
  every projection.** No new field, no new writer, no reconciliation.
- **Why not mirror the governed shape.** A written `observedMinutes` field
  would need every terminal writer to stamp it (RecordCAS,
  RecordProtocolError, adoption's creator-abandoned path, the reaper's
  timeout, the shell cancel path) and a missed writer would turn into
  `unknownBudget` for that record forever; the timestamps are already
  stamped by all of them. A second copy of a derivable number is the
  design smell "a wrapper chain renames the same blob"
  (docs/design/design-principles.md, Design Smells). Determinism holds:
  the same record bytes settle to the same minutes on every machine.
- **Shared rounding stays local.** The three-line rounding in
  run/conclude.go:302-309 sits beside the governed exhaustion check that
  consumes it (conclude.go:316-318); this design copies the arithmetic
  into a package-private `settledJobMinutes(started, ended time.Time, capMinutes uint64) uint64`
  in budget.go rather than exporting a helper across packages for it. Test
  T5 pins the two paths to the same answers.

## 3. Projection fields

Add two fields to `BudgetProjection` (budget.go:59-75) and keep
`ReservedJobMinutes` as their sum:

- `ObservedJobMinutes uint64`: settled minutes of ended work: terminal
  delegated jobs (section 1) plus terminal governed attempts'
  `ObservedCostMinutes` (budget.go:487).
- `OpenCapMinutes uint64`: ceilings of open work: open delegated jobs'
  `capMin` plus live governed runs' `ExecutionCostMinutes` (budget.go:456,
  which is that run's timing envelope, budget.go:451 and governed.go:132).
- Invariant, asserted by test T8: `ReservedJobMinutes == ObservedJobMinutes + OpenCapMinutes`
  for every `BudgetKnown` projection. The overflow guards at budget.go:366,
  452, 483 apply to each addition.

## 4. The message and every consumer of `ReservedJobMinutes`

One builder in budget.go owns the reserved-minutes breach text:

```go
// reservedMinutesBreach names the two parts a reader must see: settled
// minutes of ended work and ceilings of open work. proposed == 0 is the
// at-limit breach; otherwise the proposal is the third part.
func reservedMinutesBreach(projection BudgetProjection, proposed uint64) BudgetBreach
```

`Used` is `"<total> observed=<n> open-caps=<m>"` when `proposed == 0` and
`"<total> observed=<n> open-caps=<m> proposed=<p>"` otherwise, where
`<total>` is `projection.ReservedJobMinutes + proposed`; `Limit` is the
decimal limit, unchanged. `BudgetBreach` (budget.go:48-53) gains no field,
so both renderers stay untouched and print, for the two-bars-for-changes
arithmetic of today against a 240-minute limit:

```
BUDGET_REFUSED: goal two-bars-for-changes revision=26 admission closed: reservedJobMinutesLimit used=290 observed=50 open-caps=120 proposed=120 limit=240
```

The tokens `observed=`, `open-caps=`, `limit=` are the ones the goal's
Next step fixed. Nothing parses `used=` back into a number: the only
producers are admission.go:217 and steward/health.go:762, and the shell
consumer stores the text verbatim (`record_delegate_outcome REFUSED-BUDGET
refused "$output"`, dispatch.sh:599, 611).

| Consumer | Line(s) | Changes? | What |
| --- | --- | --- | --- |
| Proposal breach at revision admission | admission.go:156-162 | message only | Condition unchanged (`used < limit && proposed > limit - used`); `Used` built by `reservedMinutesBreach(projection, proposedCap)` instead of `"%d+%d proposed"`. |
| At-limit breach at admission | admission.go:192-193 | message only | `budgetIntegerBreach("reservedJobMinutesLimit", ...)` becomes `reservedMinutesBreach(projection, 0)`; boundary `>=` unchanged. `budgetIntegerBreach` stays for attempts and active jobs. |
| Governed proposal breach | governed.go:134-137 | message only | Same replacement with `cost` as the proposal. The refusal text at governed.go:140 stays. |
| Governed attempt's `ReservedBefore` | governed.go:149, read at run/conclude.go:317 | no code change; meaning improves | `ReservedBefore + observedMinutes >= limit` now compares settled plus open work before the attempt, plus this attempt's observed minutes: the intent the exhaustion check always meant. |
| Health's over-limit breach | budget.go:531-532 (`finishBudgetProjection`) | message only | Same builder with `proposed == 0`; the `>` boundary and the `StopReasonCorruptOverLimit` route through `liveStopReason` (stop.go:79-87) are unchanged, and they fire less often because ended jobs no longer inflate the total. |
| Health status lines | steward/health.go:771-783 | unchanged | Print `reservedJobMinutes=<total>/<limit>`; the total now means settled plus open. Not a refusal, so the two parts are not required there. |
| Split guard | cmd/metasystem/goalsync_mutations.go:507-508 | unchanged | Refuses when attempts, active jobs or reserved minutes are non-zero. A never-launched record charges 0 minutes but still counts 1 attempt (budget.go:365), so "recorded work" is still detected. |
| Breach-stop and stop routing | stop.go:126-137, 228-231, 315-319 | unchanged | Read `Status`, `Elapsed*` and `Unknown` only. |
| Weight discharge | gaterun/weight.go:359-361 | unchanged | Reads `Status` and `WeightEpoch` only. |
| Governed observation | governed.go:68-77 | unchanged | Reads `ActiveJobs` only. |
| Shell admission caller | scripts/agents/dispatch.sh:587-617 | unchanged | Exit codes 9 and 10 from cmd/metasystem/dispatch_verbs.go:958-967 decide; the text is stored, not parsed. |

## 5. Tests (internal/dispatch/budget_test.go unless noted)

**Fixture extension.** `writeBudgetJob` (budget_test.go:41-47) gains one
trailing value-typed parameter:

```go
type budgetJobLife struct {
    startedAt, endedAt string // "" omits the field
    pid                int    // 0 omits the field; a launched job carries its process identity
}
func writeBudgetJob(t *testing.T, root, name, operation string, revision, cap uint64, status string, life budgetJobLife)
```

Existing open-record call sites pass `budgetJobLife{}`. A launched
terminal fixture passes `pid` (any positive integer; the rule only tests
presence and positivity) plus both timestamps.

**Named cases** (all with `budgetGoal()`, claim at 08:00:00Z, limit 75,
projection time 10:00:00Z unless stated):

| Case | Fixture | Assertion |
| --- | --- | --- |
| T1 `TestCompletedJobChargesObservedMinutesNotItsCap` | `completed`, cap 120, pid 4242, 08:10:00Z to 08:20:00Z | `ReservedJobMinutes == 10`, `ObservedJobMinutes == 10`, `OpenCapMinutes == 0`, `ActiveJobs == 0`, `Attempts == 1` |
| T2 `TestRunningJobChargesItsCap` | `running`, cap 45, `budgetJobLife{}` | `ReservedJobMinutes == 45`, `OpenCapMinutes == 45`, `ActiveJobs == 1` (also `pending` and `pending-setup` subtests) |
| T3 `TestFailedJobThatEndedSecondsAfterStartChargesOneMinute` | `failed`, cap 120, pid, 08:10:00Z to 08:10:12Z | `ReservedJobMinutes == 1` |
| T4 `TestTimeoutAtTheCapChargesTheCapNeverMore` | `timeout`, cap 120, pid, 08:00:00Z to 10:01:30Z (reaper lag past the deadline) | `ReservedJobMinutes == 120` |
| T5 `TestSettledMinutesRoundUpLikeTheGovernedPath` | table over `settledJobMinutes`: 0s to 1, 1s to 1, 60s to 1, 61s to 2, 544s to 10, 711s to 12 with cap 120; 7260s with cap 120 to 120 | equals the run/conclude.go:302-309 arithmetic, then the clamp |
| T6 `TestNeverLaunchedTerminalRecordChargesZero` | (a) husk shape: `cancelled`, cap 120, no pid, no startedAt, endedAt 08:12:00Z; (b) `failed`, cap 120, no pid, startedAt 08:10:00Z, endedAt 08:20:00Z | both: `Status == BudgetKnown`, `ReservedJobMinutes == 0`, `Attempts == 1` |
| T7 `TestLaunchedTerminalRecordWithoutReadableTimestampsIsUnknown` | subtests: pid set and (a) no startedAt, (b) no endedAt, (c) `endedAt: "yesterday"`, (d) endedAt before startedAt | `Status == BudgetUnknown`, `Unknown.Record == "artifacts/agents/jobs/<name>.json"`, reason contains `startedAt`, `endedAt`, `endedAt`, `CLOCK_REGRESSED` respectively |
| T8 `TestReservedJobMinutesIsObservedPlusOpenCaps` | one completed (pid, 10 min), one running cap 45, one never-launched failed | `ReservedJobMinutes == ObservedJobMinutes + OpenCapMinutes == 55` |
| T9 `TestTwoBarsForChangesSpecimenSettlesToObservedMinutes` | the eight records on the m1b checkout (goal two-bars-for-changes, cap 120, pid set), timestamps verbatim: rev 26: 11:48:39 to 11:48:51, 11:34:36 to 11:46:27, 16:01:59 to 16:15:41, 11:52:22 to 11:52:34, 15:49:33 to 15:58:37, 16:18:10 to 16:29:46; rev 28: 16:35:27 to 16:46:05, 16:48:05 to 16:56:49 (all 2026-09-02Z) | claimed revision 26: `ReservedJobMinutes == 50` (1+12+14+1+10+12), not 720; claimed revision 28: `20` (11+9), not 240 |
| T10 `TestReservedMinutesBreachNamesBothParts` (admission_test) | projection literal `ReservedJobMinutes: 130, ObservedJobMinutes: 10, OpenCapMinutes: 120`, limit 240, proposed 120 via `EvaluateGoalRevisionAdmission` or directly `reservedMinutesBreach` | `Used == "250 observed=10 open-caps=120 proposed=120"`, `Limit == "240"`; at-limit variant `Used == "130 observed=10 open-caps=120"`; and `FormatGoalAdmission` renders `reservedJobMinutesLimit used=250 observed=10 open-caps=120 proposed=120 limit=240` |

**Existing tests: exact changes.**

- `TestBudgetProjectionUsesJobRecordsForTheBoundRevision`
  (budget_test.go:49-65): the `done` record (line 51) becomes `completed`,
  cap 30, pid 4242, 08:10:00Z to 08:19:30Z; the assertion at line 60
  changes from `ReservedJobMinutes != 75` to `ReservedJobMinutes != 55`
  (10 observed plus the running 45) and adds `ObservedJobMinutes != 10 ||
  OpenCapMinutes != 45`. Why: the test's purpose is "the projection uses
  the sole reservation facts", and under the new rule the fact for an
  ended job is its runtime. Keeping 75 would require a fixture whose
  runtime equals its cap and would prove nothing. `old-revision` (line 53,
  completed, no timestamps) is skipped at budget.go:350-352 before any
  parse, so it stays as is. Limit 75 with 55 used means no breach, as
  today.
- `TestBudgetProjectionReportsExactUnknownRecord/duplicate operation`
  (budget_test.go:134-140): `first` is `completed` with no pid, so it
  settles to 0 and the loop reaches `second.json`, which still trips the
  duplicate check at budget.go:331-333. Assertion unchanged; pass
  `budgetJobLife{}`.
- `TestBudgetProjectionSurfacesBreachesWithoutEnforcement`
  (budget_test.go:179-196): `three` (completed, cap 10, no pid) settles
  to 0; the total is 100 from the two open caps, still above 75, so all
  four breach fields stay. Assertion unchanged.
- `TestUnconsumedDischargeJSONCannotResetTheBudgetProjection`
  (budget_test.go:67-95) and `TestBudgetProjectionStartsAtConsumedDurableProofEpoch`
  (governed_budget_coverage_test.go:15-56): the first fails closed in
  `obligationBudgetStart` before the record loop (budget.go:262-265); in
  the second `before-proof` is filtered at budget.go:358-360 and
  `after-proof` is running. Unchanged.
- `TestPublishedSetupRetainsAttemptAndReservedMinutes`
  (budget_test.go:97-116): a pending-setup husk charges its cap 30.
  Unchanged.
- `TestBudgetAdmissionClosesAtEveryCurrentEqualityBoundary`
  (budget_test.go:198-214) and stop_test.go:209-211 construct projection
  literals; new fields default to zero and the assertions read only
  `Field`. Unchanged.
- evidence/gc_test.go:259 (`spent`, completed, no timestamps, no pid):
  `before` is `BudgetUnknown` today because `empty-goal.json` (line 273)
  trips budget.go:315-316 first, and the test asserts only `before ==
  after`. Unchanged.
- stop_test.go:102-105 and 132-135 (`cancelled`, no pid): settle to 0;
  those tests assert batch states, not minutes. Unchanged.
- steward/health_test.go:267-268, 278, 389: open records only. Unchanged.
- obligationstate/state_test.go:247 asserts `ReservedJobMinutes ==
  attempt.ObservedCostMinutes` on the governed path. Still true.

**Shell and fixture beds asserting the old wording: none.** Under
metasystem/scripts/agents the string `proposed` occurs only as the shell
variable in dispatch.sh:587-621 and in roles/orchestrator.md prose. The
only refusal-text assertions are `attemptLimit used=1 limit=1`
(dispatch-fixtures.sh:1146, attempt limit 1 at line 1092, reserved limit
10000) and `elapsedLimit` (goal-cli-fixtures.sh:409, 552). The
`structured-budget-within` job in that bed is a real fake-runtime dispatch
whose record carries engine-stamped `startedAt`, `endedAt` and `pid`, so it
settles to 1 and the attempt boundary still closes admission.

## 6. Evidence of intent: a bug, not a choice

- **R-13** (memory/rulings.md line 38): "the four structured limits are the
  only budget law, and dispatch admission refuses at a limit boundary with
  structured evidence." The limits bound work. A limit consumed by minutes
  never worked is evidence of nothing.
- **R-17** (line 42): the slice norm "governs the per-job runtime cap
  (capMin) at reservation." The cap is a ceiling fixed when the job is
  reserved; nothing in the ruling makes it the price of the job after it
  ended.
- **R-49-m1b** (line 95), verbatim: "Is just a bug man, why did you not
  notice. That is so far from intent it ios just a bug." and the recorded
  answer to the draft's open question: "the budget projection charges a
  terminal job its observed runtime and an open job its cap; the cap stays
  a fail-stop."
- **The draft's specimens** (plans/goals-drafts/dispatch-cap-necessity.md):
  the 120-minute default cap refusing admission against 60- and 90-minute
  tuples before any work ran (twice, m3, 2026-08-31, lines 14-17); m2's
  Fable critique that could not be admitted because every job reserved a
  flat 120 against a 210-minute pool, freezing the seat four hours
  (lines 25-30); and today's nine-rounds-for-1080 on two-bars-for-changes
  (R-49-m1b; eight of those records are on this checkout, section 5 T9).
- **R-51-m1b** (line 97) fixes the path: Fable designs, Sol critiques, Sol
  builds, Fable critiques; the seat writes no code.

## 7. Unchanged by this design

The reaper's kill line (`CapExpired`, reapfacts.go:134-152), `capMin`
immutability (record.go:64), the slice-norm admission on the cap
(goal/norm.go), the attempt and active-job counting (budget.go:362-375),
the elapsed accounting, the governed-run path (budget.go:377-488), the
`BUDGET_UNKNOWN` and exit-code contract of the admission verbs, and the
mission fence ledgers.

## 8. Self-grade

- **Confidence: high** on the charge rule, the settlement shape and the
  consumer table (every claim is a read line in this worktree; the
  specimen arithmetic is from the live records). **Medium** on the
  fixture list being exhaustive: the grep for terminal job records bound
  to a goal in `*_test.go` was run repository-wide and every hit is
  dispositioned above, but a bed that builds records through the engine
  and then asserts a minute total could exist outside that grep shape.
- **Weakest claim:** that "process never published" (`pid` absent, null
  or below 1) is a sufficient proof of "no delegate ran". A dispatcher
  that spawned the runtime and died before its ownership write leaves a
  record that ran seconds but settles to 0; a pre-census legacy record
  without `pid` at the claimed revision would settle to 0 instead of its
  runtime. Both under-charge, never over-charge, and neither exceeds what
  the old rule allowed, so the failure direction is the safe one.
- **Reject condition:** if any job-record writer clears or omits `pid` on
  a launched record, or if a terminal writer exists that does not stamp
  `endedAt` (the grep at record.go:460, :541, adoption.go:141 and the
  reaper's CAS is the full set found), the never-launched proof or the
  fail-closed rule is wrong and this design must be reopened before build.
  Also reject if a validation bed asserts the `<used>+<proposed> proposed`
  wording; none was found.
