# Design: reservation settlement for delegated jobs (goal dispatch-cap-necessity), revision 3

Date: 2026-09-02. Author: Fable design delegate (jobs cap-settle-design,
cap-settle-design-r2, cap-settle-design-r3) for orchestrator
m1b+main-1788333346-60696-6a3256. Status: revision 3, for critique round 3
(the declared failsafe). All line numbers are from this worktree at commit
160c31f6 (no source under internal/, cmd/ or scripts/ changed since
revision 1's 4142106d).

Changelog: revision 2 folds critique round 1 (chain cap-settle-crit,
dispositions in plans/dispatch-cap-settlement-dispositions.md):
DCS-R1-TIMESTAMP-AUTHORITY, DCS-R1-START-PROOF (accepted in part),
DCS-R1-REFUSAL-COVERAGE, DCS-R1-DISCHARGED-HUSK, DCS-R1-GOVERNED-COMPONENT-PROOF.
Revision 3 folds critique round 2 (dispositions in
plans/dispatch-cap-settlement-dispositions-r2.md):
DCS-R2-STALE-RESERVED-SNAPSHOT, DCS-R2-END-BEFORE-DEATH,
DCS-R2-MIXED-START-END-CLOCKS.

## 0. The defect in one paragraph

`ProjectBudget` (internal/dispatch/budget.go:237) walks every job record
bound to the claimed goal revision and, at budget.go:366-369, adds the
record's `capMin` to `projection.ReservedJobMinutes` regardless of status.
A record that ended (`completed`, `failed`, `cancelled`, `timeout`;
`TerminalStatus`, record.go:45-52) therefore keeps charging the minutes it
was ALLOWED to run, not the minutes it ran. On the m1b checkout today the
eight records bound to two-bars-for-changes (revisions 26 and 28) carry
`capMin` 120 each and ran between 11 seconds and 13 minutes 41 seconds
measured from the launcher's ownership stamp: old rule 960 minutes,
observed rule 70 minutes (section 5, case T9). The governed-run path in
the same function already settles ended attempts to `ObservedCostMinutes`
(budget.go:483-487); the delegated-job path never got that settlement.
Wido's word (R-49-m1b): "Is just a bug ... so far from intent". This
design is the fix.

DONE (goal record, plans/goals/dispatch-cap-necessity.md line 4): a goal's
reserved job-minutes equal the minutes its ended jobs ran plus the caps of
its open jobs, and every budget refusal names those two numbers.

## 1. The charge rule (what the record loop adds per record)

Insert the rule at the one charging site, budget.go:366-369, after the
status vocabulary check (346-349), the revision filter (350-352) and the
post-discharge filter (353-361, amended in 1.4), so a record that is
skipped today is never parsed for timestamps.

```
charge(record) :=
  status in {pending-setup, pending, running}        -> capMin            (open: the ceiling it may still consume)
  TerminalStatus(status) and no process identity     -> 0                 (never launched: no delegate ran)
  TerminalStatus(status) and process identity:
    start := time.Parse(RFC3339, ownershipProof.provenAt)   when the field is present and non-empty
             else time.Parse(RFC3339, startedAt)            when provenAt is absent or empty
             (a present, non-empty provenAt that does not parse -> unknownBudget)
             else unknownBudget
    end   := time.Parse(RFC3339, endedAt)                   else unknownBudget
    end before start                                        -> unknownBudget (CLOCK_REGRESSED)
    -> min(max(1, ceil((end - start) in whole seconds / 60)), capMin)
```

### 1.1 Open record charges its cap

Unchanged from today for `pending-setup`, `pending`, `running`
(budget.go:347 names exactly these three plus the terminal set as the
lawful vocabulary). No timestamp is read for an open record, so open
fixtures without timestamps stay valid.

### 1.2 "No process identity" is the proof of "never launched"

The record's `startedAt` is NOT the launch instant: `BuildRecord` and
`BuildFollowRecord` stamp `startedAt: nowISO()` when the pending record is
assembled (build.go:423, build.go:635), and the pending-setup husk from
`BuildSetup` carries only `createdAt` and no `startedAt`, `endedAt` or
`pid` at all (build.go:148-164). Every built record starts with `"pid":
nil, "pidStartedAt": nil, "pgid": nil` (build.go:400-402; the indexed husk
at claim.go:710-716 likewise). The launcher spawns the detached runtime
(scripts/agents/dispatch.sh:812-820), builds the ownership patch from the
live process (`job ownership-patch`, dispatch.sh:834-843, which is
`BuildOwnershipPatch` at internal/dispatch/ownership.go:58-82), lands it
with a pending-to-pending compare-and-swap (dispatch.sh:853) and only
then opens the start gate (dispatch.sh:867). `validateOwnershipPatch`
(ownership.go:138-166) admits exactly one such write and refuses any later
rewrite of `pid`; no job-record writer nulls it afterwards (grep `["pid"]
=` in internal: only the claim-verification result object at claim.go:625
and the supervision ledger at supervise/ledger.go:74, neither a job-record
clear). The predicate is the one the reaper already uses for "no process
identity": `pid, hasPID := numInt(record["pid"]); !hasPID || pid < 1`
(reapfacts.go:121-122). Extract it as `recordHasProcessIdentity(record)`
and call it from both `fingerprintedIdentitylessReservation` and the
charge rule so the vocabulary has one owner. Records this covers: a husk
that went `pending-setup -> failed|cancelled` (statusTransitions,
record.go:39; creator-abandoned at adoption.go:137-142), and a `pending
-> failed|cancelled` record whose launch never published ownership. Both
charge 0 and are still counted as an attempt (budget.go:362-365 is
untouched), so the split guard (section 4) still sees work.

### 1.3 The start instant is the launcher's ownership stamp, in the clock domain of `endedAt`

The ownership patch stamps `ownershipProof.provenAt`: the launcher's own
wall clock at the ownership write (`proven_at=$(now_iso)`,
dispatch.sh:829; `now_iso` is `date -u +%Y-%m-%dT%H:%M:%SZ`,
dispatch.sh:108; carried into the proof at ownership.go:75 and required
non-empty by `validateOwnershipPatch` at ownership.go:163-166). `endedAt`
is stamped from the same wall clock by every terminal writer (`nowISO()`,
record.go:709-711; `now.Format(time.RFC3339)`, adoption.go:141;
`nowStamp()`, lease/lease.go:67). Both ends of the measurement are
therefore one clock domain.

`pidStartedAt` is NOT read by the settlement. It stays the census's
identity fact (census/run.go:56, 256, 361; supervise/watchdog.go:69,
122). On Linux it is synthesised from the boot-time epoch plus the
process's start ticks (identity/identity_linux.go:57-58), a wall-clock
second that moves under a clock step (KI-37, memory/known-issues.md
line 44), so a start from that source and an end from the wall clock
would mix two domains.

Live shape, record cap-settle-crit (artifacts/agents/jobs, runtime
state): `startedAt` 17:21:04Z, `ownershipProof.provenAt` 17:21:05Z,
`source` "trusted-launcher", `endedAt` 17:31:16Z. Across the eleven
goal-bound records on this checkout `provenAt` lies 0 to 1 second after
`startedAt`.

Rule: `start` is `ownershipProof.provenAt` parsed with
`time.Parse(time.RFC3339, ...)`, the parser budget.go:354 already applies,
when `ownershipProof` is an object carrying a non-empty `provenAt`; when
the record carries `pid` but no `ownershipProof` or an empty `provenAt` (a
legacy or foreign shape; a shipped launcher always writes both,
ownership.go:75 and 163-166), fall back to `startedAt`; a present,
non-empty `provenAt` that does not parse is `unknownBudget`; when neither
`provenAt` nor `startedAt` parses, `unknownBudget`.

### 1.4 The post-discharge filter reads `startedAt`, else `createdAt`

Today budget.go:353-361 parses `startedAt` for every record at the
claimed revision once a discharge moved the budget start past the claim,
and a pending-setup husk (build.go:148-164; claim.go:705 for the indexed
husk) has only `createdAt`, so a failed or cancelled husk after a consumed
discharge is `unknownBudget` and would stay so at the design's insertion
point. Amended rule: the filter's instant is `startedAt` when the record
carries a non-empty string, else `createdAt`; parse with
`time.Parse(time.RFC3339, ...)`; a record with neither parseable is
`unknownBudget(file.Id, revision, logicalPath, "the post-discharge
reservation has no readable startedAt or createdAt")`; an instant not
after the budget start skips the record as today (budget.go:358-360).
The husk then reaches the charge rule and, having no process identity,
charges 0 (case T6c).

### 1.5 `endedAt` becomes transition-owned

Today `endedAt` is absent from `immutableFields` (record.go:60-75), so
`RecordCAS` preserves a lawful open-record patch that carries it and only
stamps its own value when the field is still empty at terminalization
(record.go:541-542). A settlement read from a value any open-record
writer may set is not transition-owned. Rule: in `RecordCAS`, directly
after the `status` refusal (record.go:516-518, the precedent: `status` is
transition-owned and refused in every patch) and before
`validateOwnershipPatch` (record.go:519), refuse any patch that has an
`endedAt` key, whatever its value or the record's status:

```go
if _, has := patch["endedAt"]; has {
    return refuse(1, "record patch cannot contain endedAt; the terminal transition stamps it")
}
```

Exit code 1, the same refusal shape as `record patch attempts to change
immutable identity` (record.go:523-525). The terminal transition alone
stamps it, each writer only when the field is still empty, which after
this rule is always, since every built record starts at null
(build.go:424, 636). The four terminal writers: `RecordCAS`
(record.go:541-542), `RecordProtocolError` (record.go:460-461), the
creator-abandoned reconciliation (adoption.go:138-141) and the lease claim
sweep's stale-job conclusions (lease/sweep.go:151-152, 166-167; amended
in 1.9). Writers checked that never patch `endedAt` on an open record:
dispatch.sh and the adapters (grep `endedAt` under scripts/agents: only
dispatch-fixtures.sh, which lists the field at 1289, nulls it in a copied
source record at 1705 and writes a whole fixture record at 3169, none a
CAS patch); build.go initialises it null; claim.go copies it into the
`recordedOutcome` view (claim.go:757-765); the mission runner's `endedAt`
writes are its own runner record (missionrunner/loop.go:213, guarded by
its pid) and turn records (host.go:400, 464, loop.go:2048-2213), not job
records; host/fake.go:89 creates a whole terminal fixture record, not a
patch. Test T11.

### 1.6 Rounding and the clamp

Seconds are whole (`end.Sub(start) / time.Second`), minutes are
`(seconds + 59) / 60`, and a result of 0 becomes 1: byte-for-byte the
rounding the governed path uses at run/conclude.go:302-309, so an
11-second handshake failure charges 1 and a 9-minute-4-second critique
charges 10. The clamp `min(observed, capMin)` is what makes "a job killed
at its cap settles to its cap or less, never more" true: the reaper stamps
`endedAt` when it acts, one poll interval after `capDeadline`
(reapfacts.go:134-152 is the verdict), so an unclamped timeout would read
121 against a cap of 120. Invariant: for every record, new charge <= old
charge; this change can only lower a goal's projected spend.

### 1.7 Fail closed

Return `unknownBudget(file.Id, revision, logicalPath, reason)`
(budget.go:195-198) with reasons `"the terminal reservation has no
readable ownership proof time or startedAt"`, `"the terminal reservation
has an unreadable ownershipProof.provenAt"`, `"the terminal reservation
has no readable endedAt"`, `"CLOCK_REGRESSED: the terminal reservation
ended before its ownership was proven"` (mirrors budget.go:267). The
projection stays typed-unknown rather than all-zero (budget.go:20-22).

### 1.8 No new configuration key

The cap stays the per-job runtime ceiling fixed at reservation (R-17;
`capMin` is immutable, record.go:64) and the reaper's kill line
(reapfacts.go:134-138). The four structured limits (R-13) keep their
fields, boundaries and refusal exits.

### 1.9 The stamp follows death evidence

A terminal record whose runtime still runs would settle its minutes while
the process keeps consuming them, and its cap would no longer be reserved
for a live process. Each of the four writers in 1.5 must therefore act
only on a dead or self-reporting process. Three already do:

- **The reaper's timeout.** The standing reaper has no kill authority
  (supervise/reaper.go:23-31): with a live custodian past its cap it only
  emits `REAP-DECLINED ... kill authority stays with dispatch`
  (reaper.go:150-156); it transitions to `timeout` only when the custodian
  is dead and `identity.TaggedSurvivors` proves no tagged group member
  alive, else `REAP-DEFERRED` (reaper.go:158-176), stamping
  `groupDeathProvenAt` in the same patch (reaper.go:177-200). The
  kill-capable path is dispatch.sh's reap: `wind_down_group`
  (dispatch.sh:1066, 1077) before the timeout compare-and-swap with
  `groupDeathProvenAt` (dispatch.sh:1079-1081); `wind_down_one_group`
  (dispatch.sh:339-353) is the ladder: SIGTERM, a 2-second bounded wait,
  a re-proof of ownership and SIGKILL, a second 2-second wait, and a
  refusal if the group survives.
- **`RecordProtocolError`.** Called by the adapter's own
  `finish_protocol_error` (adapters/runtime-common.sh:197-202; fake.sh:130,
  261) through `__protocol-error` (dispatch.sh:2469-2472) after the runtime
  it launched has exited and its return failed validation: a self-report
  from the process owner.
- **The creator-abandoned reconciliation.** `ReconciliationCreatorAbandoned`
  is returned only when the creator's recorded identity is proven dead
  (adoption.go:211-212, `creator-identity-proven-dead`) after the census
  found no tagged survivor to adopt (adoption.go:190-200); the record was
  never launched, so there is no group to kill.

The fourth does not. `stopStaleGroup` (lease/sweep.go:182-204) returns
as soon as SIGTERM delivery succeeds (`case nil, unix.ESRCH: return nil`,
sweep.go:199-200) and `concludeStaleJob` (sweep.go:136-170) stamps
`endedAt` at once (sweep.go:151-152, 166-167). Amended rule for the sweep,
the same ladder dispatch.sh climbs:

1. After a successful SIGTERM (`ESRCH` on the first signal means the
   group is already gone: proceed to the stamp), poll `sweepGroupAbsent`
   every 50 milliseconds for up to 2 seconds.
2. If the group is still present, re-prove ownership with `groupOwnsTag`
   (sweep.go:226; an unprovable or foreign group is the same refusal as
   sweep.go:191-196) and send SIGKILL through `sweepKill`; poll again for
   up to 2 seconds.
3. Final check: `sweepGroupAbsent(pgid)`. Absent: proceed to the stamp.
   Present: `stopStaleGroup` returns `claim sweep could not stop stale job
   %s: process group %d survived SIGKILL` and emits `sweep-refused` with
   the job id and pgid.

Bound: 4 seconds of waiting plus the two polling granularities per stale
job, the numbers dispatch.sh uses at :343-352. `sweepGroupAbsent` is a
fifth seam beside the four at sweep.go:214-224, defaulting to
`identity.GroupAbsent`. That function is today's `kernelGroupAbsent`
(supervise/arming.go:356-366: `kill(-pgid, 0)`, `ESRCH` means absent,
`nil` or `EPERM` means present) moved to the identity package, which
already owns the kernel process facts (`AllPids`, `TaggedSurvivors`,
tagstate.go:24); it moves because supervise imports lease (arming.go,
reaper.go) so lease cannot import supervise. `recordedComponentControl.groupAbsent`
keeps its behaviour by pointing at the moved function.

When the group will not die the record must not become terminal: on the
error `concludeStaleJob` writes nothing (the record keeps its status and
its `endedAt` stays null), `sweepOne` propagates (sweep.go:119, 130),
`cleanupStaleJobs` returns from its loop (sweep.go:46-52) before
`sweep-completed` (sweep.go:57), and the caller refuses: the takeover returns without writing its sweep
stamp (claim.go:263-268), succession (claim.go:201-205) and
`completeInterruptedSweep` (claim.go:289-294) return the error. Who
retries: the next lease claim, succession or `up` re-runs
`cleanupStaleJobs` because the stamp is absent (`stampComplete`,
claim.go:310, consulted at 202 and 290), and once the job's cap expires
the dispatch reap path (dispatch.sh:1066-1081) winds the group down and
stamps `timeout` under its own ladder. Test T14.

## 2. Settlement shape: compute from the record, do not mirror the governed shape

The governed path writes a settled figure once at terminalization
(`ObservedCostMinutes`, run/conclude.go:306-310), publishes it a second
time into durable obligation state (run/conclude.go:329-337), and then
`terminalStateContradiction` (budget.go:492-507) refuses the projection
when the two copies disagree. That reconciliation exists because two
records hold the number.

The delegated-job path has one authoritative record and, after 1.5 and
1.9, the settled figure is already on it and owned by transitions: `pid`
and `ownershipProof` (with `provenAt`) are written once by the ownership
patch and never rewritten (every `ownershipPatchFields` key,
ownership.go:9-13, is refused once `pid` is set, ownership.go:149-151),
`startedAt` is immutable (record.go:63), `endedAt` is refused in every
patch and stamped once by a terminal transition acting on death evidence,
after which a terminal record accepts only `mirror`, `chainClosed`,
`chainUsage`, `runnerClosed` (record.go:92-95, 530-536). Therefore:

- **Decision: compute observed minutes from the record's own timestamps on
  every projection.** No new field, no new writer, no reconciliation.
- **Why not mirror the governed shape.** A written `observedMinutes` field
  would need every terminal writer to stamp it (the four in 1.5) and a
  missed writer would turn into `unknownBudget` for that record forever;
  the timestamps are already stamped by all of them. A second copy of a
  derivable number is the design smell "a wrapper chain renames the same
  blob" (docs/design/design-principles.md, Design Smells). Determinism
  holds: the same record bytes settle to the same minutes on every machine.
- **Shared rounding stays local.** The three-line rounding in
  run/conclude.go:302-309 sits beside the governed exhaustion check that
  consumes it (conclude.go:316-318); this design copies the arithmetic
  into a package-private `settledJobMinutes(start, end time.Time, capMinutes uint64) uint64`
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
- Invariant, asserted by test T8 over delegated, governed and mixed beds:
  `ReservedJobMinutes == ObservedJobMinutes + OpenCapMinutes` for every
  `BudgetKnown` projection. The overflow guards at budget.go:366, 452, 483
  apply to each addition.

## 4. Every refusal names the two numbers; every consumer of `ReservedJobMinutes`

### 4.1 The reserved evidence and its renderers

The per-limit breach objects (`BudgetBreach`, budget.go:48-53) keep their
fields and today's values: `budgetIntegerBreach` for the at-limit
breaches (admission.go:189-197, budget.go:528-536) and the
`"%d+%d proposed"` form for a proposal breach (admission.go:156-162,
governed.go:134-137) are unchanged. The two numbers live on the refusal
itself:

```go
// ReservedMinutesEvidence is the two-part reserved-minutes fact every
// budget refusal names: settled minutes of ended work, ceilings of open
// work, and the limit both are measured against.
type ReservedMinutesEvidence struct{ Observed, OpenCaps, Limit uint64 }

func reservedMinutesEvidence(projection BudgetProjection) *ReservedMinutesEvidence // budget.go
```

`GoalAdmissionRefusal` (admission.go:15-21) gains `Reserved
*ReservedMinutesEvidence`, filled from the `BudgetKnown` projection at
every refusal that has one: admission.go:98-104 (goal admission breaches),
:148-153 (live-stop refusal) and :164-166 (revision admission). Unknown
refusals (:82-89, :92-96, :133-141, :144-147) carry nil: the projection
refused to guess, and the renderer invents nothing.

`FormatGoalAdmission` (admission.go:203-223) keeps one line per refusal
and appends the reserved segment after the joined breach fields, separated
by `"; "`, whenever `Reserved` is non-nil. Exact renderings:

```
attempt-only refusal (the dispatch-fixtures bed, attempt limit 1, reserved limit 10000, one settled 1-minute job):
BUDGET_REFUSED: goal structured-budget revision=1 admission closed: attemptLimit used=1 limit=1; reserved observed=1 open-caps=0 limit=10000

reserved-minutes refusal (today's two-bars-for-changes arithmetic against a 240-minute limit, one job open):
BUDGET_REFUSED: goal two-bars-for-changes revision=26 admission closed: reservedJobMinutesLimit used=170+120 proposed limit=240; reserved observed=50 open-caps=120 limit=240
```

Both parts and the proposal are visible on the second; the first shows
the parts on a refusal the reserved limit did not trip. The tokens
`observed=`, `open-caps=`, `limit=` are the ones the goal's Next step
fixed. Substring assertions on single fields keep matching
(`attemptLimit used=1 limit=1`, dispatch-fixtures.sh:1146; `admission
closed: elapsedLimit`, goal-cli-fixtures.sh:409, 552, since the elapsed
field is rendered first). Nothing parses the line back: the only producers
are admission.go:217-220 and steward/health.go:762, and the shell stores
the text verbatim (`record_delegate_outcome REFUSED-BUDGET refused
"$output"`, dispatch.sh:599, 611); the verb prints each line and decides
by exit code (cmd/metasystem/dispatch_verbs.go:958-967).

The governed refusal (governed.go:139-141) carries the same line. Today it
is the bare `BUDGET_REFUSED: goal %s revision=%d admission closed`; it
becomes `BUDGET_REFUSED: goal %s revision=%d admission closed: <breach
fields joined as FormatGoalAdmission joins them>; reserved observed=<n>
open-caps=<m> limit=<L>`, rendered by the one helper the admission
renderer uses (extract `formatRefusalDetail(breaches []BudgetBreach,
reserved *ReservedMinutesEvidence) string` in admission.go and call it
from both). No test or bed asserts the governed text (grep `admission
closed` in `*_test.go` and scripts: only the two elapsed assertions above
and a subtest name in steward/health_test.go:247).

### 4.2 Consumers

| Consumer | Line(s) | Changes? | What |
| --- | --- | --- | --- |
| Goal admission refusals | admission.go:98-104 | evidence added | `Reserved` filled from the projection; breach objects unchanged. |
| Revision admission refusals | admission.go:148-153, 164-166 | evidence added | Same; the proposal breach at :156-162 keeps its `"%d+%d proposed"` text. |
| Admission renderer | admission.go:203-223 | rendering | Appends `; reserved observed=<n> open-caps=<m> limit=<L>` on every refusal with a known projection. |
| Governed admission refusal | governed.go:134-141 | rendering | Breach objects unchanged; the error text carries fields and the reserved segment through the shared helper. |
| Governed exhaustion at conclusion | governed.go:149 (`ReservedBefore`), run/conclude.go:316-318 | decision changes (4.3) | The check re-projects at conclusion; `ReservedBefore` stays on the record as the admission-time fact and no longer decides. |
| Health's over-limit breach | budget.go:531-532 (`finishBudgetProjection`) | unchanged | `budgetIntegerBreach` text as today; the `>` boundary and the `StopReasonCorruptOverLimit` route through `liveStopReason` (stop.go:79-87) are unchanged and fire less often because ended jobs no longer inflate the total. |
| Health status and BREACH lines | steward/health.go:755-783 | unchanged | Health verdicts are not budget refusals; they print the total, which now means settled plus open. |
| Split guard | cmd/metasystem/goalsync_mutations.go:507-508 | unchanged | Refuses when attempts, active jobs or reserved minutes are non-zero. A never-launched record charges 0 minutes but still counts 1 attempt (budget.go:365), so "recorded work" is still detected. |
| Breach-stop and stop routing | stop.go:126-137, 228-231, 315-319 | unchanged | Read `Status`, `Elapsed*` and `Unknown` only. |
| Weight discharge | gaterun/weight.go:359-361 | unchanged | Reads `Status` and `WeightEpoch` only. |
| Governed observation | governed.go:68-77 | unchanged | Reads `ActiveJobs` only. |
| Shell admission caller | scripts/agents/dispatch.sh:587-617 | unchanged | Exit codes 9 and 10 decide; the text is stored, not parsed. |

### 4.3 The governed exhaustion check re-projects at conclusion

Today `EvaluateGovernedRunAdmission` freezes `ReservedBefore:
projection.ReservedJobMinutes` on the attempt (governed.go:149) and
`applyGovernedTerminal` decides exhaustion as `ReservedBefore +
observedMinutes >= ReservedJobMinutesLimit` (run/conclude.go:316-318).
Under the settled meaning an open cap inside that snapshot may have
settled to a few minutes by the time the attempt concludes, so the frozen
figure over-counts and can exhaust an obligation falsely. Rule: the
check uses a projection taken at the conclusion instant, excluding the
attempt being concluded, plus that attempt's observed minutes.

- **Seam.** `run.Store` gains one injection hook beside `ObserveGoverned`
  (run/run.go:234-235):

  ```go
  // SpendSnapshot is a goal's settled spend at one instant, excluding the
  // run being concluded: minutes of ended work and ceilings of open work.
  type SpendSnapshot struct{ ObservedMinutes, OpenCapMinutes uint64 }
  // ProjectSpend re-projects the goal at conclusion. A non-empty unknown
  // names the record that prevented a trustworthy projection.
  ProjectSpend func(record *Record, now time.Time) (snapshot SpendSnapshot, unknown string)
  ```

  Production wiring sits where `ObserveGoverned` is wired
  (cmd/metasystem/run.go:62-64): `ProjectSpend` calls
  `dispatchcore.SettledSpendAtConclusion(root, record, now)`, a sibling of
  `ObserveGovernedRun` (governed.go:55-78) that resolves the goal binding
  and refuses on a revision mismatch exactly as governed.go:63-66 does,
  then calls `ProjectBudgetWithoutRun(root, binding.File, now,
  record.RunId)` and returns the projection's `ObservedJobMinutes` and
  `OpenCapMinutes`, or `"record=<r> reason=<why>"` from
  `projection.Unknown`. The run package cannot import dispatch (budget.go:17
  imports run), which is why the seam exists.
- **How the concluding attempt is excluded.** `ProjectBudget` becomes a
  wrapper over `ProjectBudgetWithoutRun(repoRoot, file, now, excludeRunID
  string)` with an empty id; the run-record loop's skip at budget.go:412-414
  becomes `if record.GoalId != file.Id || record.RunId == excludeRunID {
  continue }`. That suffices because of the read path: `terminalizeWithVerdict`
  (conclude.go:216-236) and `terminalize` (conclude.go:254-272) call
  `applyGovernedTerminal` inside the `s.cas` callback, before the record
  is written, so the disk copy the projection reads (`Store.Read`,
  run.go:375-380, called at budget.go:402-404) still holds the pre-terminal
  status and would otherwise be charged as an open cap at
  budget.go:451-462; and the durable obligation copy for this run is
  published only by `RecordTerminal` at conclude.go:329-337, after the
  check, so budget.go:464-488 cannot see it either.
- **The check.** conclude.go:317 becomes: with a known snapshot,
  `snapshot.ObservedMinutes + snapshot.OpenCapMinutes + observedMinutes >=
  ReservedJobMinutesLimit`; the attempt and elapsed clauses at
  conclude.go:316 and :318 are unchanged. When the seam is nil or returns
  a non-empty unknown, fail closed: `reached` is true and, if the attempt
  is failing, `ExhaustionReason` is `"BUDGET_UNKNOWN at conclusion: " +
  unknown` (nil seam: `"no spend projection is wired"`), so the obligation
  waits for the human exactly as any exhausted breaker does
  (governed.go:121-126). A green, assumption-matching attempt is
  unaffected: exhaustion needs `failing` (conclude.go:319-320). A real
  exhaustion's reason names the parts: `"terminal non-green attempt
  reached the human-set tuple: observed=<n> open-caps=<m> attempt=<o>
  limit=<L>"`; the same string goes into the durable attempt, so
  `terminalStateContradiction` (budget.go:498-504) still matches.
- **`ReservedBefore`** keeps being written at governed.go:149 and read
  by nothing that decides; it is the admission-time fact for the audit
  trail. `terminalStateContradiction` does not compare it (budget.go:498-504).

## 5. Tests (internal/dispatch/budget_test.go unless noted)

**Fixture extension.** `writeBudgetJob` (budget_test.go:41-47) gains one
trailing value-typed parameter:

```go
type budgetJobLife struct {
    startedAt, endedAt, createdAt string // "" omits the field
    pid                           int    // 0 omits the field; a launched job carries its process identity
    provenAt                      string // "" omits ownershipProof; else {"provenAt": ..., "source": "trusted-launcher"}
}
func writeBudgetJob(t *testing.T, root, name, operation string, revision, cap uint64, status string, life budgetJobLife)
```

Existing open-record call sites pass `budgetJobLife{}`. A launched
terminal fixture passes `pid` (any positive integer; the rule only tests
presence and positivity), `provenAt` and `endedAt`.

**Named cases** (all with `budgetGoal()`, claim at 08:00:00Z, limit 75,
projection time 10:00:00Z unless stated; "launched at T" means `pid` 4242
plus `provenAt` T):

| Case | Fixture | Assertion |
| --- | --- | --- |
| T1 `TestCompletedJobChargesObservedMinutesNotItsCap` | `completed`, cap 120, launched at 08:10:00Z, endedAt 08:20:00Z | `ReservedJobMinutes == 10`, `ObservedJobMinutes == 10`, `OpenCapMinutes == 0`, `ActiveJobs == 0`, `Attempts == 1` |
| T2 `TestRunningJobChargesItsCap` | `running`, cap 45, `budgetJobLife{}` | `ReservedJobMinutes == 45`, `OpenCapMinutes == 45`, `ActiveJobs == 1` (also `pending` and `pending-setup` subtests) |
| T3 `TestFailedJobThatEndedSecondsAfterStartChargesOneMinute` | `failed`, cap 120, launched at 08:10:00Z, endedAt 08:10:11Z | `ReservedJobMinutes == 1` |
| T4 `TestTimeoutAtTheCapChargesTheCapNeverMore` | `timeout`, cap 120, launched at 08:00:00Z, endedAt 10:01:30Z (reaper lag past the deadline) | `ReservedJobMinutes == 120` |
| T5 `TestSettledMinutesRoundUpLikeTheGovernedPath` | table over `settledJobMinutes`: 0s to 1, 1s to 1, 60s to 1, 61s to 2, 544s to 10, 711s to 12 with cap 120; 7260s with cap 120 to 120 | equals the run/conclude.go:302-309 arithmetic, then the clamp |
| T6 `TestNeverLaunchedTerminalRecordChargesZero` | (a) husk shape: `cancelled`, cap 120, no pid, no startedAt, createdAt 08:05:00Z, endedAt 08:12:00Z; (b) `failed`, cap 120, no pid, startedAt 08:10:00Z, endedAt 08:20:00Z; (c) discharge branch, on the bed of governed_budget_coverage_test.go:15-36 (obligation revision 6, consumed proof at 09:30:00Z): a `cancelled` husk with createdAt 09:45:00Z, endedAt 09:46:00Z, no startedAt, no pid; a second husk with createdAt 09:00:00Z; a third with neither startedAt nor createdAt | (a), (b): `Status == BudgetKnown`, `ReservedJobMinutes == 0`, `Attempts == 1`. (c): the 09:45 husk counts (`Attempts == 1`, `ReservedJobMinutes == 0`, `Status == BudgetKnown`); the 09:00 husk is filtered (`Attempts` unchanged); the stampless husk is `BudgetUnknown` naming its record with a reason containing `startedAt or createdAt` |
| T7 `TestLaunchedTerminalRecordWithoutReadableTimestampsIsUnknown` | pid 4242 and (a) no ownershipProof and no startedAt, (b) no endedAt, (c) `endedAt: "yesterday"`, (d) endedAt 08:09:00Z before provenAt 08:10:00Z, (e) `provenAt: "yesterday"` with startedAt 08:10:00Z | `Status == BudgetUnknown`, `Unknown.Record == "artifacts/agents/jobs/<name>.json"`, reason contains `ownership proof time`, `endedAt`, `endedAt`, `CLOCK_REGRESSED`, `provenAt` respectively |
| T8 `TestReservedJobMinutesIsObservedPlusOpenCaps` | (a) delegated: one completed (launched, 10 min), one running cap 45, one never-launched failed. (b) governed: `obligationstate.RecordTerminal(root, "bounded", 3, 6, TerminalAttempt{RunID "settled-run", Status green, StartedAt 08:10:00Z, EndedAt 08:35:00Z, PrunedAt 08:40:00Z, AttemptOrdinal 1, ExecutionCostMinutes 30, ObservedCostMinutes 25, WeightGeneration 1, BudgetEpoch nil, Breaker closed})` plus one live governed run written as governed_test.go:179-186 does (`run.Store{Root, Now, AdmitGoverned}` returning an attempt with `GoalRevision 3, ExecutionCostMinutes 30, BudgetEpoch nil`, then `Launch` with `GoalId "bounded"`). (c) both beds together | (a): `ObservedJobMinutes == 10`, `OpenCapMinutes == 45`, sum 55. (b): `ObservedJobMinutes == 25`, `OpenCapMinutes == 30`, `ReservedJobMinutes == 55`, `Attempts == 2`, `ActiveJobs == 1`. (c): `ObservedJobMinutes == 35`, `OpenCapMinutes == 75`, `ReservedJobMinutes == 110`, `Attempts == 5`; and in every subtest `ReservedJobMinutes == ObservedJobMinutes + OpenCapMinutes` |
| T9 `TestTwoBarsForChangesSpecimenSettlesToObservedMinutes` | the eight records on the m1b checkout (goal two-bars-for-changes, cap 120, pid set), `ownershipProof.provenAt` and `endedAt` verbatim, all 2026-09-02Z: rev 26: 11:48:40 to 11:48:51, 11:34:36 to 11:46:27, 16:02:00 to 16:15:41, 11:52:23 to 11:52:34, 15:49:33 to 15:58:37, 16:18:11 to 16:29:46; rev 28: 16:35:28 to 16:46:05, 16:48:05 to 16:56:49 | claimed revision 26: `ReservedJobMinutes == 50` (1+12+14+1+10+12), not 720; claimed revision 28: `20` (11+9), not 240 |
| T10 `TestEveryBudgetRefusalNamesObservedAndOpenCaps` (admission_test) | (a) attempt-only: goal with `AttemptLimit 1, ReservedJobMinutesLimit 10000`, one settled 1-minute job, `EvaluateGoalRevisionAdmission` with proposed cap 120; (b) reserved-minutes: limit 240, one settled 50-minute job, one running cap 120, proposed 120; (c) unknown: a revisionless record | (a): breaches `[attemptLimit used=1 limit=1]`, `Reserved == {1, 0, 10000}`, `FormatGoalAdmission` renders exactly `BUDGET_REFUSED: goal bounded revision=3 admission closed: attemptLimit used=1 limit=1; reserved observed=1 open-caps=0 limit=10000`. (b): breach `reservedJobMinutesLimit used=170+120 proposed limit=240`, `Reserved == {50, 120, 240}`, rendered `...admission closed: reservedJobMinutesLimit used=170+120 proposed limit=240; reserved observed=50 open-caps=120 limit=240`. (c): `Reserved == nil` and the `BUDGET_UNKNOWN` line has no reserved segment. Plus a governed subtest: `EvaluateGovernedRunAdmission` on an active obligation whose cost breaches, error text contains `; reserved observed=` |
| T11 `TestRecordCASRefusesEndedAtPatch` (record_test.go, beside `TestRecordCASRefusesImmutableField` at :196-204) | `createPending`, `setupPending`; patch `{"endedAt": "2026-08-28T09:00:00Z"}` for pending to running, then for pending to pending; then empty patches pending to running and running to completed; then a terminal metadata patch `{"mirror": ..., "endedAt": "2026-01-01T00:00:00Z"}` | the first two: exit code 1 and message `record patch cannot contain endedAt; the terminal transition stamps it`, record unchanged; after the completion the record's `endedAt` parses as RFC3339; the terminal patch: exit code 1 and the same message, `endedAt` byte-identical to the stamp |
| T12 `TestObservedMinutesRunFromTheOwnershipStamp` | (a) `completed`, startedAt 08:10:00Z, launched at 08:12:00Z, endedAt 08:22:00Z; (b) fallback: pid 4242, no ownershipProof, startedAt 08:10:00Z, endedAt 08:20:00Z; (c) ownershipProof with `provenAt: ""` and startedAt 08:10:00Z, endedAt 08:20:00Z; (d) `pidStartedAt` set to the epoch of 07:00:00Z beside provenAt 08:12:00Z and endedAt 08:22:00Z | (a): `ReservedJobMinutes == 10`, not 12; (b), (c): `ReservedJobMinutes == 10`; (d): `10`, proving `pidStartedAt` is not read |
| T13 `TestGovernedExhaustionReprojectsSettledSpendAtConclusion` (dispatch package, on the bed of governed_test.go:194-200 with `ProjectSpend` wired to `SettledSpendAtConclusion`; goal `ReservedJobMinutesLimit` 150, `AttemptLimit` at least 2, obligation `TimingEnvelopeSeconds` 1800 so the cost is 30) | (a) settled cap: a delegated job `running` cap 120; the governed attempt is admitted at equality (used 120, cost 30, `ReservedBefore == 120`); the delegated job then completes (launched at 08:00:00Z, endedAt 08:10:00Z); the attempt starts 09:00:00Z and concludes red at 09:30:00Z. (b) real exhaustion: as (a) but the delegated job stays running. (c) exclusion: no other job, limit 60, cost 30, the attempt concludes red after 30 minutes. (d) unknown: as (b) plus an unreadable job record (duplicate keys, as budget_test.go:159) | (a): `Governed.Exhausted == false`, `Breaker != BreakerExhausted`, `ExhaustionReason == ""` (the old check would have read 120 + 30 >= 150). (b): `Exhausted == true`, reason contains `observed=0 open-caps=120 attempt=30 limit=150`. (c): `Exhausted == false` (an un-excluded run would have read 30 + 30 >= 60). (d): `Exhausted == true`, reason contains `BUDGET_UNKNOWN at conclusion` and the record path |
| T14 `TestClaimSweepStampsEndedAtOnlyAfterGroupDeath` (lease/sweep_test.go, with the seams as sweep_test.go:17-30 plus `sweepGroupAbsent`) | a stale `running` record with `pgid` and `instanceTag`, ownership provable; `sweepKill` records each signal; `sweepGroupAbsent` scripted: (a) present for the first three polls after SIGTERM, then absent; (b) present until the SIGKILL, absent after it; (c) present forever; (d) SIGTERM returns `ESRCH` | (a): signals `[TERM]`, `endedAt` stamped, status `failed`, `error` `stale-claim-epoch`. (b): signals `[TERM, KILL]`, `endedAt` stamped. (c): signals `[TERM, KILL]`, `concludeStaleJob` returns an error containing `survived SIGKILL`, the record on disk keeps status `running` and no `endedAt`, `cleanupStaleJobs` returns the error and emits no `sweep-completed`, and a takeover through it leaves `stampComplete` false. (d): no wait, `endedAt` stamped |

**Existing tests: exact changes.**

- `TestBudgetProjectionUsesJobRecordsForTheBoundRevision`
  (budget_test.go:49-65): the `done` record (line 51) becomes `completed`,
  cap 30, launched at 08:10:00Z, endedAt 08:19:30Z; the assertion at line
  60 changes from `ReservedJobMinutes != 75` to `ReservedJobMinutes != 55`
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
  (budget_test.go:67-95) fails closed in `obligationBudgetStart` before
  the record loop (budget.go:262-265). Unchanged.
- `TestBudgetProjectionStartsAtConsumedDurableProofEpoch`
  (governed_budget_coverage_test.go:15-56): `before-proof` is filtered at
  the amended post-discharge filter (its `startedAt` 09:00 is before the
  09:30 budget start) and `after-proof` is running; the green-proof
  attempt sits in budget epoch 0 while the projection's epoch is 1, so
  budget.go:474-476 skips it. The assertion at line 53 gains
  `|| projection.ObservedJobMinutes != 0 || projection.OpenCapMinutes != 20`.
- obligationstate/state_test.go:247 gains `|| projection.ObservedJobMinutes
  != attempt.ObservedCostMinutes || projection.OpenCapMinutes != 0`.
- Governed conclusion fixtures. `testStore` (run/run_test.go:38) wires
  `ProjectSpend` to return `SpendSnapshot{}` and an empty unknown, so
  `TestFailingTerminalAttemptOwnsExhaustionAndRaisesDebt`
  (run/governed_test.go:29-60, exhausted through `AttemptLimit` 1) and the
  attempt-limit-2 case at run/governed_authority_coverage_test.go:95-110
  (failing through an unavailable observation, asserts NOT exhausted: 0 +
  0 + 1 < 10) keep their assertions; without the wiring the second would
  fail closed into exhaustion. The other Store literals that conclude
  governed runs (dispatch/governed_test.go five sites,
  gaterun/weight_test.go:121-125, gaterun/weight_authority_coverage_test.go:30-37,
  steward/governed_test.go:21) wire `ProjectSpend` to
  `dispatch.SettledSpendAtConclusion` wherever they already wire
  `ObserveGoverned` to `dispatch.ObserveGovernedRun`, and to the
  known-zero stub elsewhere; their green proofs are unaffected either way
  (exhaustion needs a failing attempt).
- `TestPublishedSetupRetainsAttemptAndReservedMinutes`
  (budget_test.go:97-116): a pending-setup husk charges its cap 30.
  Unchanged.
- `TestBudgetAdmissionClosesAtEveryCurrentEqualityBoundary`
  (budget_test.go:198-214) and stop_test.go:209-211 construct projection
  literals; new fields default to zero and the assertions read only
  `Field`. Unchanged.
- `TestRecordCASRefusesImmutableField` (record_test.go:196-204) and every
  other `RecordCAS` test: no existing patch carries `endedAt` (grep
  `"endedAt"` in internal/dispatch/*_test.go patches: none). Unchanged.
- Lease sweep tests (sweep_test.go:16-60, 118-200; refusals_test.go:345-400)
  stub `sweepKill` and the process-table seams; they gain a default
  `sweepGroupAbsent` stub that returns absent, which reproduces today's
  immediate stamp, so their assertions are unchanged. T14 scripts the
  seam explicitly.
- evidence/gc_test.go:259 (`spent`, completed, no timestamps, no pid):
  `before` is `BudgetUnknown` today because `empty-goal.json` (line 273)
  trips budget.go:315-316 first, and the test asserts only `before ==
  after`. Unchanged.
- stop_test.go:102-105 and 132-135 (`cancelled`, no pid): settle to 0;
  those tests assert batch states, not minutes. Unchanged.
- steward/health_test.go:267-268, 278, 389: open records only. Unchanged.

**Shell and fixture beds asserting the old wording: none.** Under
metasystem/scripts/agents the string `proposed` occurs only as the shell
variable in dispatch.sh:587-621 and in roles/orchestrator.md prose; the
`"%d+%d proposed"` breach text is kept in any case. The only refusal-text
assertions are `attemptLimit used=1 limit=1` (dispatch-fixtures.sh:1146,
attempt limit 1 at line 1092, reserved limit 10000) and `admission closed:
elapsedLimit` (goal-cli-fixtures.sh:409, 552); both remain substrings of
the new lines. The `structured-budget-within` job in that bed is a real
fake-runtime dispatch whose record carries engine-stamped `pid`,
`ownershipProof.provenAt`, `startedAt`, `endedAt`, so it settles to 1 and
the attempt boundary still closes admission.

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

The reaper's kill line (`CapExpired`, reapfacts.go:134-152) and its
no-kill-authority rule, `capMin` immutability (record.go:64), the
ownership write and its one-shot rule (ownership.go:138-166), the census's
identity facts (`pid`, `pidStartedAt`, which the settlement does not
read), the slice-norm admission on the cap (goal/norm.go), the attempt and
active-job counting (budget.go:362-375), the elapsed accounting, the
governed-run projection path (budget.go:377-488), the `BUDGET_UNKNOWN` and
exit-code contract of the admission verbs, the steward's health renderer,
dispatch.sh's own wind-down ladder, and the mission fence ledgers.

## 8. Self-grade

- **Confidence: high** on the charge rule, the transition ownership of
  `endedAt`, the one-clock start instant, the death ladder for the sweep
  and the re-projection at conclusion (every claim is a read line in this
  worktree; the specimen arithmetic is from the live records, whose
  ownership stamp lies 0 to 1 second after `startedAt`). **Medium** on
  the governed fixture list: the six Store literals were found by grep
  for `AdmitGoverned` and `ObserveGoverned` assignments; a fixture that
  builds its store through a helper in another package would escape it,
  and would then fail closed into exhaustion rather than pass silently.
- **Residual, recorded not built (KI-45, memory/known-issues.md line 53):**
  a dispatcher that dies between spawning the detached runtime
  (dispatch.sh:812-820) and landing the ownership patch (dispatch.sh:853)
  leaves a terminal record with no process identity, and the settlement
  charges it 0 minutes although the runtime ran for seconds. The bound:
  one job's seconds-to-minutes under-charged per such crash, never an
  over-charge and never a refusal of lawful work; the old rule charged the
  full cap in the same case.
- **Residual, recorded not built (the KI-37 family, memory/known-issues.md
  line 44):** the start and end stamps share one wall clock, so a host
  clock step DURING a job shifts that job's charge by the size of the
  step, bounded above by the clamp to `capMin` (1.6) and below by the
  one-minute floor. No monotonic cross-process measure exists and none is
  built; the census's clock-step-immune pair (`startTicks` plus `bootId`)
  proves identity, not duration.
- **Weakest claim:** that the sweep's 4-second ladder is long enough on a
  loaded machine. dispatch.sh has used the same 2-plus-2 seconds since the
  ladder was written (dispatch.sh:343-352) and a survivor is a refusal
  the next claim retries, never a false stamp, so the failure direction is
  a delayed takeover rather than a wrong settlement.
- **Reject condition, re-run for revision 3:** reject if any lawful writer
  patches `endedAt` through `RecordCAS` on an open record (none found);
  if a terminal job-record writer exists that neither goes through
  `RecordCAS` nor stamps `endedAt` itself, or stamps it without death
  evidence after 1.9 (the set found is the four in 1.5 and, after the
  amendment, each acts on death or self-report); if any job-record writer
  clears or rewrites `pid` or `ownershipProof` on a launched record
  (`validateOwnershipPatch` forbids the rewrite and no writer clears them);
  if a shipped launcher writes an ownership proof without `provenAt`
  (ownership.go:163-166 refuses it); if any path concludes a governed run
  with the record already terminal on disk (then the exclusion by run id
  would double-count nothing but the durable copy would already exist:
  conclude.go:329-337 publishes it inside the same callback, so none was
  found); or if a validation bed asserts the bare `BUDGET_REFUSED: ...
  admission closed` line with nothing after it (none does). Any of these
  reopens the design before build.
