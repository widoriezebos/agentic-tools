# The lane proves only what is new

- Kind: design
- Id: the-lane-proves-only-what-is-new
- Status: accepted
- Goals: a-push-folds-a-records-only-move, the-steward-proves-main-on-the-live-clock
- Critique: round 1 (Astra, designs/astra-lane-proves-new-design.r1.findings.md; 5 material) folded in revision 3; round 2 (designs/astra-lane-proves-new-design.r2.findings.md; 0 material, "DESIGN STANDS") notes folded in revision 4; closed

Accepted 2026-11-09 by a1 in Wido's name. Drafted for Wido (Wido 08:38: "run the analysis … design the solution(s) … get this defined as goal(s) and get this implemented with prio 1 before anything else"). Revision 2: repackaged after the analysis closed (its round 3: do not bundle what can land apart; use the goals that exist). Revision 3: Astra design round 1 folded (five material findings): D1 handles the archive move as a deletion plus an addition; the classification of a replay red on main is taken out (it would have removed the proof that clears a transient main red, with nothing in its place); the brief of item 2 gains a test; decision 226's order is replaced for item 1. Revision 4: Astra design round 2 found nothing material; its notes are folded (the estimate of D1, the call site of D2, the conclusion that still costs a check, the test fixtures).

Source analysis: designs/analysis-unit-speed-2026-11-09.md (revision 4, closed after three Astra rounds), with the proof-by-proof trace designs/lane-forensics-2026-11-09.md and the seam read designs/lane-seams-2026-11-09.md, all in the evidence directory /Users/wido/agentic-tools-evidence/lane-test-20261009/. Code read at main f4834c582a; paths are under metasystem/, `plain/` is internal/landing/plain/.

## Problem

Half of a goal's time from claim to landed is the landing stage (six goals, 85 goal-hours, 49.7%). The best observed landing took 143 minutes; its segments add up to about an hour when nothing is repeated. In 56 hours the lane started 28 full proofs for 12 landings. Thirteen are kept under the current rules. Fifteen repeated what the lane already knew, or could have learned from a narrower run. Unit checks wait behind the same proofs.

Most of the fifteen already have an owner that is built or prepared and has waited in the queue. This page orders that work and decides the two small rules that have no owner.

## The program, in order

Every item is its own goal and can land alone; goals handed in close together share one batch proof.

| # | Goal | What it removes (proofs of the trace) | State today | Route |
|---|---|---|---|---|
| 1 | a-push-folds-a-records-only-move | a second full proof after a goal's conclusion moved main (#8), and the records check of main after an ordinary conclusion | approved 11-01, design gate off, never started | **D1 of this page is its brief.** One unit. Decision 226 placed it after batching U2; that order is replaced here: D1 changes only the classifier (internal/goal/goalmove.go and `goalRecordsBetween`), which no batching unit touches |
| 2 | a-return-names-the-failing-test, unit U1a | whole-package replays for a failing subtest; gives item 3 its names | approved 11-04, final brief briefs/a-return-names-the-failing-test-U1-brief.md (Astra round 1 folded) | built from its brief with one addition: `TestTrunkRepeatKeepsWhatOnlyTheRepeatSaw`, case `new-subtest-in-whole-repeat` (plain/repeat_attribution_test.go:101) expects an empty `LANDING_ONLY_TESTS` for `TestA1/sub`; U1a selects `TestA1` there, so the fixture is updated and its assertion kept (a sibling failure only the repeat saw keeps its own identity and evidence). U1b (the return's wording) stays open with the goal |
| 3 | a-final-prefix-red-is-repeated-before-own | a goal returned as its own red with no run on its own tree, and the person's full proof that follows (#14, #16) | approved 11-07, design gate off, scope N4 and section 7 of designs/analysis-fixes-2026-11-07.md (two Astra rounds) | built from that scope, unchanged, after item 2 |
| 4 | the-steward-proves-main-on-the-live-clock (new) | a full proof of main decided on a clock frozen before the previous green was written (#28) | does not exist | **D2 of this page.** One unit |
| 5 | the-lane-batches-goals-and-proves-once, published units U1a, U2, U5, U3b | clock, hourly-debt and sticky-flag full proofs (#13, #15, #24); records reuse (#11) | built and published on the goal branch; the goal is parked behind unit-route defects 574 and 576 | not a decision of this page: either the unit-route goal's U5 and U6 unblock the hand-in, or the published units land by the hand route |

Order by delivery: 1 and 2 start at once on the two seats (each is one small unit); 3 follows 2 on the same seat; 4 follows 1. Item 5 runs beside them.

Not in the program, and why:

- A full proof of main right after the lane recorded main's red from a replay (#3, #6, #12; 155 minutes). Two of those three proofs came out green and closed a transient incident that had no fix goal; they were the only thing that released the held landings. Replacing them needs the lane to rerun an open incident's own tests on main and to close the incident from their passing outcomes (today a scoped green closes an incident only by its declared group, and the standing scope does not run the failing tests at all). That is a design of its own, on top of the closure rules the-lane-finishes-what-it-starts is landing. It is written after that goal is on main. A red whose incident write failed (#20) goes with it.
- A template-covered records path (#19), a full red whose failures later passed (#21), a standing red that masks a newer full green (#17): one occurrence each, in the same standing logic. They wait with the item above.
- Any way for a batch to land while main is red. A red on main outranks every landing (plans/designs/a-red-is-classified-once.md:124). The analysis adds an operating rule for the stewards instead (first restore main where the act that broke it is reversible, then repair); it changes no engine code.
- The unit route (a red check that corrects itself, the builder's own check, parallel groups): 7% to 11% shares with unmeasured reach; later.

## Decisions

**D1. A goal's conclusion is a records move.** Concluding a goal deletes `plans/goals/<id>.md` and adds `records/goals/<id>.md` in one commit. Today neither half is admitted: `goal.GoalRecordPath` (internal/goal/goalmove.go:29-31) knows only `plans/goals/*.md` and the incident register, and `goalRecordsBetween` (plain/trunk_recovery.go:228-250) reads every changed path from the new tree, so the deleted live page fails at `git show` (:243-246) and the move is not records-only. Two changes:

1. `GoalRecordPath` accepts `records/goals/<id>.md`. `GoalRecordsOnly` (:9-25) reads it with the same parser as a live page (`ParseFile` verifies the `Integrity:` trailer and the history, internal/goal/file.go:574, :598); a page that does not parse refuses, as today.
2. `goalRecordsBetween` takes the changed paths with their status. A deleted `plans/goals/<id>.md` is admitted only when the same move adds or changes `records/goals/<id>.md` and that page parses as the goal `<id>`: the goal's identity stays on main. Every other deletion refuses, as today (deleting a goal that project memory still names would break `internal/project/goals.go:82`, which reads both directories).

A conclusion is one commit but not always two files: priority compaction and goals that return from waiting change other live pages in the same commit, and those are admitted as the surviving, parseable goal pages they are. `GoalRecordsOnly` stays as it is; an admitted deletion is simply not handed to it. Git's rename detection stays off (`--no-renames`), so a conclusion always shows as its deletion and its addition. Not admitted, and left to the check as today: a reopen (it deletes an archive), a conclusion that also writes `plans/goals/backlog.md` (a displacement acknowledgment; that file has another parser), an archive under the legacy `plans/goals/done/`. The classifier is a statement about one move, not a validation of the whole ledger: the verbs' own transaction checks own that.

Both readers of the classifier widen with it: `inheritFlakeRecords` (plain/trunk_recovery.go:181-221), so a move of main that only concludes a goal inherits main's full green without a run, and `inheritFold` (plain/push.go:242-255), so a batch that took such a move from main keeps its green. Their other guards (same environment, a full green base, no later red, a `FullAt` not in the future) are untouched.

Evidence: 11 of the 325 commits main gained in the window changed records/goals pages. Proof #8: a-fixture's batch was proved in full a second time because its third fold carried records/goals pages (47.7 minutes, after 27 of push races). Each such move of main today costs a five-group records check that takes the heavy slot (plain/prove.go:1316-1317) and answers the batch's agent "busy".

Rejected: widening to every path of class Record (records/reads, decisions, designs). Only goal pages have a parser that proves the file is a ledger page and nothing else; design pages are read by tests and by the publication check. Rejected: ignoring a failed read of a deleted path; that would admit the deletion of any goal page.

The condition under which D1 inherits a green it should not: a conclusion that parses and keeps the goal's identity, yet changes the outcome of a check. The known reader of named ledger pages in tests is refused by `TestAuditNoTestReadsANamedLiveLedgerPage` (cmd/metasystem/audit_live_ledger_test.go:21, :149), which covers both `plans/goals` and `records/goals`; a test that scans the directories or computes a page name is not covered by it. That risk exists today for every `plans/goals` edit, which is already inherited.

This is the remaining intent of the approved goal a-push-folds-a-records-only-move (its other rules went to batching U5 by decision 226; its design gate is off). The goal is built as it stands, with this decision as its brief.

**D2. The steward's proof of main decides with the time at which it decides.** A steward step freezes its clock when it begins (`seams.Now = func() time.Time { return now }`, plain/steward.go:80) and may reach `stewardTrunk` (:494-511) later. `decideScope` treats a full green whose `FullAt` is after the decision's clock as unusable (`age < 0`, plain/scope.go:142) and falls to "main has no full green to keep": full. The step keeps the clock it was given before that assignment and hands a copy of the proof seams carrying it to the `Trunk` call (plain/steward.go:420): `stewardTrunk`'s `applicableMainProof` and `Start` read that clock (the injected one in the beds, the real one in production; no direct `time.Now()`), the rest of the step keeps its frozen `now`. A `FullAt` that is in the future of that clock stays refused.

Evidence: proof #28 (60.8 minutes): the decision carries a clock 8 seconds before the previous full green of main was written (results rows 1224, 1225); one goal page had changed. The cause is inferred from the reason string and the code path; the unit's first test reproduces it or the unit is dropped.

## Units

Lines are production lines changed; the bound is 250 per unit and the forecast is doubled (memory: unit estimates run double).

| Goal | Unit | Decision | Files | Counted | Doubled |
|---|---|---|---|---:|---:|
| a-push-folds-a-records-only-move | U1 (last) | D1 | internal/goal/goalmove.go, internal/landing/plain/trunk_recovery.go | 40 | 80 |
| the-steward-proves-main-on-the-live-clock | U1 (last) | D2 | internal/landing/plain/steward.go | 8 | 16 |

## Tests

Injected clocks; no real waits; fixtures through the lane's existing beds. New tests carry `t.Parallel()` where their file's tests do.

- D1: `TestGoalRecordsOnly` case "archive" turns true and gains "archive that does not parse" (false); `TestGoalRecordPathNamesOnlyTheRegisterAndGoalPages` gains `records/goals/x.md` true and `records/reads/x.md`, `records/goals/x.txt` false (internal/goal/goalmove_test.go:40-81). `TestFlakeRecordMoveKeepsMainsFullGreen` case "goal-done" (plain/flake_inherit_test.go:222: it deletes the live page and adds the archive) joins the accepted moves, and gains three refusals: a live page deleted with no archive; a live page deleted with the archive of another goal; an archive whose page names a different id. The fold regression `TestAFoldOfGoalPagesKeepsTheBatchsGreen` (plain/fold_inherit_test.go:18, real Git) gains a batch that took a conclusion from main (inherits its green) and one with an unexplained deletion (does not). The Git fake of flake_inherit_test.go (:87) answers the new status query and keeps its name-only answers for the existing callers.
- D2: through `StewardStep`: a step whose frozen clock is earlier than the newest full green's `FullAt` and whose given clock is after it starts no full proof of main ("main already has applicable proof" or an inherited row); with the given clock before `FullAt` the decision refuses the green as today (`TestFlakeRecordMoveKeepsMainsFullGreen` cases "future-full-at" and "helpers-refuse-future" stay). The reproduction uses distinct frozen and given times and a changed goal-record tree, reaches the real proof path and fails against today's wiring; an exact-tree green or a health row that never fires does not count. If it cannot reproduce proof #28's decision from the shape of rows 1224 and 1225, the goal is withdrawn.

## What this page does not change

- The depth of a batch proof, the gate, the push checks, holds and exceptions, the incident register's format.
- The standing logic, the replay and the incident closure (plain/standing.go, scope.go, replay.go, cmd intent_landing_incident.go).
- Person acts: a person's proof, land-anyway and give-back stay as they are.
- Functions the open lane goals change. In trunk_recovery.go D1 touches only `goalRecordsBetween` (:228-250); the-lane-finishes-what-it-starts changes `RecoverMovedMain` and adds its own functions there. In steward.go D2 touches the clock at :80 and `stewardTrunk` (:494-511); the-lane-finishes-what-it-starts changes :288-376. replay.go belongs to items 2 and 3 alone.

## Measures

After the program's items are on main, over the next ten landings: full proofs per landing (window: 2.3), median hand-in to push (window: 237 minutes), full proofs of main not caused by a code push (window: 11 of 15). The steward reports them from results.jsonl and pushes.jsonl.

## Critique record

- Round 1 (Astra, 2026-11-09 10:15): five material findings. The replay-classification decision ran a full proof anyway (the replay row carries no environment) and removed the clearance of transient incidents: taken out, owned by a later design. D1 did not handle the deletion half of a conclusion: rewritten. Item 2's brief missed `TestTrunkRepeatKeepsWhatOnlyTheRepeatSaw`: added. Item 1 contradicted decision 226: replaced explicitly.
- Round 2 (Astra, 2026-11-09 10:27): no material finding; "DESIGN STANDS". Notes folded in revision 4.
