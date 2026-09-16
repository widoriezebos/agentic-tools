# Design: proof admission fits a seat proving several units (r1)

Goal: proof-admission-fits-a-seat-proving-several-units (tier 2, priority 1,
sequence 30). Brief: S5/proof-admission-fits-a-seat-proving-several-units-
design-brief-r1.md. Code citations are origin/main e70a832e2, paths under
metasystem/. Every rule names the witness that fails without it (Wido
2026-09-14). Tests run on injected clocks (Wido 2026-09-12). No unit lowers a
floor, removes a witness or gate, or narrows the DONE (R-115-m1e rule 6). No
unit touches hooks, deny mode or tmux (R-118-m1e item 1, R-114-m1e item 2).

## 1. The problem, with evidence

Three refusals hit a seat that proves units of several goals under one claim.

1.1 Attempts are charged to the machine's claim, never to the candidate's
goal. A top-level `test run` resolves its goal from `--goal` or from the
machine's single live claim (proof_run.go:409-416, test.go:1362-1382); the
binding requires that goal to be claimed (stop.go:68-73, "not a claimed
accepted goal"); the admission verdict is evaluated on that goal
(proof_run.go:536-537) and the projection counts every retained attempt whose
`GoalID` equals the file id (budget.go:517-520, 575). So `--goal` cannot name
an unclaimed goal, and every proof the seat runs, for whichever goal's
candidate, counts against the claim's `attemptLimit` (admission.go:384-386).
Evidence: on 2026-09-15 other units' proofs and controls spent
coordinator-context-stays-under-budget's six attempts and blocked unit C1b
(goal intent); tonight m1e bound the 8d-page and rulings proofs to
seats-spend-tokens-in-bounded-sessions, then released it and claimed
coordinator-context-stays-under-budget to build units (seat relay).

1.2 Retry decisions are ambiguous across candidates. `componentDecisionLocked`
collects, per component id, the newest attempt whose planned identity for
that id matches (attempt.go:653-682) among attempts of the same goal and
accounting revision (650-652); a group's execution identity is the same
across two candidate trees when the group's inputs did not change, so a
failure under candidate T1 stays the newest observation of that group while
the seat proves candidate T2. Two components whose newest failures come from
two attempts give `len(failedIDs) == 2` and the refusal "component retry
decision is ambiguous across 2 failed attempts" (711-712), even for one
section. The attempt records no candidate tree; the tree is only the first
element of `IdentityInputs` (test.go:814).

1.3 A breach-stopped goal cannot be re-budgeted in one step, and the run is
refused afterwards. set-budget refuses while fenced (verbs.go:1206-1208,
"only goal resume with its standing approved budget may reopen admission");
resume refuses a changed tuple (stop.go:433-438, "APPROVAL_REQUIRED: resume
cannot change the human-approved budget; ... use the proof-bearing goal
set-budget before resuming"). The two verbs point at each other. A resume
with the same tuple lifts the fence but the accounting revision stays, so
the spent limit refuses the next run; the seat then releases (refused while
fenced, verbs.go:481-482) and re-claims to reset the episode (m1c, about 40
minutes). Separately, a human set-budget from outside the lease carries
`req.ClaimEpoch = 1` (goalsync_mutations.go:529-533) and the set-budget tail
rebinds the claim with it (verbs.go:1240-1249, `bindClaim` at 327-338), so
the capability's ClaimEpoch is overwritten with 1 while the lease holder is
at a later epoch; the holder gate then refuses "active coordinator does not
own the claimed goal reservation" (proof_run.go:495-501). This is the
5-versus-1 divergence stop-capability-follows-the-lease-epoch reproduces.
Assumption A1: for a human caller with no lease, `classification.ClaimEpoch`
is nil, so the fallback to 1 is taken; the fallback branch is verbatim at
goalsync_mutations.go:531-533, the nil condition is inferred.

## 2. Decisions

D1. An attempt carries an accounting goal apart from its authority goal.
`Attempt` gains `AccountedGoalID` and `AccountedRevision` (json
`accountedGoalId`, `accountedRevision`, omitempty). Empty means "the
authority goal and its accounting revision" (`GoalID`, `AccountingRevision`),
so every retained record reads as today. The authority chain (claim, stop
capability, ClaimEpoch, fence, stop batch, goal-revision lock) stays on the
machine's claimed goal; the budget count (attempts, reserved minutes) and the
retry and reuse history key on the accounting pair.

D2. The seat names the accounting goal with `test run --charge <goal-id>`.
`--goal` keeps its meaning (the accepted goal owning delivery, claimed on this
machine). `--charge` defaults to the authority goal, so a run without it
behaves as today. The charged goal must be an open, approved goal with a
budget, not done, not parked, not breach-stopped; when it is claimed on this
machine (a second claim, see open question Q1) its claim's accounting
revision is the accounted revision; when it is unclaimed, the accounted
revision is the revision at which its budget tuple was last set or approved
(the budget episode). A run for a candidate of the claimed goal itself needs
no flag.

D3. Gates split by what they protect. The authority goal keeps the fence,
elapsed and active-job gates (a seat past its claim's elapsed limit still
renews under R-117-m1e item 3 or R-119-m1e). The accounting goal keeps the
attempt and reserved-minutes gates. `EvaluateGoalRevisionAdmissionForDispatch`
runs once per goal with the breach list filtered to that goal's gates; the
refusal names which goal and which limit refused.

D4. Retry decisions are scoped to the candidate tree. `Attempt` gains
`CandidateTree` (json `candidateTree`, omitempty), set from
`prepared.CandidateTree` at launch. In `componentDecisionLocked` a failed
observation from an attempt whose `CandidateTree` differs from the request's
tree is dropped (the component executes afresh); a success observation is
reused as today (the group's execution identity is the same work), and a
live observation stays a live duplicate as today. A retained attempt without
a candidate tree is treated as the request's own tree, so old failures still
demand a decision (no floor lowered). The retry decision's prior attempt must
be on the request's tree and accounting pair.

D5. set-budget on a breach-stopped claimed goal lifts the fence in the same
transaction. Preconditions are resume's own: the stop batch is complete
(`VerifyStopBatchComplete`, stop.go:449), the machine holds no other live
claim (stop.go:453-467), and the act carries human authority (set-budget
already requires `--by` and a proof, or `--under` within the box). The
mutation runs resume's steps (stop.go:448-479), then the budget steps
(verbs.go:1216-1249) with the new tuple, binding the new revision so the
attempt count restarts as a set-budget does today
(goal-cli-fixtures.sh:1596-1604). One history op, `set-budget`, with
`resumed=<stopId>`. The same tuple on a fenced goal refuses and points to
`goal resume`, which keeps its own approval token. Over-norm tuples still
need `--approved-ref` and a rulings row (R-119-m1e item 1).

D6. A budget or resume act never overwrites a live claim's ClaimEpoch with
the caller's default. The set-budget tail keeps `f.StopCapability.ClaimEpoch`
unless the caller is the authenticated lease holder of the claim's machine
(`classification.Holder` with a non-nil epoch); the `1` fallback at
goalsync_mutations.go:531-533 stays for acts that create a claim and never
reaches a rebind. Resume already keeps the recorded epoch (stop.go:452). This
is the "claim's reservation follows" half of the DONE. Coordination with
stop-capability-follows-the-lease-epoch: this goal lands the stamping rule
(unit U5); that goal owns restamping already-diverged records on `up` or a
reconcile verb, the health role and the 5-versus-1 reproduction. Q3 asks the
seat to sequence the two.

D7. Nothing changes for governed runs and native delegates: their goal is the
run's or the job's (proof_run.go:346-408) and `--charge` is refused there
("a governed or delegate proof is charged to its run's goal").

## 3. Rules and their witnesses

Each witness is a test or fixture that fails on origin/main or fails when the
rule's code is removed by mutation (the builder proves each by mutation).

R1. An attempt's accounting goal is the charged goal, or the authority goal
when none is named; the budget projection counts an attempt against its
accounting pair only. Witness W1 `TestChargedAttemptCountsAgainstTheCharged
GoalNotTheClaim` (proof_run_test.go): claim on C with attemptLimit 1, goal X
approved and unclaimed with attemptLimit 1; launch `--charge X` admitted; a
second `--charge X` refused naming X's attemptLimit; a launch without
`--charge` admitted on C. Fails without R1: the second launch is charged to
C, the third is refused.

R2. The authority chain does not move: a charged launch still passes the
holder gate on the authority goal and the record's `goalId` is the authority
goal. Witness W2 `TestChargedLaunchStillNeedsTheClaimHolder`: `--charge X`
from a caller that is not the lease holder is refused with "active
coordinator does not own the claimed goal reservation"; the admitted record
of W1 has `goalId == C`, `accountedGoalId == X`. Fails without R2 when the
charged path resolves the binding on X.

R3. A charged goal must be open, approved, budgeted, not done, not parked,
not fenced; the refusal names the state. Witness W3 `TestChargeRefusesA
ParkedOrFencedGoal`: parked X refused "charged goal X is parked"; fenced X
refused naming the stop id. Fails without R3: the launch is admitted.

R4. Retained records without the new fields read as today. Witness W4 in
`attempt_test.go` beside `TestAttemptSchemaTwoAtomicallyRetainsTestingAnd
ReadsSchemaOne`: a record without `accountedGoalId`, `accountedRevision` and
`candidateTree` yields accounting pair `(GoalID, AccountingRevision)` and is
counted by the projection, and its failure still demands a retry decision.
Fails without the defaults.

R5. The retry decision ignores failed observations from other candidate
trees; successes reuse across trees; live duplicates are unchanged. Witness
W5 `TestComponentRetryDecisionIgnoresAnotherCandidatesFailure`: attempt A on
tree T1 fails group g; attempt B on tree T2 fails group h; a request on T2 for
{g, h} with a retry decision naming B is not ambiguous, g executes afresh, h
retries B. Fails on origin/main with "ambiguous across 2 failed attempts".
W5b: A's success on T1 for group g is reused on T2. Fails if D4 drops
successes.

R6. The retry decision's prior attempt is on the request's tree and
accounting pair. Witness W6: a decision naming an attempt on T1 for a request
on T2 is refused "retry decision prior attempt is outside the same candidate
tree". Fails without R6 (readRetryDecision checks goal and identity only,
attempt.go:850).

R7. set-budget on a breach-stopped claimed goal with a changed tuple lifts
the fence, binds the new revision and reopens admission in one op. Witness
W7 (goal-cli-fixtures.sh, after line 1604's pattern): drive a claim to its
attempt boundary, `job breach-stop`, then `goal set-budget` with a larger
tuple `--by Wido --fixture-human-authority`; the rendered record has the new
Budget, no StopFence, one new history op `set-budget ... resumed=stop-...`;
`job goal-admission` returns 0. Fails on origin/main at verbs.go:1206.

R8. set-budget on a fenced goal refuses while the stop batch is incomplete or
the machine holds another live claim, naming the batch or the claim as
resume does. Witness W8 (fixture): batch incomplete, refusal names the stop
id and `metasystem job stop-batch`. Fails without R8: the fence lifts with
stop work pending.

R9. set-budget with the same tuple on a fenced goal refuses and names `goal
resume`. Witness W9 (fixture). Fails without R9: NothingToDo hides the fence.

R10. A budget or resume act by a caller who is not the claim's lease holder
keeps the capability's ClaimEpoch; the holder's own act stamps the holder's
epoch. Witness W10 `TestSetBudgetOutsideTheLeaseKeepsTheCapabilityEpoch`
(cmd/metasystem or internal/goal): claim at epoch 5; a human set-budget
request with `ClaimEpoch 1` leaves `StopCapability.ClaimEpoch == 5`; a
holder request at epoch 6 stamps 6. Fails on origin/main (restamped to 1).
W10b: after W7's set-budget, the proof gate admits a launch by the holder
without a release or re-claim. Fails without R10.

R11. `--charge` is refused for governed and delegate proofs. Witness W11:
`METASYSTEM_PROOF_RUN_ROOT` set and `--charge X` refused. Fails without R11.

R12. The refusal texts introduced here are registered (register.go:80 style)
with owner and site. Witness W12: the existing register test (or a new one)
lists the new codes `CHARGE_REFUSED_STATE`, `CHARGE_REFUSED_CONTEXT`,
`RETRY_PRIOR_OUTSIDE_TREE`, `SET_BUDGET_FENCED_SAME_TUPLE`. Fails when a
code is missing (Q7 confirms the register's test).

## 4. Units

Changed-line allocation counts additions plus deletions, tests included; at
most 300 per unit.

| Unit | Files (tests) | Lines | DONE | Witness | After |
|---|---|---|---|---|---|
| U1 accounting pair on the attempt | internal/proofrun/attempt.go (attempt_test.go); internal/dispatch/budget.go (budget_test.go or a new budget_accounting_test.go) | 200 | `AccountedGoalID`, `AccountedRevision` with `accountingPair()` accessor, validation, the projection and `componentDecisionLocked` keyed on the pair; defaults keep old records | W4; a projection test with two attempts of one goalId and two accounted goals | none |
| U2a charged launch: flag, resolution, stamping | cmd/metasystem/test.go, proof_run.go (proof_run_test.go) | 260 | `--charge` parsed and carried in `proofLaunchAdmission.ChargeGoalID`; charged goal resolved and checked (R3, R11); attempt stamped; authority gate unchanged | W2, W3, W11 | U1 |
| U2b split admission verdict | cmd/metasystem/proof_run.go, internal/dispatch/admission.go (admission_test.go, proof_run_test.go) | 240 | the verdict runs per goal with gates filtered per D3; an unclaimed charged goal's projection uses its budget episode (Q2); refusal names goal and limit | W1; a test that C past its elapsed limit still refuses a `--charge X` launch | U2a |
| U3 candidate-scoped retry | internal/proofrun/attempt.go (attempt_test.go); cmd/metasystem/test.go | 200 | `CandidateTree` recorded; D4 filtering; R6 in `readRetryDecision` | W5, W5b, W6, W4's tree half | U1 (field placement only; may build in parallel) |
| U4 one-step set-budget on a fenced goal | internal/goal/verbs.go, internal/goal/stop.go (verbs or stop tests); scripts/agents/goal-cli-fixtures.sh | 280 | D5: shared `liftFenceForRebudget` used by set-budget; R7-R9; history marker | W7, W8, W9 | U5 |
| U5 capability epoch follows the holder | internal/goal/verbs.go:1240-1249, cmd/metasystem/goalsync_mutations.go:529-533 (goalsync_mutations_test.go or a goal package test) | 120 | D6: rebind keeps the recorded epoch unless the caller is the holder; attorney path carries the holder's epoch (Q4) | W10, W10b | none; coordinate with stop-capability-follows-the-lease-epoch (Q3) |
| U6 docs and refusal register | docs (the test-run and goal-verbs pages), internal/refusal/register.go (its test) | 100 | `--charge`, the split gates and the one-step set-budget documented; new codes registered | W12 | U2b, U4 |

Total allocation: 1,400 changed lines over seven units. Build order: U1,
U5 in parallel; then U2a, U3, U4; then U2b; then U6. Each unit lands through
R-117-m1e item 5's lane (Opus read with no NOT LAND, green selected groups,
receipt in the same commit).

## 5. What the seat does after this lands

A seat holding claim C and proving a unit of goal X runs
`metasystem test run --charge X ...` on X's candidate. The attempt is
recorded under C's authority and X's budget; C's attempt count is untouched;
X's count grows on this checkout. A retry after a failure on X's candidate
is decided on that candidate's failures alone. When a claim breach-stops,
the human (or a seat under R-119-m1e) runs one `goal set-budget` with the
new tuple; the fence lifts, the revision rebinds, the capability keeps the
holder's epoch, and the next run admits.

## 6. Open questions (gaps left open, never filled silently)

Q1. Does `goal claim` refuse a second live claim on one machine? Resume does
(stop.go:453-467) and `uniqueActiveProofGoal` handles several (proof_run.go:
610-613), but no such refusal was found in verbs.go by phrase. D2 covers both
branches; the builder confirms which and drops the dead branch.

Q2. Does `ProjectBudget` return `BudgetKnown` for an approved goal without a
claim? `obligationBudgetStart` (budget.go:87) takes the episode from the
claim; U2b assumes a small branch that starts an unclaimed charged goal's
episode at its budget's set or approval revision. If the projection needs
more than about 60 lines, U2b splits.

Q3. Sequencing with stop-capability-follows-the-lease-epoch: this design
proposes that U5 lands first and that goal builds its `up`/reconcile restamp
and health role on it; the seat (m1e) decides, and that goal's design page
cites U5's helper by name.

Q4. The attorney path (`--under`, goalsync_mutations.go:1770) builds its
request with `syncReq(name, root, "", lineage)`; what `ClaimEpoch` does it
carry? If 0, `bindClaim` refuses "requires the authenticated lease holder's
positive claim epoch" (verbs.go:328-330) on any rebind; U5 must cover it.

Q5. Does `job prove-round` launch `test run` itself, and does its round
record name the candidate's goal? If so `--charge` passes through and can
default from the record (prove_round.go was read at 33-137 only).

Q6. The m1c sequence: confirm from the transcript that it was set-budget
refused (verbs.go:1206), resume refused APPROVAL_REQUIRED (stop.go:438),
resume with the same tuple, run refused, release, re-claim. D5 and D6 cover
each step as read; a different step changes nothing in the rules but should
be in the page's evidence.

Q7. Does internal/refusal/register.go have a test that every refusal text in
the tree is registered? W12 depends on its shape.

Q8. Elapsed on the accounting goal: D3 leaves an unclaimed charged goal
without an elapsed gate (its clock starts at claim). If Wido wants a bound,
a wall of reserved minutes on X already caps spend; a ruling is needed only
if that is not enough.

## 7. Assumptions

A1 (section 1.3): a human caller outside the lease has a nil
`classification.ClaimEpoch`, so `req.ClaimEpoch` falls to 1.
A2: attempt records are per checkout (`proofrun.ReadAttempts(repoRoot)`,
budget.go:513), so charging X on this checkout never counts attempts made on
another seat's checkout; the limit stays a per-checkout spend fence.
A3: a group's execution identity is unchanged across two candidate trees
when its inputs are unchanged (the reuse path at attempt.go:673-678 depends
on it); this is why D4 drops only failures and keeps successes.
A4: the fixture bed `goal-cli-fixtures.sh` runs through the engine and
accepts `METASYSTEM_GOAL_NOW` (lines 1586, 1595); W7-W9 add to it rather
than to a standalone script.
A5: line numbers hold at origin/main e70a832e2; the checkout's cited
directories showed no diff against it.
