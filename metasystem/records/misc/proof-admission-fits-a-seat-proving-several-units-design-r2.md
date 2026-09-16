# Design: proof admission fits a seat proving several units (r2)

Goal: proof-admission-fits-a-seat-proving-several-units (tier 2, priority 1,
sequence 30). Revision 2 folds critique r1 (9 material findings, verdict
rework). Code citations are origin/main dacadefe0 read on 2026-09-16, paths
under metasystem/; `git diff --stat e70a832e2 origin/main -- cmd internal
scripts` touches only internal/spend, so every cited line is identical to
r1's e70a832e2 and the critique's 912ea6f11 (PA-010 corrected). Every rule
names the witness that fails without it (Wido 2026-09-14). Tests run on
injected clocks and injected barriers, never wall time or sleeps (Wido
2026-09-12). No unit lowers a floor, removes a witness or gate, or narrows
the DONE (R-115-m1e rule 6: "no unit of the efficiency program lowers a proof
floor, removes a witness or a gate, or narrows a DONE to save tokens"). No
unit touches hooks, deny mode or tmux (R-118-m1e item 1: "Units that turn on
deny mode or change hook settings beyond observe mode still wait"; R-114-m1e
item 2: "No machinery may assume or rely on tmux"). Builds are approved in
Wido's name under R-119-m1e item 1 once this page lands with the critique
closed (R-119-m1e item 2 keeps R-117-m1e items 2 and 5 as quality gates).

## 0. Findings table

| Finding | Disposition | Carried by |
|---|---|---|
| PA-001 plan risk from the wrong goal | FOLDED | D2, R1, U3a (W1) |
| PA-002 unclaimed charge erased at claim | FOLDED | D3, R2, U2 (W5) |
| PA-003 one projection key, two gates | FOLDED | D4, R3, U2, U3c (W4) |
| PA-004 no linearization rule | FOLDED | D5, R6, U3b (W7) |
| PA-005 holder epoch not authenticated | FOLDED | D7, R12, U6 (W14) |
| PA-006 legacy tree rule keeps the defect | FOLDED | D6, R8, U1 (W8, W9) |
| PA-007 readers omitted | FOLDED | section 3.1 inventory, U5a, U5b (W12, W13) |
| PA-008 `resumed=` has no schema owner | FOLDED | D8, R14, U7 (W15) |
| PA-009 rules without full witnesses | FOLDED | W3 table, W10c, W16b-c, W4 |
| PA-010 stale provenance | FOLDED (not material) | header |
| Q1-Q8 recommended answers | FOLDED, all eight adopted | section 6 |
| Seat ruling on Q3 (U5 of r1 lands first) | FOLDED | D7, U6, section 7 |
| NEW-FROM-FOLD: `--charge` replaced by candidate `--goal` plus `--authority` | NEW-FROM-FOLD | D2 |
| NEW-FROM-FOLD: the automatic raise must not start a consumption episode | NEW-FROM-FOLD | D3, A3 |
| NEW-FROM-FOLD: trusted base engine rollout order | NEW-FROM-FOLD | section 5 |
| NEW-MISSED: attorney set-budget on a claimed goal is refused today | NEW-MISSED | section 1.3, D7 |

## 1. The problem, with evidence (verified at origin/main)

1.1 Proofs are charged to the claim, not the candidate's goal. A top-level
`test run` resolves its goal from `--goal` or the machine's single live claim
(proof_run.go:404-411; test.go:213-218, 1362-1382); `ResolveGoalBinding`
requires a claimed goal (proof_run.go:430); the plan is selected with that
goal's risk, which `testingGoalRisk` reads only from a claimed goal
(test.go:310-315, 1338-1360, "no accepted risk and accounting lineage"); the
trusted engine gets the same `--goal` (test.go:431-434); the admission
verdict runs on that goal (proof_run.go:536-537) and the projection counts
every attempt whose `GoalID` equals the file id and whose
`AccountingRevision` is at or past the claim's (budget.go:517-526, 571-577).
`Claim` binds `AccountingRevision` to the claim revision (verbs.go:323-325,
991), so a re-claim starts a fresh count. Evidence: goal intent (2026-09-15,
unit C1b blocked) and the seat relay of 2026-09-15 night.

1.2 Retry decisions cross candidates. `componentDecisionLocked` keeps, per
component, the newest attempt of the same goal and accounting revision whose
planned identity matches (attempt.go:650-682); a failure under tree T1 stays
newest while T2 is proved; two such failures give "ambiguous across 2 failed
attempts" (711-712). The tree is retained only as `IdentityInputs[0]` for
testing attempts (test.go:814-815) and as `TestResult.CandidateTree`
(test_result.go:181).

1.3 The fenced goal's two verbs point at each other. set-budget refuses while
fenced (verbs.go:1206-1208); resume refuses a changed tuple (goal/stop.go:
437-438) and keeps the recorded epoch (452, 477). Separately, the command
edge sets `req.ClaimEpoch` to the live lease epoch for any caller class when
a lease file exists and to `1` for a HUMAN caller when none does
(goalsync_mutations.go:529-533; lease/verbs.go:372-376 hands out
`&lease.ClaimEpoch` whenever `lease != nil`, so A1 of r1 is now a fact: the
nil branch is the no-lease case). The set-budget tail replaces the recorded
epoch with any positive `r.ClaimEpoch` (verbs.go:1240-1249); `1` is positive,
so a human set-budget run while no seat held the lease stamps 1 over the
holder's 5, and the holder gate then refuses (proof_run.go:495-501). This is
the 5-versus-1 divergence's cause. The attorney path (`--under`,
goalsync_mutations.go:1770, `syncReq(name, root, "", lineage)`) carries
`by == ""`, so a MAIN non-holder process gets `ClaimEpoch 0`, the tail's
fallback needs `Actor.Human != ""`, and `bindClaim` refuses "requires the
authenticated lease holder's positive claim epoch" (verbs.go:328-330): an
attorney set-budget on a claimed goal fails today unless run by the holder.

## 2. Decisions

D1. Every attempt carries two goal references. Authority: `GoalID`,
`GoalRevision`, `AccountingRevision`, `BudgetEpoch` as today (the claim the
launch runs under). Candidate: new fields `CandidateGoalID`,
`CandidateRevision`, `CandidateBudgetEpoch` (`*uint64`), `CandidateTree`
(json `candidateGoalId`, `candidateRevision`, `candidateBudgetEpoch`,
`candidateTree`, omitempty). Accessors on `Attempt`: `AccountedGoal()`
(candidate id, else `GoalID`), `AccountedRevision()` (candidate revision,
else `AccountingRevision`), `AccountedBudgetEpoch()` (candidate epoch, else
`BudgetEpoch`), `CandidateTreeDigest() (string, bool)` (D6). Every reader in
section 3.1 calls an accessor, never a raw field pair.

D2. `--goal` names the candidate's goal; authority is resolved apart. `--goal
X` keeps its documented meaning, "accepted goal owning delivery" (test.go:
152): X supplies risk, required mode, the trusted engine's `--goal`, the
consumption episode and the retry and reuse history. Authority is the claim
the launch runs under: X itself when X is claimed live on this machine
(today's path, unchanged); otherwise the machine's unique live claim
(`uniqueActiveProofGoal`, proof_run.go:590-632); when the machine holds
several live claims (verbs.go:943-1000 checks only the target goal, so a
machine may claim several, Q1), the new flag `--authority <goal-id>` names
one, and it must be a live unfenced claim of this machine. `job prove-round`
already passes the job goal as `--goal` (prove_round.go:99-100) and needs no
change to its call (Q5). Governed and native-delegate proofs keep candidate
== authority == the run's goal (proof_run.go:346-408) and refuse
`--authority`. r1's `--charge` is withdrawn: it made the low-risk claim
select the plan (PA-001).

D3. The consumption episode is the human's budget act, not the claim.
`goal.BudgetEpisodeRevision(f) uint64` returns `f.Approved.Revision` when
`f.Approved != nil && f.Budget != nil`, else 0; `bindApproval` stamps
`Revision: f.Revision` at every approve and set-budget (approval.go:427-436),
so the value moves only when a human binds a tuple. The automatic
consumption-earned raise rewrites `Approved` too (verbs.go:2670) and must
keep the prior `Revision` (a raise lifts a limit inside the episode, it never
restarts spend; A3). A launch stamps `CandidateRevision =
BudgetEpisodeRevision(X)` and `CandidateBudgetEpoch` = X's weight epoch.
Consequence, stated: within one budget episode a release and re-claim no
longer resets a goal's attempt count; the remedy is `goal set-budget`
(approved in Wido's name under R-119-m1e). This raises a floor and lowers
none (R-115-m1e rule 6).

D4. Two lenses over one attempt store. Authority lens (claimed goal C):
elapsed from the claim episode as today; active jobs and open cap over
attempts with `GoalID == C`, `AccountingRevision >= C.Claimed.
AccountingRevision` and today's epoch rules (budget.go:517-538). Consumption
lens (any budgeted goal X, claimed or not): attempts and reserved or observed
minutes over attempts with `AccountedGoal() == X`, `AccountedRevision() >=
BudgetEpisodeRevision(X)`, `AccountedBudgetEpoch()` matching X's weight epoch
(nil matches nil; a non-nil epoch against a goal without one is
`BudgetUnknown`, as budget.go:531-532 today). One live attempt launched under
C for X's candidate counts in C's active jobs and in X's attempts and
minutes, and in nothing else. `ProjectBudget` (claimed goals) computes its
attempt and minute members with the same predicate as the new
`ProjectConsumption(repoRoot, file, now) ConsumptionProjection` so the two
never disagree; `ProjectConsumption` answers for an unclaimed approved goal,
which `ProjectBudget` refuses (budget.go:266-268, Q2). Gates: the authority
goal keeps `elapsedLimit` and `activeJobLimit`; the candidate goal keeps
`attemptLimit` and `reservedJobMinutesLimit` (admission.go:369-394 split by
lens); each refusal names the goal and the limit. Old records: candidate
fields empty, so both lenses read `(GoalID, AccountingRevision)` and count
exactly what they count today, plus attempts from an earlier claim inside the
same budget episode (the D3 tightening).

D5. Linearization: authority lock, candidate lock, attempt lock, re-read. A
launch reads X once before locking (state, budget, episode revision, claim),
then acquires in this fixed order: `goalrevision.Acquire(root, C,
C.revision, "proof-admission")` (proof_run.go:453), `goalrevision.Acquire(
root, X, X.revision, "proof-admission")` when X != C, then
`proofrun.AcquireMutation` (458). Under the three locks it re-reads C (as
today, 505-508) and X; the re-read is the admission snapshot; if X's
revision, state, fence, claim or `BudgetEpisodeRevision` differ from the
pre-lock read, the launch is refused `CANDIDATE_GOAL_MOVED` naming both
revisions (the same shape as the delegate check at 434-436). A goal
transition that lands before the re-read wins (refusal); one that lands after
publication finds the attempt already charged to the episode it read, and a
later set-budget starts a new episode without touching it. The seam
`proofAdmissionUnderLocks func()` (package var, nil in production) runs after
the locks and before the re-read so a test can land a transition there.

D6. One candidate-tree accessor, validated. `CandidateTreeDigest()` returns
the `CandidateTree` field when set; else `TestResult.CandidateTree` when a
result is retained; else `IdentityInputs[0]` when `CommandClass == "testing"`
and it is a valid tree digest (test.go:813-815 puts the tree first); else
`("", false)`: unidentifiable. Validation (beside `validateProofIdentity`,
attempt.go:499) refuses a record whose field disagrees with either derived
source. In `componentDecisionLocked` a failed observation whose tree is known
and differs from the request's tree is dropped; a success is reused as today
(same execution identity, A2); a live observation stays a live duplicate;
an unidentifiable legacy failure is treated as the request's own tree, so it
still demands a decision (fail closed). The retry decision's prior attempt
must be on the request's tree and accounting pair.

D7. Epoch replacement needs an authenticated holder fact. `VerbRequest` gains
`EpochAuthority string`; the one site that sets it is
`syncReqClassifiedWithTerminalGrade`, `EpochAuthorityHolder` exactly when
`classification.Holder && classification.ClaimEpoch != nil`
(goalsync_mutations.go:529-533 keeps its number, gains the fact). Domain
helper `goal.ClaimEpochForRebind(f *GoalFile, r VerbRequest) (int64, error)`
in internal/goal/verbs.go: holder authority with `ClaimEpoch >= 1` returns
`r.ClaimEpoch`; holder authority with `ClaimEpoch < 1` or `CallerClass !=
lease.ClassMain` is refused as contradictory; no holder authority returns
`f.StopCapability.ClaimEpoch` when present, else refuses
`REBIND_EPOCH_UNAUTHENTICATED`. The set-budget tail (verbs.go:1240-1249)
calls it; resume keeps its recorded epoch (goal/stop.go:452) and gains
nothing. The `1` fallback stays for acts that create a claim (`claim`,
verbs.go:991); restamping a diverged record from the live lease on `up` or a
reconcile verb, the health line and the 5-versus-1 reproduction belong to
stop-capability-follows-the-lease-epoch, which builds on exactly these two
names: `VerbRequest.EpochAuthority` and `goal.ClaimEpochForRebind`. Seat
ruling (m1e, 2026-09-16): U6 lands first. Replay: `claimIntentArgs`
(verbs.go:492-500) records `claimEpoch` in the intent; `epochAuthority`
joins it so recovery rebuilds the same rebind (A4).

D8. One-step set-budget on a fenced claimed goal. Preconditions are resume's
own (goal/stop.go:449-467): stop batch complete, no other live claim on the
machine outside the arc. The mutation lifts the fence, then runs today's
budget steps (verbs.go:1216-1249) with the new tuple and D7's epoch, so the
count restarts as the fixture at goal-cli-fixtures.sh:1595-1604 shows. The
history line is `set-budget ... resumed=<stopId>`: `HistoryLine` gains
`Resumed string`; the parser (file.go:1769, token case beside `stopId=` at
1824) and renderer carry it; validation allows `resumed=` on `set-budget`
lines only and keeps `stopId=` for abandon and carry (file.go:854-857). The
same tuple on a fenced goal refuses `SET_BUDGET_FENCED_SAME_TUPLE` and names
`goal resume`. Over-norm tuples still need `--approved-ref` and a rulings row
(R-119-m1e item 1).

## 3. Rules and witnesses

Each witness fails on origin/main or fails when the rule's code is removed;
the builder proves each by mutation and records the mutation in the return.

R1. The candidate goal selects the plan: `testingGoalRisk(root, X)` reads
X's risk and `BudgetEpisodeRevision(X)` without requiring a claim; `Select`
and the trusted engine receive X. W1 `TestCandidateGoalSelectsThePlanRisk`
(test.go's tests): C claimed at low risk, X approved unclaimed at high risk
under a fixture policy whose selection differs by risk; `--goal X` yields
X's plan (required mode or groups). Fails when C's risk reaches `Select`.

R2. The consumption episode survives claim, release and re-claim and restarts
only at a human budget act. W5 `TestCandidateEpisodeSurvivesClaim`
(budget_test.go): two attempts charged to unclaimed X; claim X; release;
re-claim; `ProjectConsumption(X)` reports the same two attempts and minutes
at each step; a set-budget then reports zero. Fails on origin/main (the claim
advances the key). W5b: an automatic raise leaves the count unchanged.

R3. One live charged attempt counts in the authority's active jobs and the
candidate's attempts and minutes, and the gates split by lens. W4
`TestOneLiveChargedAttemptCountsForBothLenses` (proof_run_test.go): C
claimed (`activeJobLimit 1`, `attemptLimit 6`, an obligation with a weight
epoch), X approved unclaimed (`attemptLimit 1`, no epoch), distinct
revisions; launch `--goal X` and keep it live; a second launch under C is
refused naming C and `activeJobLimit`; terminate the first; a second `--goal
X` is refused naming X and `attemptLimit`; `ProjectBudget(C).Attempts == 0`.
W4b: C past its elapsed limit refuses `--goal X`. Fails without the split.

R4. The record's authority is the claim and the launch still passes the
holder gate. W6 `TestCandidateLaunchStillNeedsTheClaimHolder`: a non-holder
`--goal X` is refused "active coordinator does not own the claimed goal
reservation"; W4's record has `goalId == C`, `candidateGoalId == X`.

R5. A candidate goal must be live, approved, budgeted, not done, not parked,
not fenced, and not claimed on another machine; the refusal
`CANDIDATE_GOAL_REFUSED` names the state. W3
`TestCandidateGoalEligibilityTable`: seven rows (absent, queued, no budget,
done, parked, fenced naming the stop id, claimed on machine m2), each
refused with its state; an eighth row (approved, unclaimed) admitted.

R6. Admission is linearized at the re-read under the three locks. W7
`TestCandidateGoalTransitionUnderLockIsBeforeOrAfter`: the seam parks X
(row a) or re-budgets X (row b) after the locks; each launch is refused
`CANDIDATE_GOAL_MOVED` naming both revisions; row c lands the set-budget
after publication and the attempt keeps its `candidateRevision`. No sleep;
the seam is the barrier.

R7. Retained records without candidate fields read as today. W8
`TestAttemptWithoutCandidateFieldsReadsAsToday` beside
`TestAttemptSchemaTwoAtomicallyRetainsTestingAndReadsSchemaOne`: accessors
return `(GoalID, AccountingRevision, BudgetEpoch)`; the projection counts it;
a testing record with `IdentityInputs[0] == T1` derives T1 and is ignored as
a failure for a T2 request; a record with no source is treated as the
request's tree and demands a decision. Fails without the defaults or the
derivation.

R8. The candidate-tree field must agree with its derived sources. W9
`TestCandidateTreeFieldsMustAgree`: a record whose field differs from
`TestResult.CandidateTree` or from a testing `IdentityInputs[0]` is invalid.

R9. Retry decisions ignore other trees' failures, reuse successes, keep live
duplicates. W10 `TestComponentRetryDecisionIgnoresAnotherCandidatesFailure`:
A on T1 fails g, B on T2 fails h; a T2 request for {g, h} with a decision
naming B is not ambiguous, g executes afresh, h retries B (fails on
origin/main with "ambiguous across 2 failed attempts"). W10b: A's success on
T1 for g is reused on T2. W10c: a live A on T1 for g makes a T2 request for
g a live duplicate. Fails if D6 drops successes or live observations.

R10. The prior attempt of a decision is on the request's tree and accounting
pair. W11 `TestRetryDecisionPriorMustBeOnTheRequestsTree`: refused
`RETRY_PRIOR_OUTSIDE_TREE`. Fails without R10 (attempt.go:848-850 checks
goal and identity only).

R11. Exact reuse, moved-input diagnostics, round discovery and extension
evidence follow the candidate pair. W12
`TestExactReuseFollowsTheCandidatePair` (test_result.go:357): a success
charged to X under C's authority is reused for X and not for C; W12b the
diagnostics at test.go:1145 and 1198 pick X's newest success. W13
`TestProveRoundFindsTheAttemptByCandidate` (prove_round.go:214): the round
record names the attempt whose `AccountedGoal()` is the job goal when
authority is another claim; W13b `TestExtensionEvidenceFollowsTheCandidate`
(budget_extension.go:284): X's advancement counts the charged attempt, C's
does not.

R12. Only the authenticated holder replaces a recorded claim epoch. W14
`TestRebindEpochFollowsTheAuthenticatedHolderOnly` (goal package, four
rows): capability at 5; human with `ClaimEpoch 1`, no authority keeps 5
(fails on origin/main: restamped 1); human with `ClaimEpoch 0` keeps 5;
attorney request (`Human == ""`, `ClaimEpoch 0`) keeps 5 (fails on
origin/main: refused); holder authority at 6 stamps 6; holder authority with
`ClaimEpoch 0` is refused as contradictory. W14b (proof_run_test.go): after a
holder's set-budget the launch is admitted without release or re-claim. W14c
(goalsync_mutations_test.go): `EpochAuthority` is set for the classified
holder only.

R13. set-budget on a fenced goal with a changed tuple lifts the fence, binds
the new revision and reopens admission in one op, under resume's
preconditions. W16 (goal-cli-fixtures.sh after 1604's pattern): drive the
claim to its boundary, `job breach-stop`, `goal set-budget` with a larger
tuple `--by Wido --fixture-human-authority`; the record has the new Budget,
no StopFence, one history line `set-budget ... resumed=stop-...`; `job
goal-admission` returns 0 (fails on origin/main at verbs.go:1206). W16b:
batch incomplete refuses naming the stop id and `metasystem job stop-batch`.
W16c: another live claim on the machine refuses naming it. W16d: the same
tuple refuses `SET_BUDGET_FENCED_SAME_TUPLE` naming `goal resume`.

R14. `resumed=` is a parsed, rendered, validated history field. W15
`TestHistoryLineResumedField` (file_test.go): parse and render round-trip,
duplicate token refused, `resumed=` on a non-set-budget line refused,
`stopId=` on set-budget still refused. Fails without the grammar.

R15. `--authority` and a candidate other than the run's goal are refused in
governed and delegate contexts. W17: `METASYSTEM_PROOF_RUN_ROOT` set and
`--goal X` (X != run goal) refused `PROOF_AUTHORITY_REQUIRED` text; W17b:
`--authority` naming a goal not claimed live on this machine refused.

R16. New refusal codes have rows. W18: `TestHCL03EveryCodeRowed`
(register_test.go:20) passes and a new exact test asserts row and site for
`CANDIDATE_GOAL_REFUSED`, `CANDIDATE_GOAL_MOVED`, `PROOF_AUTHORITY_REQUIRED`,
`RETRY_PRIOR_OUTSIDE_TREE`, `SET_BUDGET_FENCED_SAME_TUPLE`,
`REBIND_EPOCH_UNAUTHENTICATED` (Q7).

### 3.1 Reader inventory (PA-007)

Authority side, unchanged: holder gate and reservation (proof_run.go:
495-528); active jobs, open cap and governed-owner checks (budget.go:
480-561, `record.GoalId != attempt.GoalID` stays authority); stop batches by
machine and epoch (dispatch/stop.go:436-470); goal-revision locking
(proof_run.go:453). Candidate side, changed: attempts and minutes (budget.go:
571-577); `componentDecisionLocked` and `repeatDecisionLocked` (attempt.go:
651, 790); `readRetryDecision` (848-850); `ExactReusableTestResult`
(test_result.go:357); moved-input diagnostics (test.go:1145, 1198);
`newestAttemptForTree` (prove_round.go:214); `proofAttemptAdvancement`
(budget_extension.go:284); plan risk (test.go:310-315, 1338). Both:
receipts read by attempt id (`CommittedDeliveryReceipt`). U5b's DONE includes
a census `grep -n "\.GoalID\b\|\.AccountingRevision\b" cmd internal
--include=*.go` with every hit classified in the unit's return; an
unclassified hit blocks its landing.

## 4. Units

Changed lines count additions plus deletions, tests included; at most 300
per unit. Each lands through R-117-m1e item 5's lane ("land.sh in your name
only after an Opus read with no NOT LAND verdict, a green run of every group
the test plan selects, and a receipt in the same commit").

| Unit | Files (tests) | Lines | DONE | Witness | After |
|---|---|---|---|---|---|
| U1 candidate fields on the attempt | internal/proofrun/attempt.go (attempt_test.go) | 240 | D1 fields and accessors; D6 `CandidateTreeDigest` and agreement validation; `AdmissionRequest` gains `CandidateGoalID`, `CandidateRevision`, `CandidateBudgetEpoch`, `CandidateTree`; `ReserveLocked` stamps them and checks `CandidateRevision <= candidate goal revision` | W8, W9 | none |
| U2 consumption lens and episode | internal/dispatch/budget.go (budget_test.go); internal/goal/approval.go, verbs.go:2670 (approval_test.go) | 280 | `BudgetEpisodeRevision`; raise keeps the episode; `consumptionMember` predicate; `ProjectConsumption`; `ProjectBudget` attempts and minutes through the predicate; old records count as today | W5, W5b | U1 |
| U3a candidate goal on the testing side | cmd/metasystem/test.go (test_test.go or proof_run_test.go) | 200 | `--authority` parsed; `testingGoalRisk` without the claim requirement, returning `BudgetEpisodeRevision`; `proofLaunchAdmission` carries `AuthorityGoalID`, candidate tree and revision | W1 | U2 |
| U3b authority resolution, eligibility, locks | cmd/metasystem/proof_run.go (proof_run_test.go) | 290 | D2 resolution; R5 table; D5 lock order, re-read, seam; R15 refusals | W3, W6, W7, W17 | U3a |
| U3c two-lens verdict | internal/dispatch/admission.go, cmd/metasystem/proof_run.go (admission_test.go, proof_run_test.go) | 220 | `EvaluateGoalRevisionAdmissionForDispatch` per lens with gates filtered per D4; refusal names goal and limit | W4, W4b | U3b |
| U4 candidate-scoped retry | internal/proofrun/attempt.go (attempt_test.go) | 200 | D6 filtering in `componentDecisionLocked` and `repeatDecisionLocked`; R10 in `readRetryDecision` | W10, W10b, W10c, W11 | U1 |
| U5a reuse and diagnostics readers | internal/proofrun/test_result.go, cmd/metasystem/test.go:1145,1198 (tests beside each) | 180 | accessors replace raw pairs; keyed on the candidate pair | W12, W12b | U1 |
| U5b round discovery, extension, census | cmd/metasystem/prove_round.go, internal/dispatch/budget_extension.go (tests beside each) | 180 | `newestAttemptForTree` and `proofAttemptAdvancement` on `AccountedGoal()`; census classified | W13, W13b | U1 |
| U6 epoch authority | cmd/metasystem/goalsync_mutations.go:525-533, internal/goal/verbs.go:197-215, 1240-1249, 492-500 (goalsync_mutations_test.go, goal tests) | 180 | D7: `EpochAuthority`, `ClaimEpochForRebind`, intent arg; the two names published for the neighbour goal | W14, W14b, W14c | none; lands first |
| U7 history grammar | internal/goal/file.go (file_test.go) | 140 | D8 `Resumed` field, parser, renderer, validation | W15 | none |
| U8 one-step set-budget on a fenced goal | internal/goal/verbs.go, internal/goal/stop.go (goal tests); scripts/agents/goal-cli-fixtures.sh | 290 | shared `liftFenceForRebudget` extracted from resume (stop.go:449-477) and used by set-budget; R13 | W16, W16b, W16c, W16d | U6, U7 |
| U9 docs and refusal register | docs (test-run and goal-verbs pages), internal/refusal/register.go (register_test.go) | 130 | `--goal` as candidate, `--authority`, two lenses, one-step set-budget documented; six codes rowed | W18 | U3c, U8 |

Total allocation: 2,530 changed lines over twelve units. Build order: U6 and
U1 in parallel; U2, U7; then U3a, U4, U5a, U5b in parallel; U3b; U3c and U8;
U9. U6 lands before U8 and before stop-capability-follows-the-lease-epoch
consumes it (Q3). Where a unit's build exceeds 300 lines, it splits at a
witness boundary and the split is recorded in the goal history.

## 5. Rollout facts

The trusted base engine (`metasystem.steward.landing-ref`, test.go:1384-1385)
plans with `--goal X`; an engine landed before U3a refuses an unclaimed X
("no accepted risk and accounting lineage"). So `--goal X` for an unclaimed
X works from the first landing after U3a lands, when the landed engine
carries it; U3a's own proof runs with `--goal` on the claimed goal and is
unaffected. The D3 tightening can close a live goal whose attempts from an
earlier claim now count; the remedy is one set-budget in Wido's name.

## 6. Open questions answered from the code

Q1 (second claim on one machine): `Claim` checks only the target goal
(verbs.go:943-1000); several live claims are possible; `--authority`
disambiguates; a candidate claimed on another machine is refused (R5).
Q2 (`ProjectBudget` for an unclaimed goal): unknown (budget.go:266-268);
`ProjectConsumption` answers (D4). Q3: ruled, U6 first (D7). Q4 (attorney
epoch): 0 unless the process is the holder; today refused at `bindClaim`;
D7 keeps the recorded epoch. Q5 (prove-round): passes `--goal` job goal
(prove_round.go:99-100), matches by goal (214); U5b matches by
`AccountedGoal()`. Q6: the transcript is corroboration only; the code paths
are the evidence. Q7: `TestHCL03EveryCodeRowed` exists (register_test.go:
20); W18 adds exact rows. Q8: elapsed stays on the authority claim, reserved
minutes bound the candidate; a second elapsed clock would need Wido.

Open, marked: OQ1. Whether any existing test asserts that a release and
re-claim resets the attempt count. The builder of U2 searches for it; if one
exists it is rewritten to assert continuity and the read names R-115-m1e
rule 6 (a raised floor). OQ2 (for Wido, not blocking): whether he wants the
re-claim reset kept for some case; the page decides against it (D3).

## 7. Assumptions

A1 resolved into fact (section 1.3). A2: a group's execution identity is
unchanged across trees when its inputs are unchanged (attempt.go:673-678
reuse depends on it); D6 drops only failures. A3: no reader needs
`Approved.Revision` to be the raise's own revision; the U2 builder greps
`Approved.Revision` readers before keeping the prior revision, and if one
does, `BudgetEpisodeRevision` reads the pre-raise revision from
`BudgetExtensionRecord` instead (adding it there if absent), same unit.
A4: recovery replays verbs from intents (verbs.go:940-941 comment); the
`claimEpoch` intent arg (492-500) shows the pattern `epochAuthority`
follows. A5: attempts are per checkout (`proofrun.ReadAttempts(repoRoot)`,
budget.go:513); charging X here never counts another seat's attempts. A6:
`goalrevision.Acquire(root, goalID, revision, tag)` (goalrevision/lock.go:
125) may be held for two goals by one process in a fixed order; the builder
confirms no same-process exclusion.

Seat ruling needed: 1. D3's consequence that release and re-claim no longer
resets a goal's attempt count (decided here; overturn or confirm). 2. Whether
`--authority` may also name an arc-mate claim (D8 lets resume skip arc
mates); the page refuses it until asked. 3. None else; every other question
is decided above.
