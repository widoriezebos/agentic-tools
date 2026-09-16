# Design: proof admission fits a seat proving several units (r3)

Goal: proof-admission-fits-a-seat-proving-several-units (tier 2, priority 1,
sequence 30). Revision 3 folds critique r2 (6 material findings, verdict
rework) and is the final design round under R-117-m1e item 2 ("When every
remaining material finding can be fixed with a local correction, the seat
writes the corrections, lands the page and builds. Units a finding blocks are
parked."). Code citations are origin/main af15703f1 read on 2026-09-16, paths
under metasystem/; `git diff --stat faf9d057c origin/main -- cmd internal`
touches none of the cited files except internal/refusal/register.go (one added
row), and `git diff --stat dacadefe0 origin/main` over the cited packages
touches only that file, so every cited line is identical to the critique's
faf9d057c and r2's dacadefe0 (PA-010-R2 refreshed); goal-cli-fixtures.sh
moved by 20 lines and is re-cited. Every rule names the witness that fails
without it (Wido 2026-09-14). Tests run on injected clocks and injected
barriers, never wall time or sleeps (Wido 2026-09-12). No unit lowers a floor,
removes a witness or gate, or narrows the DONE (R-115-m1e rule 6: "no unit of
the efficiency program lowers a proof floor, removes a witness or a gate, or
narrows a DONE to save tokens"). No unit touches hooks, deny mode or tmux
(R-118-m1e item 1: "Units that turn on deny mode or change hook settings
beyond observe mode still wait"; R-114-m1e item 2: "No machinery may assume
or rely on tmux"). Builds are approved in Wido's name under R-119-m1e item 1
once this page lands (R-119-m1e item 2 keeps R-117-m1e items 2 and 5 as
quality gates; item 5: "land.sh in your name only after an Opus read with no
NOT LAND verdict, a green run of every group the test plan selects, and a
receipt in the same commit"). R-116-m1e is not relied on here.

## 0. Findings table

| Finding | Disposition | Carried by |
|---|---|---|
| PA-001, PA-005 (live path), PA-006, PA-007, PA-008, PA-009 | FOLDED in r2, closed by critique r2 | unchanged |
| PA-010-R2 stale provenance (not material) | FOLDED | header |
| PA-011 arc-mate authority refusal absent | FOLDED | D2, R17, W17c, W18, U3b, U9 |
| PA-002-R2 `Approved.Revision` cannot key the episode | FOLDED | D3, R2, W5a-g, U2a |
| PA-003-R2 lens omits stores and the extension transition | FOLDED | D4, R3, R20, W5e, W4c, U2b, U3c |
| PA-004-R2 locks do not linearize candidate mutations | FOLDED (the critic's second option: versioned publish) | D5, R6, R19, W7a-e, U1b, U3d |
| PA-012 optional fields revert to authority identity | FOLDED | D1, R18, W19, U1a |
| PA-013 journal attribution is not holder authority | FOLDED | D7, R12, W14d, A4, U6 |
| OQ1, OQ2 and the seat authority question, critic's answers | FOLDED, all three adopted | section 6, R17 |
| NEW-FROM-FOLD: `WithdrawReservationLocked` | NEW-FROM-FOLD | D5, R19, W20, U1b |
| NEW-FROM-FOLD: schema-3 records and engines older than U1a | NEW-FROM-FOLD | section 5 |
| NEW-FROM-FOLD: `goalrevision.AcquireWait` seam for the busy witness | NEW-FROM-FOLD | D5, W7e, U3d |
| NEW-FROM-FOLD: two units split at witness boundaries (U1, U2, U3b) | NEW-FROM-FOLD | section 4 |

## 1. The problem, with evidence (verified at origin/main)

1.1 Proofs are charged to the claim, not the candidate's goal. A top-level
`test run` resolves its goal from `--goal` or the machine's single live claim
(proof_run.go:404-411; test.go:213-218, 1362-1382); `ResolveGoalBinding`
requires a claimed goal (proof_run.go:430); the plan is selected with that
goal's risk, read only from a claimed goal (test.go:310-315, 1338-1360); the
trusted engine gets the same `--goal` (test.go:431-434); the verdict runs on
that goal (proof_run.go:536-537) and the projection counts attempts whose
`GoalID` equals the file id and whose `AccountingRevision` is at or past the
claim's (budget.go:517-526, 571-577). A fresh-pair claim binds a new
`AccountingRevision` (verbs.go:323-325, 995) and starts a fresh count; a
same-pair re-claim restores the kept episode (`leaveEpisode` verbs.go:352-378,
`resumeEpisode` 410-423: `f.Claimed.AccountingRevision =
kept.AccountingRevision`); a person's approve or set-budget drops the kept
episode (approval.go:606-610, "A person's budget act starts the box afresh";
verbs.go:1229-1230). r2's blanket "a re-claim starts a fresh count" is
corrected to this.

1.2 Retry decisions cross candidates (`componentDecisionLocked`
attempt.go:650-682, "ambiguous across 2 failed attempts" 711-712; the tree
survives only as `IdentityInputs[0]`, test.go:814-815, and
`TestResult.CandidateTree`, test_result.go:181). Unchanged from r2.

1.3 The fenced goal's two verbs point at each other (set-budget refuses while
fenced, verbs.go:1206-1208; resume refuses a changed tuple, goal/stop.go:
437-438) and the set-budget tail restamps any positive `r.ClaimEpoch`
(verbs.go:1240-1249) from the command edge's `1` fallback
(goalsync_mutations.go:529-533); the attorney path carries `by == ""`
(1770) and `bindClaim` refuses it (verbs.go:328-330). Unchanged from r2.

## 2. Decisions

D1. Candidate-aware records are a new attempt schema. `CandidateAttemptSchema
Version = 3` joins `LegacyAttemptSchemaVersion = 1` and
`AttemptSchemaVersion = 2` (attempt.go:33-34). Fields: `CandidateGoalID`,
`CandidateRevision`, `CandidateBudgetEpoch` (`*uint64`, nullable by domain),
`CandidateTree` (json `candidateGoalId`, `candidateRevision`,
`candidateBudgetEpoch`, `candidateTree`). `validateAttempt` (395-405):
schema 3 requires the tuple as one fact, `CandidateGoalID != ""`,
`CandidateRevision >= 1`, `CandidateTree` a valid digest when `CommandClass
== "testing"` and empty otherwise; schemas 1 and 2 refuse any candidate
field. Accessors `AccountedGoal()`, `AccountedRevision()`,
`AccountedBudgetEpoch()` branch on `SchemaVersion == 3`, never on emptiness:
schema 3 reads the candidate fields only, schemas 1-2 read `(GoalID,
AccountingRevision, BudgetEpoch)`. `ReserveLocked` (532, stamp at 580)
writes schema 3 for every new record and refuses an `AdmissionRequest` whose
`CandidateGoalID` is empty or `CandidateRevision` zero; `AdmissionRequest`
gains the four fields and every caller passes candidate == authority until
U3b resolves them apart. The strict decoder (296) keeps refusing unknown
fields. Authority fields stay as today. `CandidateTreeDigest()` is D6.

D2. `--goal` names the candidate's goal; authority is resolved apart. As r2:
X supplies risk, required mode, the trusted engine's `--goal`, the
consumption episode and the retry and reuse history; authority is X when X
is claimed live here, else the machine's unique live claim
(`uniqueActiveProofGoal`, proof_run.go:590-632), else `--authority
<goal-id>` naming a live unfenced claim of this machine; governed and
delegate proofs keep candidate == authority (proof_run.go:346-408). New:
when candidate X and authority C differ and `X.Arc != "" && X.Arc == C.Arc`
(`GoalFile.Arc`, file.go:38; the relation resume already treats apart,
goal/stop.go:461-463), the launch is refused `PROOF_AUTHORITY_ARC_MATE_
REFUSED` naming X, C and the arc, whether C came from `--authority` or from
resolution; the remedy named is to claim X (arc joins exist,
verbs.go:2314-2315 `classifyArcJoin`). Seat ruling adopted (critique r2):
arc-mate authority is refused in this revision.

D3. The consumption episode is the human's budget act, recorded on the
approval. `ApprovalRecord` (file.go:183-191) gains `EpisodeRevision uint64`,
rendered `episode=%d` on the Approved line (file.go:1615) and parsed beside
`digest=`; zero when absent. `bindApproval` (approval.go:427-436) stamps
`EpisodeRevision: f.Revision`; its callers are the two human budget acts,
approve (approval.go:606-610) and set-budget (verbs.go:1229-1230). The risk
raise builds its own record (verbs.go:2669-2670, `Revision: f.Revision`,
`Authority: "raise=" + opid`, under verb `edit` at 2653) and copies
`prior.EpisodeRevision`, so validation's event binding (file.go:759-768,
`Revision` resolves to `History[Revision-1]`) and risk_test.go:112-126 are
untouched. `ExtendBudget` (verbs.go:1035-1108, verb `extend-budget`) writes
`Budget` limits and `BudgetExtension` and never `Approved`; claim binds the
approved budget without a tuple (verbs.go:985-992); release, park, resume
and re-claim never write `Approved`. Validation: `EpisodeRevision == 0 ||
(EpisodeRevision <= Revision && History[EpisodeRevision-1].Verb` is
`approve` or `set-budget)`, the index invariant of 763. `goal.BudgetEpisode
Revision(f) uint64`: `Approved.EpisodeRevision` when non-zero; legacy zero:
the smallest of `Approved.Revision`, `Claimed.AccountingRevision` when
claimed and `Episode.AccountingRevision` when a kept episode exists (a raise
moves `Approved.Revision` past the claim's accounting revision, so the
minimum never counts fewer than today); 0 when `Approved == nil || Budget ==
nil`. Transition table: approve, set-budget start a new episode; risk raise,
extend-budget, release, park, resume, same-pair re-claim, cross-pair re-claim
keep it. Consequence, stated: within one episode a release and re-claim no
longer resets a goal's attempt count; the remedy is `goal set-budget`
(approved in Wido's name under R-119-m1e). A raised floor (R-115-m1e rule 6).

D4. Two lenses over three stores. The projector sums job records
(budget.go:332-446: `goalId` 353-368, `recordRevision < accountingRevision`
412, `Attempts++` 446), governed runs (585-666: `GoalRevision <
accountingRevision` 600 and 630, epoch rules 644-649, `Attempts`,
`ReservedJobMinutes`, `OpenCapMinutes` 664-666), terminal governed attempts
(676-702: epoch 687-691, time rule 693, 700-702) and proof attempts
(513-583). Consumption lens (any budgeted goal X): one key,
`consumptionKey := BudgetEpisodeRevision(X)`, replaces `accountingRevision`
at 412, 600, 630, 676-694 and 571-577; identity is `goalId == X` for job and
governed records (candidate == authority there, D2) and `AccountedGoal() ==
X` for proof attempts; epoch rules stay (a non-nil epoch against a goal
without one is `BudgetUnknown`, 531-532, 648-649, 690-691); line 693's
`episodeAt` is the zero time when X is unclaimed. Authority lens (claimed
goal C): elapsed, active jobs and open cap over today's claim key
(480-561, 664-666) unchanged. One record may be a member of both lenses
under different keys. `ProjectConsumption(repoRoot, file, now)` answers for
an unclaimed goal (which `ProjectBudget` refuses, 266-268) and produces the
same members as `ProjectBudget`'s attempt and minute fields for the same
goal claimed (W5e). Gates split as r2: C keeps `elapsedLimit` and
`activeJobLimit`; X keeps `attemptLimit` and `reservedJobMinutesLimit`
(admission.go:369-394); each refusal names the goal and the limit. Earned
extension: admission.go:285 offers one only when candidate == authority;
`ExtendBudget` stays the holder pair's own act (verbs.go:1069-1076); a
candidate X != C whose breaches are consumption-only is refused
`CANDIDATE_EXTENSION_REFUSED` naming X, the limit and the two remedies
(claim X as authority, or `goal set-budget X` in Wido's name). No mutation
authority over an unclaimed goal's budget is introduced. Old records:
schemas 1-2 read `(GoalID, AccountingRevision)` and count what they count
today plus attempts from an earlier claim inside the episode (D3).

D5. Linearization by sorted locks and a versioned publish. A launch reads X
before locking (state, budget, fence, claim, `BudgetEpisodeRevision`, weight
epoch), then acquires `goalrevision.Acquire(root, id, revision,
"proof-admission")` (lock.go:125-146) for C and X in lexicographic goal-id
order (one lock when C == X, today's path at proof_run.go:453), then
`proofrun.AcquireMutation` (458). Snapshot 1: re-read C and X under the
locks; any difference from the pre-lock read refuses `CANDIDATE_GOAL_MOVED`
naming both revisions. `ReserveLocked` publishes. Snapshot 2: re-read X and
C; if the tuple (revision, state, fence stop id, claim pair,
`BudgetEpisodeRevision`, weight epoch) differs from snapshot 1,
`WithdrawReservationLocked(root, id)` (R19) removes the record under the
same mutation lock and the launch refuses `CANDIDATE_GOAL_MOVED`. Mutations
need not participate: set-budget (goalsync_mutations.go:1782), park (1005)
and approve (1775) take no goal-revision lock today while extend-budget
(1654), resume (1969) and split (2060) do; the versioned publish catches
both kinds, because a goal write is an atomic replace (A7) and lands either
before snapshot 2 (withdrawn) or after publication (the attempt keeps the
episode it read; a later set-budget starts a new episode without touching
it). Seams, package vars nil in production: `proofAdmissionUnderLocks`
(after the locks, before snapshot 1), `proofAdmissionBeforePublish`,
`proofAdmissionAfterPublish` (before snapshot 2), and
`proofAdmissionLockOrder func([]string)` recording the acquisition order.
Reciprocal charges (C=A, X=B and C=B, X=A) both acquire [A, B]; a lock held
elsewhere returns `BusyError` after the lock's own bound (lock.go:137,
`Wait: time.Second`) and the launch refuses naming both goals and the busy
path; `goalrevision.AcquireWait` (package var, default `time.Second`) lets
W7e run at zero wait, production unchanged.

D6. One candidate-tree accessor, validated. As r2: `CandidateTreeDigest()`
returns the schema-3 field; else `TestResult.CandidateTree`; else
`IdentityInputs[0]` for a testing record (test.go:813-815); else
`("", false)`. Validation refuses a field that disagrees with a derived
source. Retry filtering as r2 (drop other trees' failures, reuse successes,
keep live duplicates, treat an unidentifiable legacy failure as the
request's own tree).

D7. Epoch replacement needs an authenticated holder fact, on the live path
only. As r2: `VerbRequest.EpochAuthority`, set to `EpochAuthorityHolder` by
`syncReqClassifiedWithTerminalGrade` exactly when `classification.Holder &&
classification.ClaimEpoch != nil` (goalsync_mutations.go:529-533);
`goal.ClaimEpochForRebind(f, r)` (holder with `ClaimEpoch >= 1` returns it;
holder with `ClaimEpoch < 1` or a non-MAIN class is contradictory; no
holder authority returns `f.StopCapability.ClaimEpoch`, else
`REBIND_EPOCH_UNAUTHENTICATED`); the set-budget tail calls it; resume keeps
its epoch (goal/stop.go:452). Corrected: no intent argument carries epoch
authority. Recovery treats the journal as evidence, never a credential
(recover.go:43; a journaled human name is refused, 147-151) and refuses
set-budget replay outright (376-377, "APPROVAL_REQUIRED: set-budget is
proof-bearing and cannot be replayed from journal text"); that refusal
stands and gains its missing witness (W14d; recover_test.go has none
today). `claimIntentArgs` (verbs.go:492-500) is unchanged. Seat ruling
(m1e, 2026-09-16): U6 lands first; stop-capability-follows-the-lease-epoch
builds on the two names `VerbRequest.EpochAuthority` and
`goal.ClaimEpochForRebind`.

D8. One-step set-budget on a fenced claimed goal. As r2 (preconditions
goal/stop.go:449-467; the fixture pattern at goal-cli-fixtures.sh:1605-1624;
`HistoryLine.Resumed`, file.go:386-410; `SET_BUDGET_FENCED_SAME_TUPLE`).

## 3. Rules and witnesses

Each witness fails on origin/main or fails when the rule's code is removed;
the builder proves each by mutation and records the mutation in the return.
R1, R4, R5, R7-R11, R13-R16 stand as r2 with their witnesses W1, W6, W3,
W8-W13, W16, W15, W17, W18; W18's list is now eight codes (R16 below).

R2. The consumption episode starts only at a human budget act. W5
`TestCandidateEpisodeSurvivesClaim` (budget_test.go): two attempts charged
to unclaimed X; claim, release, re-claim; `ProjectConsumption(X)` reports
the same two at each step. W5a (goal tests, `TestBudgetEpisodeRevision
Transitions`, one row each): approve starts; risk raise keeps and
`Approved.Revision` still advances (risk_test.go:112-126 stays green);
extend-budget keeps; release and same-pair re-claim keep; cross-pair
re-claim keeps; set-budget starts. W5f: legacy record without `episode=`
reads the minimum of the three keys. W5g (file_test.go): `episode=` parses
and renders, an `episode=` naming a non-budget event is invalid. Fails on
origin/main (no field) and when the raise stops copying.

R3. One live charged attempt counts in the authority's active jobs and the
candidate's attempts and minutes; the gates split by lens. W4 and W4b as
r2. W5e `TestConsumptionLensSameForClaimedAndUnclaimed`: a job record, a
governed run and a proof attempt charged to X; `ProjectConsumption(X)`
attempts and minutes equal `ProjectBudget(X)` attempts and minutes with X
claimed, and are unchanged after release. Fails when any store keeps the
claim key.

R6. Admission is linearized by sorted locks and the versioned publish. W7
`TestCandidateGoalTransitionUnderLockIsBeforeOrAfter`: row a parks X at
`proofAdmissionUnderLocks` (refused before publication, no record); row b
re-budgets X at `proofAdmissionBeforePublish` (refused, no record); row c
re-budgets X at `proofAdmissionAfterPublish` (refused, the store holds no
record: withdrawn); row d re-budgets X after admission (admitted; the
record keeps its `candidateRevision`; `ProjectConsumption(X)` in the new
episode is zero). W7d: `proofAdmissionLockOrder` records [A, B] for both
reciprocal launches. W7e: with `AcquireWait = 0` and B's revision lock held
by the test, a launch charging B is refused naming A, B and the busy path.
No sleep; the seams and the held lock are the barriers.

R12. Only the authenticated holder replaces a recorded claim epoch. W14,
W14b, W14c as r2. W14d (recover_test.go): a journal entry for an
interrupted set-budget with `by=Wido` and a stored epoch is refused by
`RecoverWithPolicy` with the 376-377 text and the goal file unchanged.
Fails when the refusal is removed.

R16. New refusal codes have rows. W18 asserts row and site for
`CANDIDATE_GOAL_REFUSED`, `CANDIDATE_GOAL_MOVED`, `PROOF_AUTHORITY_REQUIRED`,
`PROOF_AUTHORITY_ARC_MATE_REFUSED`, `CANDIDATE_EXTENSION_REFUSED`,
`RETRY_PRIOR_OUTSIDE_TREE`, `SET_BUDGET_FENCED_SAME_TUPLE`,
`REBIND_EPOCH_UNAUTHENTICATED`; `TestHCL03EveryCodeRowed`
(register_test.go:20) passes.

R17. An arc mate may not be the authority for a candidate. W17c
`TestArcMateAuthorityIsRefused` (proof_run_test.go): X and C share arc `a`;
`--goal X --authority C` refused `PROOF_AUTHORITY_ARC_MATE_REFUSED` naming
X, C and `a`; `--goal X` with C the unique live claim refused the same; Y
outside the arc as authority admitted. Fails without the arc check.

R18. A schema-3 record carries its candidate tuple atomically; older
schemas carry none. W19 `TestAttemptSchemaThreeCarriesTheCandidateTuple
Atomically` beside `TestAttemptSchemaTwoAtomicallyRetainsTestingAndReads
SchemaOne`: a reserved record is schema 3 with candidate == authority; a
schema-3 record missing `candidateGoalId` or `candidateRevision` is
refused, not reattributed; a schema-3 testing record without
`candidateTree` is refused; a schema-2 record with any candidate field is
refused; accessors on a schema-2 record read the authority pair. Fails
when fallback keys on emptiness.

R19. A reservation with no observation can be withdrawn under the mutation
lock. W20 `TestWithdrawReservationRemovesAReservedOnlyRecord`: after
`ReserveLocked`, withdraw removes the file and `ReadAttempts` omits it; a
record with a process or a result refuses withdrawal. Fails without it.

R20. A candidate other than the authority earns no extension. W4c: X at
`attemptLimit` with advancement evidence that would earn a raise under its
own claim; `--goal X` under C refused `CANDIDATE_EXTENSION_REFUSED` naming
X, the limit and both remedies, `X.BudgetExtension` still nil; claim X and
the same launch is admitted through today's offer. Fails when the offer is
made for a candidate.

### 3.1 Reader inventory

As r2, with the stores of D4 added to the candidate side: job records
(budget.go:412, 446), governed runs (600, 630, 664-666) and terminal
governed attempts (676-702). U5b's census `grep -n "\.GoalID\b\|
\.AccountingRevision\b\|accountingRevision" cmd internal --include=*.go`
classifies every hit; an unclassified hit blocks its landing.

## 4. Units

Changed lines count additions plus deletions, tests included; at most 300
per unit. Each lands through R-117-m1e item 5's lane (quoted above).

| Unit | Files (tests) | Lines | DONE | Witness | After |
|---|---|---|---|---|---|
| U6 epoch authority | cmd/metasystem/goalsync_mutations.go:525-533, internal/goal/verbs.go:197-215, 1240-1249 (goalsync_mutations_test.go, goal tests, recover_test.go) | 200 | D7: `EpochAuthority`, `ClaimEpochForRebind`; no intent arg; recovery refusal witnessed; the two names published | W14, W14b, W14c, W14d | none; lands first |
| U1a attempt schema 3 | internal/proofrun/attempt.go (attempt_test.go) | 260 | D1 constant, fields, schema-keyed accessors, tuple validation; D6 `CandidateTreeDigest` and agreement | W8, W9, W19 | none |
| U1b reservation tuple and withdrawal | internal/proofrun/attempt.go, cmd/metasystem/proof_run.go (attempt_test.go, proof_run_test.go) | 140 | `AdmissionRequest` candidate fields, `ReserveLocked` writes schema 3 and refuses an empty tuple, callers pass candidate == authority; `WithdrawReservationLocked` | W19 (reserved row), W20 | U1a |
| U2a budget episode revision | internal/goal/file.go, approval.go:427-436, verbs.go:2669-2670 (file_test.go, approval_test.go, risk_test.go) | 220 | D3 field, render, parse, validation, stamp, raise copy, `BudgetEpisodeRevision` with legacy minimum | W5a, W5f, W5g | none |
| U2b consumption lens | internal/dispatch/budget.go, admission_test.go:289-331 (budget_test.go) | 260 | D4 `consumptionKey` in the four member sites, `ProjectConsumption`, `ProjectBudget` through the same predicate; OQ1 rewrite | W5, W5e | U1a, U2a |
| U3a candidate goal on the testing side | cmd/metasystem/test.go (proof_run_test.go) | 200 | `--authority` parsed; `testingGoalRisk` without the claim requirement; `proofLaunchAdmission` carries authority, tree, revision | W1 | U2b |
| U3b resolution, eligibility, arc | cmd/metasystem/proof_run.go (proof_run_test.go) | 240 | D2 resolution; R5 table; R15 refusals; R17 arc-mate refusal | W3, W6, W17, W17c | U3a, U1b |
| U3d linearization | cmd/metasystem/proof_run.go, internal/goalrevision/lock.go (proof_run_test.go) | 260 | D5 sorted order, snapshots, withdraw on a moved snapshot, four seams, `AcquireWait` | W7, W7d, W7e | U3b |
| U3c two-lens verdict | internal/dispatch/admission.go, cmd/metasystem/proof_run.go (admission_test.go, proof_run_test.go) | 260 | per-lens evaluation, gates filtered per D4, extension only for candidate == authority, `CANDIDATE_EXTENSION_REFUSED` | W4, W4b, W4c | U3d |
| U4 candidate-scoped retry | internal/proofrun/attempt.go (attempt_test.go) | 200 | D6 filtering; R10 in `readRetryDecision` | W10, W10b, W10c, W11 | U1a |
| U5a reuse and diagnostics readers | internal/proofrun/test_result.go, cmd/metasystem/test.go:1145, 1198 (tests beside each) | 180 | accessors replace raw pairs | W12, W12b | U1a |
| U5b round discovery, extension evidence, census | cmd/metasystem/prove_round.go, internal/dispatch/budget_extension.go (tests beside each) | 180 | `AccountedGoal()` at prove_round.go:214 and budget_extension.go:284; census classified | W13, W13b | U1a |
| U7 history grammar | internal/goal/file.go (file_test.go) | 140 | D8 `Resumed` field, parser, renderer, validation | W15 | none |
| U8 one-step set-budget on a fenced goal | internal/goal/verbs.go, stop.go; scripts/agents/goal-cli-fixtures.sh (goal tests) | 290 | `liftFenceForRebudget` shared with resume; R13 | W16, W16b, W16c, W16d | U6, U7 |
| U9 docs and refusal register | docs (test-run and goal-verbs pages), internal/refusal/register.go (register_test.go) | 150 | `--goal` as candidate, `--authority`, arc rule, two lenses, one-step set-budget documented; eight codes rowed | W18 | U3c, U8 |

Total allocation: 3,180 changed lines over fifteen units. Build order: U6,
U1a, U2a and U7 in parallel; U1b and U2b; U3a, U4, U5a, U5b in parallel;
U3b; U3d; U3c and U8; U9. U6 lands before U8 and before
stop-capability-follows-the-lease-epoch consumes it. Where a unit's build
exceeds 300 lines it splits at a witness boundary, recorded in the goal
history.

## 5. Rollout facts

The trusted base engine (`metasystem.steward.landing-ref`,
test.go:1384-1385) plans with `--goal X`; an engine landed before U3a
refuses an unclaimed X ("no accepted risk and accounting lineage"), so
`--goal X` for an unclaimed X works from the first landing after U3a. After
U1b every new record is schema 3, and `ReadAttempt`'s strict decoder
(attempt.go:296) in an engine older than U1a refuses `candidateGoalId`;
attempts are per checkout (A5), so the only hazard is a checkout whose
engine is rebuilt to a commit before U1a after it wrote schema 3; the
engine-rebuild rule applies: rebuild after every pull that includes U1a
and never build an older engine over a live store. The D3 tightening can
close a live goal whose attempts from an earlier claim now count; the
remedy is one set-budget in Wido's name.

## 6. Open questions answered

Q1-Q8 as r2. OQ1 adopted: the fresh-claim consumption expectation at
admission_test.go:289-331 is rewritten to assert continuity (U2b, the read
names R-115-m1e rule 6 as a raised floor); the same-pair continuity
witnesses at verbs_test.go:601-634 and budget_test.go:957-979 stay, and no
ownership or elapsed-history assertion is erased. OQ2 adopted with the
corrected owner: only a person's approve or set-budget starts a new
consumption episode; release, re-claim, risk raise and earned extension
keep it (D3). Seat authority question adopted: arc-mate authority is
refused (R17).

## 7. Assumptions

A1 resolved into fact (section 1.3). A2: a group's execution identity is
unchanged across trees when its inputs are unchanged (attempt.go:673-678);
D6 drops only failures. A3 withdrawn: D3 no longer asks the raise to keep
`Approved.Revision`. A4 corrected: recovery never replays set-budget
(recover.go:376-377); no journal argument proves authority. A5: attempts
are per checkout (`proofrun.ReadAttempts(repoRoot)`, budget.go:513). A6:
`goalrevision.Acquire` may be held for two goals by one process in sorted
order (lock.go:125-146 is per goal/revision path); the builder confirms no
same-process exclusion. A7: a goal write is an atomic replace (Publish
applies `Change{Path, Content}`, verbs.go:2679); the U3d builder confirms
the rename-based write before relying on snapshot 2. A8: history index
`Revision-1` names the event of that revision for every valid goal file
(file.go:759-763 enforces it); D3's validation reuses it.

Seat rulings (m1e, 2026-09-16, after checking each r2 finding folded against
critique r2 under R-117-m1e item 2): 1. D3 CONFIRMED. Only approve or
set-budget restarts a goal's count; release and re-claim never do. Reason: a
count that a release and re-claim can reset is a floor the holder can lower
alone, which R-115-m1e rule 6 forbids; a real reset stays a human budget act
(Wido's, or a seat under his recorded power of attorney). 2. D4 CONFIRMED.
Earned extension is refused for a candidate other than the authority
(`CANDIDATE_EXTENSION_REFUSED`), because the alternative gives a holder
mutation authority over an unclaimed goal's budget; the two named remedies
stay. 3. Nothing else is open; the page lands and U6 builds first after the
landing lane drains.
