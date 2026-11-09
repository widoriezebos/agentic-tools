# The lane finishes what it starts

- Kind: design
- Id: the-lane-finishes-what-it-starts
- Status: accepted
- Goals: the-lane-finishes-what-it-starts
- Critique: round 1 (Astra, designs/astra-finishes-r1.log; 7 material) folded in revision 2; round 2 (designs/astra-finishes-r2.log; 5 material) folded in revision 3; closed at round 2 on 0 material findings (a1 for Wido)

Accepted 2026-11-08 by a1 in Wido's name (decision 228: priority one; the machine may be bypassed to land it).

Priority one, Wido's word (2026-11-08 10:1x): these gaps hinder everything built with the machine; the machine may be bypassed to land it (R-145-m1e hot-fix exception, with an Astra read). The Landing contract below says what must hold before that push.

Rulings relied on: R-104-m1e (tests on injected clocks and probers), R-142-m1e and R-143-m1e (a person's act is never blocked; D3 keeps the person path), R-145-m1e (the hand route and its Astra review), R-150-m1e (an executable batch has full green evidence on its final tree before push).

Code read at a434ec4d30 in `/Users/wido/LocalStorage/GitHub/agentic-tools-m1e/metasystem`; origin/main is ahead only by records commits. Lane records: `/Users/wido/LocalStorage/GitHub/agentic-tools-landing/metasystem/artifacts/agents/landing`. Record times are UTC (Z); finding times are local (UTC+1).

Revision 2 folds Astra round 1 (seven material findings). What changed is in the Critique record.

## Problem

The night of 2026-11-07/08 the lane started work it did not finish. A person finished it each time.

The night's numbers:

- Three person proves of a merged batch: attempts 20261107T230215, 20261108T012806 and 20261108T034809 (results.jsonl). Each followed a skipped gate (gates.jsonl 20261107T225512, 20261108T010518, 20261108T032949; each `skipped`, `fast-static-build: green`, reason "change selects cmd/metasystem: full").
- Ten repeated scoped proofs of two unchanged main trees: 5b5fd3fc four times (07:23Z to 07:57Z starts), 92058229 six times (08:13Z to 08:49Z starts). All green, all with no `fullAt`.
- One static red kept open: red:fast-static-build-7 was recorded at 07:22Z (commit 35f90f2994) and closed by a person at 08:51Z (9e377c47aa). In between, ten scoped trunk greens on main each ran `fast-static-build` green. The steward's tally says "1 h through five green runs"; the records say 1 h 29 m and ten.
- 35 minutes of a red main while a notes-only read waited for a person (finding 563).
- One worktree a unit run could not stop after a rebase (finding 566).
- Two land-anyways and three withdrawn questions (steward's tally; hypothesis, I did not count them in the records).

Six code roots, one decision each, plus the gaps Astra found in my first answers. The root-cause report (`designs/lane-gaps-root-causes-2026-11-08.md`) has eight items. Item 3 (fix goal refused "main has an open incident") has no code root: every hold path honours `fixGoal` (hold.go:151-174) and the refusal was on a co-member's hand-in. Item 7 (return after a green replay) is owned by a-return-names-the-failing-test and a-final-prefix-red-is-repeated-before-own. This page does neither.

**1. A skipped gate names no next step (finding 565).** `landing prove --gate --wait` on Skipped returns only "<words>: <reason>" (cmd/metasystem/intent_landing_prove.go:407-409). The gate's Green branch says "the batch still needs landing prove before landing push" (:427-428) but also names no `next`. The skill says "Green or skipped: merge the next recorded member, or run `landing prove` after the last member and end your turn" (skills/landing-agent/SKILL.md:96). Twice the agent ended its turn without the prove.

**2. Recovery and the clock take the slot from a batch that has a next step.** A skipped gate always carries a green static group: `prepareGate` returns Red before any skip when the static group fails (gate_impact.go:40-50) and sets Skipped only after it (:81-83). Two owners then compete with the batch.

- The keeper calls `Observe` before the clock (internal/landing/lane/agent.go:229, then `Clock` at :250). `Observe` runs `RecoverMovedMain` on every tick (cmd/metasystem/landing_agent.go:347; internal/landing/plain/trunk_recovery.go:82-107). Recovery reads only main's proofs (`applicableMainProof`, :91). It does not read the batch. So a records-moved main takes the proof slot whenever the batch is between steps: after its gate, and after its green candidate before the push. At 03:37:56Z recovery started a scoped proof 92 seconds after the batch gate settled (03:36:25Z). The agent's `landing prove` then met `*Busy`.
- When the clock is due (keeper_clock.go:91-94 returns early otherwise), it gives the batch one wake only when `gate.Result == Green` (:167). A skipped gate falls through to `Start` of main (:176-192).

Why nothing woke the agent at 03:46Z when that proof ended is not in the records I read (hypothesis: a hold won over `proof-finished`, wake.go:29-31).

**3. The agent may not re-prove a tree a person has since excepted.** `checkBound` refuses any non-green result on the tree unless `Repeat == "allowed"` or a person runs it (internal/landing/plain/prove.go:466-471). Tree 44982db5 had a full red at 00:02:37Z. The fix goal was re-handed with a person's exception at 01:03:55Z (queue.jsonl:267, `person.checked-at` 01:03:51Z). The agent's prove after the 01:11Z gate was refused. A person's prove at 01:28Z passed and was green at 02:17Z.

**4. Recovery re-proves a tree it has already examined (item 8).** `RecoverMovedMain` starts a proof unless `applicableMainProof` finds reusable coverage (trunk_recovery.go:91-93). A scoped green counts only with a current full base (`inherited && !currentFull` skips it, :154-157). After a red full there is no full base, so every scoped green is skipped and every keeper tick proves the same tree again. `Start` rechecks the same reuse rule under the lane lock (prove.go:593-596), so a check placed only before `Start` races a proof that finishes in between (the fixture shape of TestTrunkRecoveryRechecksAfterProofFinalizes, changed_main_test.go:924). The batching page assumed the opposite: "`RecoverMovedMain` stays as is" (the-lane-batches-goals-and-proves-once.md:164).

**5. A scoped green of the failing group never closes its red (finding 567), and a green that does close one is not checked against the current main.** `clearLandingIncidents` returns at once unless the proof is a full green on its own tree (cmd/metasystem/intent_landing_incident.go:84). The scoped proofs record their groups in `Ran` (results.jsonl 20261108T072337 onward: `fast-static-build` among five groups) and say "the rest is unproved" because the base full was red (internal/landing/plain/standing.go:276-281). The red's register entry names the group. Separately, the clear reads the register at `proof.Commit` (intent_landing_incident.go:91-96). The ledger writer rebuilds on each tip (internal/goal/trunkred.go:843-866) and compares the entry (:888-896). It never asks whether executable code moved between the proved commit and that tip.

**6. The incident clear can fail after the green is recorded, and nothing retries it.** The detached run records its result first. Then the command calls `clearLandingIncidents` and reports a failure as "main passed its full check, but its incidents could not be cleared" (intent_landing_prove.go:411-415). The only other callers are a classified red's next full (:269) and a push (intent_landing_push.go:214-217). An examined tree that recovery skips (D4) would leave that red open for good. Greens already recorded before this lands have the same gap.

**7. A row with no act asks a person, and the status calls it a wait.** The cannot-heal case sets `tried = "nothing: the row has no act of its own"` and asks anyway (internal/landing/plain/steward.go:369-376, `stewardAsk` :463-483, fact "lane steward: ..."). The status lists every open lane question of this machine (internal/landing/plain/status.go:698-719). `describe.go` words each as "Waiting for a person's answer to question %s" (internal/landing/state/describe.go:85, :243-261) and makes the first one the status summary (:286-290). The keeper does not hold on these (`questionHold` skips "lane steward: " facts, cmd/metasystem/landing_agent.go:229-231), but the skill says the keeper holds you "until it is answered" (SKILL.md:221-223) and the summary says waiting. a-lane-row-carries-its-act (approved) gives these rows a real act; it does not cover the row that has none.

**8. A notes-only read waits for a person on both review routes (finding 563).** Named work: `closeWorkReview` writes a DECIDE template and returns in progress whenever `len(findings) > 0` (cmd/metasystem/intent_review_binding.go:186-206). Only a read with zero findings closes alone (:207-211). The binding check (goal, work, attempt, subject, examination, round, return digest) lives only in the explicit-file branch (:212-231). Explicit commit: the route calls `cleanExaminationJoin` (cmd/metasystem/intent_unit_review.go:1257), which is clean only with zero findings (:1474) and returns an existing decisions file without reading its binding (:1478-1480). `workOfCommit` matches a work only when the commit is its newest subject exactly (:1415), so after a rebase the new commit takes this route. `noted` is already lawful for a non-material finding (internal/validate/critiqueclosed.go:43 refuses it only for material ones; :274 lists it).

**9. A rebase leaves the worktree with an owner that stop cannot pass (finding 566).** `ReserveMutation` sets `owner.Run` to the rebase's own id (internal/launch/tree_reservation.go:512-513). Its release closure only marks that record completed and leaves `owner.Run` pointing at it (:518-526). `GateTree` lets another run in once that owner is quiescent (:130-137). `CancelRun` refuses outright when another run owns the worktree: "another run owns the worktree; this run cannot release it" (:278-280). The recorded unit 20261108t052213-e7235eedc6 (`~/.metasystem/unit/.../run.json`) has a published subject and a round-1 stop decision `close`. That satisfies the closure predicate in `treeQuiescent` (:239-240). So the unit was logically closed; the stop was refused by the distinct-owner check, not by an unclosed unit. The reservation now names a later run (U2, 20261108t084349-16db0c288a), so `GateTree` did release it when the next unit entered. Publication uses the same reservation ("read-publication", intent_unit_review.go:995, :1015).

## Decisions

**D1. A skipped or green member gate names `landing prove` next.** In intent_landing_prove.go:407-409 the Skipped result's summary becomes "<words> skipped the cheap gate (<reason>); the batch still needs landing prove before landing push", with `next: landing prove` and reason "after the last recorded member is merged". The Green gate summary at :427-428 gets the same `next`. SKILL.md:96 stays; SKILL.md:99 gains "a `main` cause in `last_proof` older than the gate is not this batch's cause".
Rejected: a new keeper wake reason "batch merged, gate settled, no proof running". It adds a state for the keeper to own; the agent already holds the turn and only lacks the step.

**D2. The current batch's next step comes before recovery and before the clock's main check.**
(a) One predicate, `acceptableGate`: Green, or Skipped with `Static == Green` (prove.go:91). keeper_clock.go:167 uses it; the wake reason stays "the batch's gate passed, its full check goes next".
(b) `batchNextStep(install, checkout, seams)` reads recorded progress only. It is true when batch.json holds an open batch (`ReadBatch`, batch.go:238; state prepared or running) and one of these holds. Merge: a member's SHA is not in the lane checkout's HEAD. Prove: the last gate (`LastGate`, gate.go:30) is this batch's, it is acceptable, and the batch has no candidate result. Push: the batch's newest candidate result is Green and no push record names its tree (`LastPush`, push.go:423).
(c) `RecoverMovedMain` (trunk_recovery.go:82) returns nil, starting nothing, when `batchNextStep` is true and the new seam `BatchMayContinue` reports true. Observe (landing_agent.go:329-347) builds that seam from the keeper's existing fallback, nothing new: the barren stop is not reached (`lane.ReadAgentState`, agent.go:59; limit 2, agent.go:45) and none of the question, provider or drain holds stands (the funcs already passed as `Holds` and `ProviderHold`, landing_agent.go:362-399). When the agent cannot or will not run, recovery runs as today. A nil seam means recovery runs as today.
(d) No timer. The r1 sixty-minute expiry is dropped: time passing is not evidence the batch was abandoned.
(e) The clock keeps its one wake per state keyed by attempt (`state.Gate`, `state.Push`, keeper_clock.go:158-171) and its fallback to main's check (TestKeeperClockCandidatePushOpportunity, push refused).
The batching page's U4b adds its gathering and readiness states to the same predicate. Its D6 "recovery stays as is" clause is amended (see Amendment).
Rejected: wait for U4b. It is gated on Wido's gathering defaults and is last in that page's order; the stall happens every night until then. Rejected: move recovery after the clock in the keeper's order (lane/agent.go:229-262). That changes every keeper's step order and still gives recovery the slot between the green and the push.

**D3. A re-handed tree with a person's later exception is proved once by the agent.** In `checkBound` (prove.go:466-468) a non-green, non-allowed result does not refuse when a current batch member's queue entry carries an `Exception` (queue.go:60) whose `Person.CheckedAt` is after the result's `At`. The admission is used once: the next result on that tree is newer than the exception, so the refusal applies again. The person path (`s.Person != nil`, :469-471) is unchanged.
Rejected: let the agent repeat any red once more. That reopens the repeat loop the refusal exists to stop; the person's fresh act is the new fact.

**D4. Recovery does not re-prove an examined tree, and it decides that under the admission lock.**
(a) `examinedMainTree(install, commit, tree)`: the newest ran result for main's exact commit and tree is a trunk Green or Red, executed (not `didNotRun`, not inherited), not `ClassificationPending`, in the newest recorded environment, whatever its scope. A Red with a `main` cause and no incident is not examined, so its incident still gets recorded (as applicableMainProof :133-140 does today). A changed environment re-proves.
(b) It runs only in `Start`'s trunk branch under the lane lock, right after `applicableMainProof` misses (prove.go:593-596), and only when the new seam flag `Recovery` is set. `RecoverMovedMain` sets it next to `Trunk` (trunk_recovery.go:86). A hit returns `*Examined`; `RecoverMovedMain` treats it as nil, as it does `*Busy` (:101-105). There is no second check outside the lock.
(c) Examined is not reuse. It never sets `Reused`, never feeds `Settled` (prove.go:753) and never permits a push. `applicableMainProof` is unchanged.
Rejected: count a scoped green as coverage in `applicableMainProof`. That function also feeds `Settled` and push reuse, where coverage matters.

**D5. A trunk green closes a red only for groups it ran, and only while it still applies to main.**
(a) `clearLandingIncidents` (intent_landing_incident.go:84) also accepts an executed scoped trunk green: `Trunk`, `Green`, `Scope == "scoped"`, ran on `proof.Tree`, not inherited. For it, only open trunk-red entries whose `Group` is in `proof.Ran` are cleared. A full green keeps clearing every open trunk-red entry.
(b) Trunk-red entries only. The class filter at :114 stays. Flake, pending-flake and known-flake entries keep their fix-plus-three-executed-passes rule (`countFlakePass`, trunkred.go:911-934).
(c) Applicability at the publication boundary, for full and scoped greens alike. `TrunkRedClearArgs` (trunkred.go:766-777) gains `Applicable func(tip string) error` (not serialized). The batch writer's `Mutate` (:843-866) calls it with every rebuilt tip before it applies a clear. The lane passes: the tip is `proof.Commit`, or `proof.Commit` is an ancestor of the tip and every path changed between them is a record (`plain.ProofStillApplies`, a wrapper of `contains` and `codeMoved`, trunk_recovery.go:71-78). Otherwise the writer returns `TRUNK_RED_PROOF_STALE`, and `clearLandingIncidents` reports "the green no longer applies to main; the incident stays open" and returns nil. Hypothesis, to confirm at brief time: the endpoint root holds the tip's objects after `Capture`, so the diff can run there.
(d) The expected-entry check stays (trunkred.go:888-896). A sighting recorded during the proof changes the entry and refuses the clear.
Rejected: wait for a full green (today). Records-only moves keep producing scoped proofs, so the full never comes until someone lands. Rejected: accept movement that leaves the group's declared inputs untouched (`testpolicy.Affected`). It is wider and needs the closure read; records-only is the fail-closed subset, and executable movement leaves the incident to the next proof.

**D6. A row with no act is a notice, never a question.** In steward.go:369-376, when `tried` stays "nothing: the row has no act of its own" and the occurrence's disposition is not a person's `ask`, the steward records one intervention with `How: "notice"`, `Outcome: "noticed"` and skips `stewardAsk`. A later step finds that intervention for the same occurrence and is bounded "noticed once for this occurrence". A later explicit `ask` disposition on the same occurrence asks as today. In the status, a question whose second fact starts "lane steward: " goes to the steward lines (`d.Steward`), worded "the lane steward asked a person: question X ...; the landing agent does not wait on it", and never becomes `d.Questions` or the summary (describe.go:243-290; `QuestionInput` gains `Steward`, set in status.go:718). SKILL.md:221-223 becomes "the keeper holds you only on your own lane question; a lane steward question is not yours to wait on". The agent's own lane question keeps its hold (`questionHold`, landing_agent.go:220-251).
Overlap: a-lane-row-carries-its-act keeps "a row with a real decision names its act". This page takes only the no-act case.
Rejected: render every "lane steward:" question as a notice and keep asking. A real cannot-heal decision would then lose its question; a notice-only row would still notify.

**D7. A read whose findings are all non-material publishes itself, on both routes, through the binding check.**
(a) One preparation, `notesOnlyDecisions(binding, findings, generated)`. It applies only when there are findings and none is material. When decisions.md is missing, or is byte-equal to the untouched template `decisionsDocument(binding, findings)` (intent_review_binding.go:79-95), it writes the document: the binding line and one `noted` row per finding, reason "not material per the read". When decisions.md exists and differs (a person's partial work, or another review's binding), it is never overwritten; it is used as the decisions file and checked like one.
(b) The binding check at :218-231 moves into a helper that both routes call. A mismatch refuses as today and names the template.
(c) Named work: in `closeWorkReview` the preparation runs where :187-205 stops today, then the existing path: binding helper, `validate.Dispositions` (:232-236), register advance (:270-274) and the close owner (:283-289).
(d) Explicit commit: where `cleanExaminationJoin` is not clean (intent_unit_review.go:1257-1276), the same preparation runs with `commitExamination`'s binding (:1424-1463), then the binding helper, then the close owner `closeChain`, which validates the join and advances the register (intent_delivery.go:1675-1691). `cleanExaminationJoin` keeps the zero-findings case and gains the binding helper on an existing file (:1478-1480).
(e) The read keeps its identity. The binding names the original examination, round and return digest. A generated disposition is not a new examination, and an old read is never relabelled as a read of a rebased commit. One material finding keeps today's stop.
Rejected: a seat skill step "decide, then publish with --dispositions". It keeps a person-shaped act on a read that has nothing to decide.

**D8. A finished rebase gives the worktree back, and stop passes an owner that is done.** (a) The `ReserveMutation` release closure (tree_reservation.go:518-526) removes the reservation and resets `owner` when `owner.Run == id` and `owner.Children` is empty, as `GateTree` does (:134-136). (b) `CancelRun` (:278-280) does not refuse when `owner.Run != id` and `treeQuiescent(*owner)` reports released. It clears that reservation first, then cancels as today. A live, unknown or child-holding owner still refuses.
Rejected: re-bind custody to the unit run at rebase. A goal-level rebase has no unit run identity to hand back to.

**D9. A recorded green's incident effects are reconciled without a new run.** Execution done and its incident effects done are two facts. In `Observe`, after `RecoverMovedMain`, `plain.MainGreenToReconcile(install, checkout, seams)` returns the newest executed trunk green whose commit is current origin/main (D4's rule, Green only). Observe calls `clearLandingIncidents` with it. That owner is idempotent: closed entries are skipped (intent_landing_incident.go:114; trunkred.go:849-853) and a repeat is `AlreadyApplied`. D5's applicability guard applies. No test reruns. The owners value Observe builds (landing_agent.go:340) gets `machine` and `now`, which `incidentRequest` needs (intent_landing_incident.go:16-31). This also closes eligible greens recorded before this lands. An examined tree or a closed static incident is never coverage for groups that did not run.
Rejected: let recovery re-prove when a clear failed. That spends a suite on a fact already recorded.

## Units

Counted by reading each owner and call site at a434ec4d30; production lines only. Estimates run double (memory: unit estimates run double), so the doubled figure is the bound. Wido's ceiling is 250 per unit. r1's U1 grew to 154 counted with the folds (308 doubled), so it is cut in two along the decision line.

**U1a. The batch finishes (D1, D2).**

| File | Change | Counted |
| --- | --- | --- |
| cmd/metasystem/intent_landing_prove.go:407-409, :427-428 | Skipped summary and `next`; Green gate `next` | 6 |
| internal/landing/plain/keeper_clock.go:167 | `acceptableGate` | 2 |
| internal/landing/plain/trunk_recovery.go (new func) | `batchNextStep`: open batch, merge, prove, push | 30 |
| internal/landing/plain/trunk_recovery.go:82-96 | yield to the batch | 4 |
| internal/landing/plain/prove.go:236-262 | `BatchMayContinue` seam | 2 |
| cmd/metasystem/landing_agent.go:329-347, :363-396 | seam from barren state and the existing holds | 10 |
| skills/landing-agent/SKILL.md:99 | one sentence | not counted |

Counted 54; doubled 108.

**U1b. Main is proved once and its red closes (D3, D4, D5, D9).**

| File | Change | Counted |
| --- | --- | --- |
| internal/landing/plain/prove.go:466-468 | D3 later person exception | 14 |
| internal/landing/plain/prove.go:588-597, seam struct | D4 `Recovery` flag, examined recheck under the lock, `*Examined` | 14 |
| internal/landing/plain/trunk_recovery.go | `examinedMainTree`, `*Examined` as nil, `ProofStillApplies`, `MainGreenToReconcile` | 34 |
| cmd/metasystem/intent_landing_incident.go:83-131 | scoped acceptance, group filter, `Applicable`, stale result | 20 |
| internal/goal/trunkred.go:766-777, :843-866 | `Applicable` field and its call per tip | 8 |
| cmd/metasystem/landing_agent.go:340-347 | reconcile call; owners get `machine`, `now` | 10 |

Counted 100; doubled 200. Under 125 counted. Seam if the build runs long: D3 (14) goes to its own unit.

**U2. Lane steward notices (D6).**

| File | Change | Counted |
| --- | --- | --- |
| internal/landing/plain/steward.go:288-376 | notice branch, notice bound, ask disposition kept | 14 |
| internal/landing/plain/status.go:718 | set `Steward` | 2 |
| internal/landing/state/describe.go:37-40, :243-261 | field and routing | 9 |
| skills/landing-agent/SKILL.md:221-223 | one sentence | not counted |

Counted 25; doubled 50.

**U3. Seat routes (D7, D8).**

| File | Change | Counted |
| --- | --- | --- |
| cmd/metasystem/intent_review_binding.go (new func) | `notesOnlyDecisions` | 24 |
| cmd/metasystem/intent_review_binding.go:186-231 | binding helper; notes-only into the checked path | 16 |
| cmd/metasystem/intent_unit_review.go:1257-1276, :1466-1484 | commit route uses the preparation and the binding helper | 16 |
| internal/launch/tree_reservation.go:278-280 | stop passes a quiescent distinct owner | 8 |
| internal/launch/tree_reservation.go:518-526 | release clears the reservation | 6 |

Counted 70; doubled 140. Under 125 counted. Seam if needed: D8 (14) apart from D7 (56).

Total counted 249. Each unit is usable alone: U1a finishes a batch with no person's prove; U1b stops repeat proofs and closes a static red without a person; U2 removes the false wait; U3 unblocks the seat. Order: U1a, U1b, U3, U2.

## Tests

All tests keep the production owners and stub only effects: Git, process launch and liveness, and the clock (R-104-m1e). Plain tests run through the existing beds (`changedMainBed`, `trunkAdmissionBed`, `newStandingBed`, `newStewardBed`, the keeper clock bed) with an injected `Now`. Command tests run through the intent invocation or `newLandingAgentKeeper` (landing_keeper_clock_test.go beds). Ledger tests use the real writer: `clearMain` nil and `mainEndpoint` on the local ledger bed (landing_incident_test.go:25). Each mutation fails the named test.

- D1 `TestASkippedGateNamesTheBatchProveNext` (cmd, the bed of landing_gate_reuse_test.go:434-439 that yields Skipped with green static). Summary words and `Next.Argv == landing prove` for Skipped and Green gates. Mutation: drop `next` on Skipped.
- D2 `TestASkippedGateWithGreenStaticGetsItsWake` (plain, keeper clock bed). Skipped with green static: one wake, no `Start`. Skipped with static not green, and Red: fall through as today. Mutation: revert :167 to `Green` only.
- D2 `TestTheBatchStepsBeforeRecovery` (cmd, `newLandingAgentKeeper`). Replays the recorded order: skipped gate, a records-only move of main, the batch's proof finished, candidate green, push. Ticks between each: zero recovery launches. After the push: one recovery check of the new main if it is unproved. Rows with one recovery launch: barren stop reached; a question hold standing. Mutation: drop the push step from `batchNextStep` (a launch between green and push).
- D2 keep `TestKeeperClockCandidatePushOpportunity` (keeper_clock_test.go:105) and `TestKeeperClockSelectionFallbackOnce` (:274) unchanged.
- D3 `TestALaterPersonExceptionAdmitsOneAgentProve` (plain, repeat_test.go bed). Red on tree T at t0; member exception at t0+1h: agent `Start` admitted; after that attempt's red, a second agent `Start`: `NoRepeat`. Exception at t0-1h: `NoRepeat`. Mutation: drop the `CheckedAt` after `At` comparison.
- D4 `TestRecoveryDoesNotReproveAnExaminedTree` (plain, `changedMainBed`). Scoped trunk green on main's exact tree with no `fullAt`: zero launches over ticks at +1 min, +1 h, +3 h. One launch each: changed environment; `ClassificationPending`; main-cause red with no incident. `Settled` stays false for the examined tree. Mutation: drop the examined check.
- D4 `TestTrunkRecoveryRechecksAnExaminedProofUnderTheLock` (plain, the shape of changed_main_test.go:924). Recovery pauses at `TimerHeld`; a scoped proof with no full base finalizes; recovery resumes: launches stay one, `RecoveryStarted` is not called with a new attempt, `Reused` is nil. Mutation: move the examined check before `Start`.
- D5 `TestAScopedGreenClosesOnlyItsGroupsRedWhileItApplies` (cmd, real writer). Open entries for `fast-static-build` and `cmd-metasystem`; scoped trunk green with `Ran` holding only `fast-static-build`. Rows: tip is the proved commit: one closed, `Executed: true`. Tip moved by a records commit: one closed. Tip moved by an executable commit: none closed, "no longer applies". A sighting added during the proof: `TRUNK_RED_CHANGED`, none closed. A flake entry for the same group: one pass counted, not closed. Mutations: drop the `Ran` filter (the second entry closes); drop `Applicable` (the executable row closes).
- D6 `TestANoActRowIsANoticeNotAQuestion` (plain, `newStewardBed`). Goal waiting 121 min, no lane act: zero questions, one intervention `notice`; next step: none. Then a person's explicit `ask` disposition on the same occurrence: one question. Mutation: remove the skip.
- D6 `TestAStewardQuestionIsNeverTheLanesWait` (plain, question_status_test.go bed). A "lane steward: " question: a steward line, no "Waiting for a person's answer", not the summary. The agent's own lane question: unchanged. Keep landing_steward_question_test.go:25-29 (own question holds, steward question does not). Mutation: route steward questions to `d.Questions`.
- D7 extend `TestIntentGoalReviewEmptyJoinRealClose` (intent_review_completion_test.go:257) with rows: one non-material finding, no file: one close through the real owner, decisions.md holds `| crit9:1 | noted | not material per the read |` and the original binding; a stale-binding file present: refused, file unchanged; a partly decided file with a bad row: refused, file unchanged; the commit form on a rebased commit (no `workOfCommit` match): closes through `closeChain` with the examination's own binding. One material plus one note: stops as today. Keep `TestReadOnTheBuildModelIsNotPromoted` (intent_unit_review_test.go:1302) unchanged. Mutations: write DECIDE rows for notes-only; skip the binding helper on the commit route.
- D8 `TestAReleasedRebaseLeavesNoOwner` (launch). `ReserveMutation` then release: reservation gone; with a child recorded: kept. Mutation: drop the removal in the closure.
- D8 `TestStopPassesAnExistingFinishedMutationOwner` (launch, `recoveryReservation`, injected prober). An existing reservation owned by a completed mutation, written without the new closure. Rows: dead mutation: `CancelRun(unit)` succeeds, reservation gone. Live: refused. Unknown: refused. Dead with a surviving child: refused, names the child. Mutation: remove the quiescence branch in `CancelRun` (the dead row fails).
- D8 `TestRebaseReviewStopKeepsTheReadAuthoritative` (cmd, public composition): `work rebase`, then `work review`, then `work stop`. The unit stops; the published subject and its read stay the original commit's; the rebased commit is not recorded as read. Keep `TestGateTreeFinishedDeadMutationKeepsLiveChild` (tree_reservation_recovery_test.go:94) and `TestCancelRunReleasesFinishedDeadMutationWithUnreadableCustody` (:62).
- D9 `TestAFailedClearIsReconciledWithoutARerun` (cmd, `newLandingAgentKeeper`, real writer). Scoped trunk green recorded; the first clear publication fails (endpoint error once). Next observation: the incident is closed, launches stay one. A third observation: no publication. Mutation: drop the reconcile call.

Existing tests that change (to confirm at brief time; hypothesis where marked):

- `TestLandingStewardAsksOnceWhenTheLaneCannotHeal` (steward_rows_test.go:329): the scenarios whose `tried` is "nothing: ..." (:416) expect a notice. "silent" keeps its ask.
- steward_rows_test.go:477-775: hypothesis, tests that ask on an overdue goal with no act change to a notice or get a person's `ask` disposition.
- landing_status_headline_test.go:40 and question_status_test.go:21: hypothesis, only if their question carries a "lane steward: " fact.
- landing_incident_test.go:501-560 (`TestIncidentLandingPushClearsOnlyNewestFreshFullGreen`): hypothesis, its `clearMain` stub sees the new `Applicable` field only.

## Landing contract

R-145-m1e permits the hand route; R-150-m1e still governs the push. Before the push:

1. Each unit passes its focused checks: the tests above, including the composed keeper traces (D2, D9), D3's first admitted retry then refusal, and D6's notice then a later explicit `ask` with the agent's own-question hold kept. Plus the contract selections that cover the touched packages: `landing-command-standard`, `launch-standard`, `agent-protocol-standard`, `proof-standard` and the carry groups they pull.
2. One independent Astra implementation review of the combined diff. This design critique is not that read.
3. One full landing suite on the final integrated candidate: `proof.full` (metasystem.conf:16), with no `LANDING_ONLY`, no `LANDING_PROOF_SCOPE=scoped` and no replay filter, so `full` is true (internal/repoproof/full.go:149, :174). Cadence sections stay deferred as the runner defers them (:312-315); no extra race or coverage battery.
4. The candidate's tree, environment, completed execution inventory and verdict are kept in the evidence folder.
5. Any executable movement of main after that run is merged and reconciled before the push. A records-only move keeps the run. The keeper's status alone never certifies the landing.

## Amendment to the-lane-batches-goals-and-proves-once (D6)

Replace the bullet "`RecoverMovedMain` stays as is. ..." (the-lane-batches-goals-and-proves-once.md:164) with: "`RecoverMovedMain` yields to the open batch's next step (the-lane-finishes-what-it-starts D2): while the batch has a member to merge, an acceptable gate without a candidate result, or a green candidate not yet pushed, and the keeper can still run the agent (no barren stop, no question, provider or drain hold), recovery starts nothing. Otherwise it proves main only when main's commit and tree have no applicable recorded proof and were not already examined (D4 of that page). U4b adds its gathering and readiness states to the same predicate. It never reads the clock and never recreates cadence." The acceptable-gate predicate of this page's D3 is the one U1a builds.

## Acceptance a person can run

- A merged batch whose gate is skipped with a green static group gets its full proof and push with no person act: `metasystem landing status` shows the batch proving within one keeper wake, then pushing, and no recovery proof in between.
- A static red closes on the next scoped green that ran its group on current main: `metasystem goal show trunk-red` shows the entry closed by that attempt.
- Main's unchanged tree is proved once: results.jsonl holds one result per main tree while nothing moves.
- A notes-only read publishes itself on both forms: `metasystem work review G --work U` and the commit form end collected, with no `--dispositions` step.
- After `work rebase`, `metasystem work review` closes the unit run or `metasystem work stop run:ID` releases it.
- An overdue goal with no lane act shows as a steward notice; `metasystem landing status` has no "Waiting for a person's answer" line for it.

## What this page does not change

- The batching page's gathering, cadence, records reuse and full-before-push (the-lane-batches-goals-and-proves-once), except the D6 recovery clause above.
- The records-only fold (a-push-folds-a-records-only-move).
- Test identity and the final-prefix repeat (a-return-names-the-failing-test, a-final-prefix-red-is-repeated-before-own).
- Real decision rows (a-lane-row-carries-its-act) and minute counting (a-persons-lane-act-is-not-overdue).
- The rebase routes themselves (a-hand-in-stands-on-main).
- `hold.go` and the fix-goal admission (report item 3).
- The flake closing rule (fix plus three executed passes).

## Critique record

- Round 1, 2026-11-08, prompt 10:15 CET, findings written 10:30, log designs/astra-finishes-r1.log, findings designs/astra-finishes-r1.findings.md: 7 material, 0 minor, verdict build after the folds. Folded in r2, as a1 decided them: F1 as D5(c)(d), applicability at the publication boundary with the expected-entry check kept, trunk-red only, Problem 5 extended, real-writer test rows; F2 as D2(b)-(e), the batch's merge, prove and push before recovery on recorded progress and the barren and hold fallback, sixty-minute expiry dropped, the Amendment section, the keeper replay test, Problem 2 corrected (Observe runs before the clock); F3 as D7, one notes-only preparation for both routes, the shared binding helper, no overwrite, the read's identity kept, Problem 8 extended, TestIntentGoalReviewEmptyJoinRealClose extended; F4 as new D9 and Problem 6, reconciliation through the idempotent owner with no rerun; F5 as D4(b)(c), the examined check under the admission lock, never Settled or push, the lock-race test; F6 as D8's two separate tests and the public rebase, review, stop test, Problem 9 corrected (the unit was closed; the refusal is the distinct-owner check); F7 as the Landing contract section. Units recounted: U1 split into U1a and U1b.


## Revision 3: Astra round 2 folded (a1, 2026-11-08 10:5x)

Astra round 2 (`designs/astra-finishes-r2.findings.md`): 5 material, 0 minor, verdict build after the folds (round 1 was 7). The critique converged; a1 folds the five as binding constraints on the units below. The briefs and the builder take them as part of the design.

| Astra r2 | Binding constraint |
|---|---|
| F1 D5's applicability guard disappears during journal recovery | A guarded clear that is not yet published is not reconstructed from the journal when its applicability checker is unavailable: that journal operation is terminalized and D9 resubmits it through the checked owner. Already-published operations keep ordinary recovery. (U1b) |
| F2 D8's quiescence predicate does not guarantee the live-owner refusal | Reclaiming another mutation's saved reservation requires proven process death and ended children in the shared predicate; the mutation's explicit release closure stays its separate completion path. Tests: completed owners with live, unknown and dead observations while critic custody is readable. (U3) |
| F3 D9 strands incident effects after a successful batch push | D9 reconciles every eligible executed green that now applies to main, including a published full batch proof; records-only descendants pass D5's applicability guard; the original executed proof is the evidence, an inherited result never counts as execution. (U1b) |
| F4 D7's binding check rejects the standalone commit binding it generates | The binding grammar round-trips both identities, named work and standalone commit, with exact comparison of goal, subject, examination, round and return digest. Tests: the real commit-form command with no workOfCommit match, with notes and with a previously generated zero-finding document. (U3) |
| F5 D2 mistakes an earlier candidate in the same batch for the current one | Gate and candidate progress are bound to the current checkout subject and the current batch membership; an earlier cancelled or obsolete candidate never counts as the current candidate's result; the existing bounded fallback stays. (U1a) |

Landing route confirmed by Astra: one combined push after the added fixtures, an independent Astra implementation review of the combined diff, and one unfiltered proof.full green on the final candidate; the candidate tree, environment, execution inventory and verdict are retained; later executable movement is reconciled before the push.
