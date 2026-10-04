# Design for records-land-through-the-lane

- Kind: design
- Id: 01M446E2Q92FQTXF3SVYGWNFJV
- Status: accepted
- Goals: records-land-through-the-lane
- Critique: closed at round 2 on 0 material findings (Codex Astra: round 1 six material, folded; the confirmation read closed all six)

Accepted 2026-10-04 22:15 by the ui seat for the build: tier 2, one Astra round and one confirmation read; m1e agreed the dispositions at 22:02 (rulings and cross-branch ids out of step 1; unit 3 waits for lane-reproves units 3 and 4).

## 1. The rule

A record file travels like a unit commit. A seat names its claimed goal and the record files; the engine makes one plan commit of them on goal/G through the branch commit owner and its token (`commit_repository.go:139-224`), publishes it through the push owner, and hands it to the lane as today. The lane merges, proves and pushes; no person pushes. The path-class rule stays one rule read two ways (`validateCommitPaths`, `range.go:288`): a build commit touches only unit-class paths, a plan commit only plan-class paths; no class moves.

Rulings: R-121-m0, R-124-m1u, R-126-m1e, R-55-m1 (the one manifest), R-64-m1, R-129-ui; `docs/project-rules.md:53`.

## 2. The act

`metasystem work land G --records PATH... [--delivered TEXT]`, from the seat holding G's claim.

1. Each PATH must be plan class by `branch.PathClass` and `record` by the path-class manifest (`pathclass.Load`, resolved as `observe.go:1209` does); `metasystem/plans/README.md` and `metasystem/records/README.md` are `behavior` there. Else: "PATH is not a record; a build carries it: metasystem work review G --changes".
2. Bytes from the caller's checkout, narrowed to PATH..., staged in the goal worktree (`stageManual`, inside `conn.section`). Already at goal/G's tip: no commit; publication and hand-in continue (R-129-ui). No branch and nothing changed: unchanged success, "PATH already says this on main". A conflicting goal/G change: refused, as `work review --changes`.
3. The records check (`project.Read`, `devgate static`'s checker) runs in-process on the staged worktree; a problem refuses with path and line.
4. `conn.commit(CommitRequest{Kind: branch.Plan})` under the token; with no branch yet, goal/G starts at main's tip as a first unit does.
5. After the section, `conn.push` publishes goal/G (as `intent_manual_submit.go:325`); the hand-in reads origin's tip (`goalBranchOriginTip`). A failed publication is partial, "goal G's records are committed as SHA but not published: CAUSE"; the same command publishes that commit and never makes another.
6. `landGoalRoute` on the published tip, unchanged; a tip already queued, landed or returned answers as `work land G` does.

## 3. The record set

A record: plan class by `PathClass` (`metasystem/plans/**`, `metasystem/records/**` minus reads) and `record` by the manifest, which drops the two READMEs and the ledger. `metasystem/memory/rulings.md` stays unit class: a ruling keeps today's route until a later step makes it a record with the append-only and id-mint checks it has elsewhere (`observe.go:1529`, `landpath/land.go:597`). Docs and other memory files stay unit class. An accepted design is the same file with its head edited (`designgate/gate.go:76-82`); the hand-in carries the status change, which the push-time design check reads from the merged tree.

## 4. Hand-in without units

`handLandingSubject` refuses `len(units)==0`. Change: a range with no unit commit and at least one plan commit lands `state.BranchTip`; an empty range keeps the refusal. With units: unchanged (all read clean lands the tip, trailing plan commits included; else the unread unit is named). Records behind an unread unit wait. Why no read: a plan commit carries no behavior-class path; a record's review is the audits, at hand-in and in the lane.

## 5. The proof

Today the lane merges `--no-ff` and runs the whole suite in the VM (16 to 30 minutes), batched with whatever waits; it never runs `fast-static-build`, so without the check at hand-in a bad record head would first be seen by main's static gate. Two hand-ins in one batch copying one id pass alone and fail main's gate: improbable, today's exposure; later the lane checks the merged candidate.

Goal lane-reproves-only-what-a-change-can-affect owns scoping; none of its units is on main. Once its units 3 and 4 land, unit 3 here adds to its decision (its design, section 4): a batch whose every changed path against `origin/main` is `record` or `ledger` class by the manifest takes main's newest full green under an hour as its base and is proved scoped, with the groups its paths select and the owed full proof as for any scoped push. No second selector, no new variables; it removes for records-only batches the first-proof-is-full case that design defers (section 9). Until then a records-only batch is proved in full.

## 6. What a person sees

Hand-in: "goal G's records at SHA handed to the lane; its landing agent proves and pushes it"; a repeat says waiting, landed or returned. Refusals: one line, one command. `landing status`: "goal/G records at SHA from ui · waiting"; the queue `Line` gains `records: true`, changing the JSON layout test in the same unit. The channel posts `--delivered` when it lands.

## 7. The goal after a records landing

Stays open. The merge leaves goal/G untouched; its merge-base with main becomes the landed tip, so the next range holds only later commits, linear. `goal done G` sweeps the branch as today.

## Units

| Unit | Content | Lines |
|---|---|---|
| 1 | `--records`: class checks, refusals, idempotent repeat, records check, plan commit, publication, `handLandingSubject`, help | 320 |
| 2 | Queue line `records`, the status word, the hand-in summary; layout and message audits | 60 |
| 3 | Records-only batch proved scoped in `plain.Run`; depends on lane-reproves units 3 and 4 | 90 |

Tests, red first, Git stubbed (`development/project-rules-local.md`):

- Unit 1, `intent_delivery_test.go`, stubbed owners: `TestRecordsLandAsOnePlanCommitOnTheGoalBranch` (one `Plan` request, one push), `TestRecordsRefuseACodePath`, `TestRecordsRefuseABehaviorReadme`, `TestRecordsAlreadyOnTheBranchMakeNoSecondCommit`, `TestRecordsRefuseABadRecordHead`, `TestRecordsStartTheGoalBranchAtMain`, `TestAFailedPublicationIsPartialAndTheRepeatPublishesIt`, `TestAPlanOnlyBranchLands`, `TestAnEmptyBranchStillRefuses`, `TestRecordsBehindAnUnreadUnitWait`.
- Unit 2: `TestLandingStatusSaysRecords`; `TestAuditOutputLayoutJSONUnchanged`, `TestAuditMessagesAPersonReads` updated.
- Unit 3, `plain/scope_test.go`: `TestARecordsOnlyBatchIsProvedScopedOnMainsGreen`, `TestARecordsOnlyBatchWithoutARecentGreenIsFull`, `TestABatchWithACodePathKeepsItsFullFirstProof`.

## Open questions for Wido

1. The record check at hand-in (2.3): the lane does not run it today; alternative: let a bad head reach main until lane-reproves lands. Recommended: keep it.
2. `--delivered` required for records? Recommended: optional, as for units.

## Later, when it hurts

- A ruling as a record, with its append-only and id-mint checks.
- The lane's project check on the merged candidate.
- Records ahead of unread units (`--through` for plan commits).
- Docs and memory edits as records.
- A hand-in from a seat not holding the goal.
- The design check under refuse mode.
- `landing status --verbose` naming the files.

## Critique round 1

| Id | Disposition | Where |
|---|---|---|
| RL-01 | Accepted: manifest `record` class required too; behavior READMEs refused | 2.1, 3, `TestRecordsRefuseABehaviorReadme` |
| RL-02 | Accepted by narrowing: rulings stay unit class; ruling as record later | 1, 3, later list |
| RL-03 | Not material for step 1; later the lane runs the project check | 5, later list |
| RL-04 | Accepted: publish through `conn.push`, hand in the published tip; partial on failure | 2.5, 2.6, two unit 1 tests |
| RL-05 | Accepted as dependent unit 3, after lane-reproves units 3 and 4 | 5, units, unit 3 tests |
| RL-06 | Accepted: idempotent repeat, no second commit; repeat test | 2.2, 2.6, 6, `TestRecordsAlreadyOnTheBranchMakeNoSecondCommit` |
