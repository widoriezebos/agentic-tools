# Design brief: proof-admission-fits-a-seat-proving-several-units

## Revision

Revision: first draft (design r1), self-filled by the Fable design delegate on
seat m1e, 2026-09-15 night, under S5/fable-frontload-instructions.md.

Reason: tier-2 design goal at priority 1, sequence 30 of the p1 program map.
No seat pack exists for this goal; the delegate fills the pack itself from
origin/main (e70a832e2; cited files identical to the checkout at a921c63a1,
checked with `git diff --stat origin/main -- cmd internal/{proofrun,dispatch,goal}`).

## Context pack

Read this pack first. Open another file only to check one of the cited lines
below; do not widen the read beyond that check.

### The goal (origin/main, plans/goals/proof-admission-fits-a-seat-proving-several-units.md)

State approved, priority 1, sequence 30, tier 2, revision 38. Budget
elapsedLimit=4h attemptLimit=6 reservedJobMinutesLimit=720 activeJobLimit=1
reviewRoundLimit=2. Approved by Wido 2026-09-15T05:55:46Z.

Intent (verbatim): "A seat proving several units hits admission refusals
unrelated to its candidates. Proof attempts are charged to the one goal the
machine may claim, so on 2026-09-15 other units' proofs and controls spent
coordinator-context-stays-under-budget's attempt limit and blocked unit C1b; a
retry decision is refused as 'ambiguous across 2 failed attempts' when
components' latest failures come from different candidates, even for a single
section; and a breach-stopped goal refuses set-budget until resumed, then
refuses the run until a release and re-claim (m1c, about 40 minutes)."

DONE (verbatim): "a proof attempt is charged to the goal its candidate belongs
to without switching the machine's claim; a retry decision is scoped to the
candidate tree so another candidate's failures never make it ambiguous;
set-budget on a breach-stopped goal works in one step and the claim's
reservation follows; fixtures prove each."

Next step (verbatim): "Design, tier 2: decide how a proof attempt names its
goal independently of the machine claim, scope retry decisions to the
candidate tree, and fix the breach-stop set-budget ordering; coordinate the
reservation part with stop-capability-follows-the-lease-epoch."

Map row 30 (S5/p1-program-map.md): evidence "by intent: other units' proofs
spent a program goal's attempts and blocked unit C1b"; no design page; note
"coordinate the reservation part with stop-capability-follows-the-lease-epoch";
size M; "Tier-2 design: a proof attempt names its goal apart from the machine
claim; candidate-scoped retries; set-budget order."

Tonight's evidence (seat m1e relay in the delegate prompt): m1e bound the 8d
page and rulings proofs to seats-spend-tokens-in-bounded-sessions, then
released it and claimed coordinator-context-stays-under-budget to build units;
a seat proving work for several goals had to bind every test run to its one
claimed goal.

### Binding rulings (memory/rulings.md, quoted where relied on)

- R-117-m1e item 2 (round limit): "When every remaining material finding can
  be fixed with a local correction, the seat writes the corrections, lands the
  page and builds. Units a finding blocks are parked."
- R-117-m1e item 5 (landing lane): "land.sh in your name only after an Opus
  read with no NOT LAND verdict, a green run of every group the test plan
  selects, and a receipt in the same commit. No hook bypass, and nothing
  touching goal files by hand."
- R-118-m1e item 1: "Units that turn on deny mode or change hook settings
  beyond observe mode still wait." No unit here touches hooks or deny mode.
- R-119-m1e item 1: seats may approve in Wido's name citing the row; "An
  over-norm set-budget still carries the engine's strict token form in a
  rulings row for the goal and its current revision." Item 3: every priority-1
  efficiency goal implemented.
- R-114-m1e item 2: "No machinery may assume or rely on tmux." Item 6: "Never
  stop work because of a limit: a cap or budget triggers a handoff or a
  delegate that continues the work, never a stop".
- R-115-m1e rule 6: "no unit of the efficiency program lowers a proof floor,
  removes a witness or a gate, or narrows a DONE to save tokens, and each
  unit's read checks it."
- Wido 2026-09-12: injected clocks, never wall time; fix a flaky test, never
  retry; add tests, never lower floors.
- Wido 2026-09-14: every rule names the witness that fails without it.

### Cited code excerpts (origin/main e70a832e2; paths under metasystem/)

1. `cmd/metasystem/proof_run.go:409-416` (goal resolution for a top-level
   launch): when `request.GoalID == ""` and the caller is the main-class
   lease holder, `uniqueActiveProofGoal` picks the machine's single live
   claim; otherwise "top-level proof launch requires --goal".
2. `cmd/metasystem/proof_run.go:434-437`: `dispatchcore.ResolveGoalBinding(
   request.ControlRoot, request.GoalID, now)`; `internal/dispatch/stop.go:
   68-73` refuses "goal %s is not a claimed accepted goal" and, without a
   StopCapability, "predates breach-stop authority". So `--goal` can name
   only a claimed goal.
3. `cmd/metasystem/proof_run.go:495-501` (the holder gate): a main-class
   caller must be the lease holder with `*classifiedCaller.ClaimEpoch ==
   binding.Capability.ClaimEpoch` and `machine == binding.Machine`, else
   "active coordinator does not own the claimed goal reservation".
4. `cmd/metasystem/proof_run.go:521-528`: the `proofrun.AdmissionRequest` is
   built with `GoalID: request.GoalID, GoalRevision: binding.Revision,
   AccountingRevision: accountingRevision` (from `binding.File.Claimed.
   AccountingRevision`, lines 512-515).
5. `cmd/metasystem/proof_run.go:536-537`: `EvaluateGoalRevisionAdmissionFor
   Dispatch(root, request.GoalID, binding.Revision, cap, now, "implementer",
   "fresh", HazardMechanical)`; `internal/dispatch/admission.go:369-394`
   `budgetAdmissionBreaches`: attemptLimit at 384-386, reservedJobMinutes at
   387-389, activeJobLimit at 390-392, elapsed at 371-383.
6. `internal/dispatch/budget.go:513-520, 521-526, 571-577`: the projection
   reads every retained attempt, skips `attempt.GoalID != file.Id`, skips
   `attempt.AccountingRevision < accountingRevision`, then
   `projection.Attempts++` and charges reserved or observed minutes.
7. `internal/proofrun/attempt.go` `type Attempt struct` (fields GoalID,
   GoalRevision, AccountingRevision, ProofIdentity, Terminal, PendingTest
   Groups, TestResult, Load; no candidate-tree field) and `type
   AdmissionRequest struct` (GoalID, GoalRevision, AccountingRevision,
   RetryDecisionPath, ComponentIdentities, ExecuteAfresh).
8. `internal/proofrun/attempt.go:636-720` `componentDecisionLocked`: filters
   attempts by `GoalID` and `AccountingRevision` (650-652); keeps the newest
   observation per component id (653-682); collects `failedIDs` (688-697);
   with a retry decision, `len(failedIDs) != 1` refuses "component retry
   decision is ambiguous across %d failed attempts" (711-712).
9. `cmd/metasystem/test.go:814`: the testing identity's `IdentityInputs`
   start with `prepared.CandidateTree, prepared.ContractDigest, ...`;
   `test.go:152` `--goal` "accepted goal owning delivery"; `test.go:1362-
   1382` `resolveTestingGoal` returns the requested id unchanged, else the
   parent attempt's goal, else `uniqueActiveProofGoal`.
10. `internal/proofrun/attempt.go:826-850` `readRetryDecision`: the prior
    attempt must satisfy `priorAttempt.GoalID == goalID` and the same
    `IdentityDigest`.
11. `internal/goal/verbs.go:1206-1208` (set-budget mutation): `if f.StopFence
    != nil { return "goal %s revision %d is breach-stopped by %s; only goal
    resume with its standing approved budget may reopen admission" }`, before
    the tuple comparison at 1216.
12. `internal/goal/stop.go:433-438` (Resume): `if approvedBudget != r.Budget
    { "APPROVAL_REQUIRED: resume cannot change the human-approved budget;
    re-approve or use the proof-bearing goal set-budget before resuming" }`;
    `449` `VerifyStopBatchComplete`; `452-467` refuses when the machine
    "already holds live claim %s"; `477` `bindClaim(f, machine, lineage,
    r.stamp(), f.Revision, claimEpoch)` with `claimEpoch :=
    f.StopCapability.ClaimEpoch` (452).
13. `internal/goal/verbs.go:1240-1249` (set-budget tail): `claimEpoch :=
    r.ClaimEpoch; if claimEpoch < 1 && r.Actor.Human != "" && f.
    StopCapability != nil { claimEpoch = f.StopCapability.ClaimEpoch }`;
    then `rebindClaimKeepEpisode(f, r.stamp(), f.Revision, claimEpoch)`;
    `verbs.go:327-338` `bindClaim` stamps `StopCapability{Generation:
    revision, Revision: revision, Machine, ClaimEpoch}` and clears the
    fence; `verbs.go:323-325` `newClaimRecord` sets `AccountingRevision:
    revision`.
14. `cmd/metasystem/goalsync_mutations.go:529-533` (syncReqClassified):
    `if classification.ClaimEpoch != nil && (Holder || by != "" || Class ==
    ClassHuman) { req.ClaimEpoch = *classification.ClaimEpoch } else if
    Class == ClassHuman { req.ClaimEpoch = 1 }`.
15. `cmd/metasystem/goalsync_mutations.go:1770` (attorney path): `req, err
    := syncReq(name, f.root, "", f.lineage)`; `1626-1631` set-budget needs
    `--id` and `--by`; `1961-1968` resume refuses "is not breach-stopped"
    when `binding.Fence == nil`.
16. `cmd/metasystem/proof_run.go:590-632` `uniqueActiveProofGoal`: multiple
    claims on the machine refuse "pass --goal"; a single fenced claim
    refuses "the only claim here is breach-stopped: %s (stop %s); pass
    --goal".
17. `internal/goal/verbs.go:480-490` `clearClaimBinding` (release) refuses
    while fenced: "only goal resume may clear its launch fence".
18. `internal/refusal/register.go:80` registers refusal codes with owner and
    site (example `ADMISSION_CLOSED_ELAPSED`, `budget.go:34`).

### Existing tests and beds (harness facts)

- `cmd/metasystem/proof_run_test.go:343-378` `TestProofAdmissionExtends
  RejudgesAndReserves`: goal fixture via `proofExtensionGoalFixture`, lease
  via `lease.AnnounceWithPair(root, ..., "m1")`, clock via
  `t.Setenv("METASYSTEM_GOAL_NOW", ...)`, then `admitProofLaunch(
  proofLaunchAdmission{... GoalID: "standing-validation", CapMin: "1",
  ScopeClass: "full", CommandClass: "testing"})`.
- `internal/proofrun/attempt_test.go:362-417` `TestComponentRepeatDecision
  SpansPlanChanges`: `proofAttemptFixture`, `BindIdentityInputs(base,
  []string{"group:a:<64>", "plan:one"})`, `ReserveLocked`, `component
  AttemptResult`, `FinalizeAttemptWithTestResultLocked`.
- `scripts/agents/goal-cli-fixtures.sh:1585-1604`: `job goal-admission
  --stop-lineage` at the breach boundary (rc 10, `BUDGET_REFUSED`), then
  `goal set-budget ... --by Wido --fixture-human-authority` and a grep of
  the rendered record; `:973-975` `job breach-stop` then release refused.
- `cmd/metasystem/dispatch_breach_stop_test.go`, `internal/proofrun/
  stop_test.go`, `test_stop_test.go` exist for the stop path.
- Every test uses `METASYSTEM_GOAL_NOW` or an injected `now`; no wall time.

### Neighbouring goals that must not overlap

- stop-capability-follows-the-lease-epoch (priority 1, seq 36, tier 3):
  "re-arming or reconciling restamps the claimed goal's stop capability from
  the live lease under the same holder, the divergence is reported by health
  with its remedy, and a test reproduces the 5-versus-1 case". This design
  keeps the stamping rule at set-budget and resume (excerpts 13 and 14) and
  leaves the repair of already-diverged records and the health role to it.
- seats-spend-tokens-in-bounded-sessions and coordinator-context-stays-
  under-budget: the goals whose attempts tonight's evidence concerns; no
  code overlap.
- fleet-doctor-repairs-what-stops-other-seats: may later repair a fenced or
  diverged claim; this design changes verbs, not a doctor.
- human-goal-verbs-forgiving: touches verb usability; this design changes
  set-budget's order on a fenced goal only.

## Tool-call budget

Maximum delegate tool calls: 45 (frontload instructions). Reads through
bounded views only; no tracked-file edits, no tests, no goal verbs, no
process starts.

## Page-size ceiling

Maximum page size: 450 lines (about 4,500 words). Units at most 300 changed
lines each.

## Fresh session

This draft runs in a new delegate session with no prior page.

## Page artifact and return shape

Write the page to: S5/proof-admission-fits-a-seat-proving-several-units-design-r1.md

Return at most 6 lines: design path, lines and words, tool calls, units and
total allocation, open questions one line each.
