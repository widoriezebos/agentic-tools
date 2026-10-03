# Design for seat-path-lands-without-help

- Kind: design
- Id: 01M3Z6EXJH4CCXWDNZA9DWB8X2
- Status: accepted
- Goals: seat-path-lands-without-help

Revision 5: answers critique round 3 (one finding, RULING-R-142-m1e). Author: seat m1g (Fable 5.1). Cites in the changed parts were read at `4babb05ec`, the rest at `eee6cc25a` and `f85f27efe`; paths are under `metasystem/`.

Wido, 2026-10-01:

> TARGET: one landing mechanism. A seat without a lane lands exactly as the lane agent does for a batch of one [...] on its own landing tree. [...] Then delete the old seat self-land plumbing.

Threat model: our own agents and operators make mistakes and crash. Nobody attacks. What defends against anything else is removed, not fixed.

## 1. What exists

- `work land G` (`landGoalRoute`, `cmd/metasystem/intent_delivery.go:1801-1874`) reads the goal branch, checks the reads (`handLandingSubject`) and the gate (`admitLanding`). With a lane it hands in (`handIn`). With no lane it runs `landByHand` (to line 2004): candidate, landing test receipt, prepared landing, push, sweep, `landed.json`, board card.
- `internal/landing/plain` takes its folders as parameters. `Start` and `Run` prove `checkout`'s HEAD in a fresh detached worktree and record the result. `Push` puts HEAD on main only when that exact tree is green and HEAD contains origin's main.
- Two things tie `plain` to a lane today:
  1. `Start` launches `metasystem landing prove --wait --attempt ID`, and that verb admits only a registered lane (`admitLane`, `intent_landing_prove.go:52-65`).
  2. `proveInWorktree` picks the command's folder with `filepath.Rel(checkout, install)` (`prove.go:331-334`), which works only when the installation is inside the checkout.

## 2. What changes: a seat with no lane is its own lane, for a batch of one

### Folders

- **Records:** the seat's own installation, `<seat install>/artifacts/agents/landing/` (`plain.Dir`): `results.jsonl`, `running.json`, `pushes.jsonl`, `proofs/`, `proof-trees/`, and a queue holding only this seat's own hand-in lines.
- **Landing tree:** `<seat install>/artifacts/agents/landing/tree`, a detached git worktree of the seat's repository (`artifacts/` is ignored). Every run rebuilds it, so a crash or a stray file in it breaks nothing.

### Who merges

`work land G` does, as the lane agent's loop step 3 (`skills/landing-agent/SKILL.md`): in the landing tree, `checkout --detach` origin's main, then `merge --no-ff SHA`. SHA is what `handLandingSubject` returns: the branch tip, or the `--through` commit.

### The loop

With no lane, every run of `work land G` does this in order:

1. **Landed.** Right after the branch read, where the lane route asks `laneQueueState` (`intent_delivery.go:1851`): when main contains the subject (`plain.ContainedIn`), say so and offer `goal done`. The subject is this goal's work: the branch tip, or a `--through` commit that the branch reader classifies as a unit of goal G (its unit trailer names G: `branch.KindOf`, `internal/goal/branch/range.go:129`) and that is the tip or an ancestor of it. Any other `--through` falls through to `handLandingSubject`'s refusal (`:2082`). The check comes before `handLandingSubject`, which refuses a landed branch: "no committed work to land".
2. **Reads and gate,** as today. The seat's own hand-in line then keeps the `--delivered` sentence (Moved effects).
3. **Proving.** `running.json` names a live proof: wait for it (below), then go on.
4. **Merge** SHA on the latest main in the landing tree. On a conflict: `merge --abort`, refuse.
5. **Read the newest result for the merged tree** (`plain.ResultFor`):
   - none, or the last proof died: `plain.Start`; say "proving in the background".
   - green: the gate again, as today before the push (`intent_delivery.go:1967-1969`), then `plain.Push`. Main moves; `noteLanded` posts the sentence and writes the landed line, as today.
   - red: refuse.

"Main moved" needs no case of its own. A new main gives a new merged tree, that tree has no result, so step 5 proves it. If main moves between this run's fetch and its push, `plain.Push` refuses (not fast-forward) and the next run merges on the new main. The merge commit differs per run but the tree does not, and the rail is per tree, as on the lane.

`work land G` holds `tree.lock` in the records folder (the existing `lock.File`) from the merge to the start or the push, so two sessions on one seat take turns. A seat proves one tree at a time: a second goal's `work land` during a proof waits for it, outside the lock.

### What a repeat says

| State | Line 1 | Line 2 |
| --- | --- | --- |
| still proving at the bound | goal G at SHA merged on main M is still being proven since T | this command |
| proof started | goal G at SHA merged on main M; proving in the background | this command |
| green | landed COMMIT on main; goal G stays open until done | `metasystem goal done G --reason TEXT` |
| landed | goal G at SHA is on main; it stays open until done | the same |
| red | goal G at SHA merged on main M proved red; nothing was pushed; the log is PATH | `metasystem work land G`: after the fix is pushed to goal/G. When the red is not this goal's: `metasystem landing prove`, then this command |
| conflict | goal G at SHA does not merge on main M: both changed PATHS; nothing was proven | `metasystem goal show conflicts-resolve-unattended` |

**How the seat learns the proof ended.** The loop has one command. In the proving state `work land G` itself waits, at most 4 minutes, until `plain.ReadRunning` (`prove.go:119-135`) says no proof is recorded or its process is dead. Then it runs steps 4 and 5 once: green pushes, red refuses, and a dead proof left no result, so `plain.Start` proves again. Still proving at 4 minutes: line 1 says so, line 2 is this command. The table prints no `work wait --path`: a dead proof leaves `running.json`, so that wait never ended.

### The surface

`work land G` is the seat's surface. One lane verb also runs on a seat: `landing prove`, because the detached proof *is* `landing prove --wait --attempt ID`. It admits "the registered lane, else this seat's landing tree", and the same verb proves a red tree again. No new flag.

`landing push`, `landing status` and `landing return` stay lane-only. The seat's push happens inside `work land G`, right after its gate, and `work land G` is its status.

`landing prove` with no lane and no landing tree refuses:

```text
this seat has no landing in progress, so nothing was proven
metasystem work land GOAL
```

### The command

`landing.prove.command`, read by the same `config.Get` call, from the seat's installation: `<seat install>/metasystem.conf`, its `.local` sibling and the environment (`internal/config/resolve.go:272-277`). `work land G` checks it before the merge. Unset:

```text
this seat has no command that proves a landing, so nothing was proven
metasystem settings set landing.prove.command COMMAND
```

### Code that changes in kept packages

- `plain.proveInWorktree`: the folder comes from the installation's place in its own repository (`git -C INSTALL rev-parse --show-prefix`) instead of `filepath.Rel(checkout, install)`. A lane gets the same folder as today.
- `plain.MergeOne(repo, tree, main, sha)`: makes the tree when missing, checks out main detached (forced), merges, and returns the merge commit and its tree, or a `*Conflict` with the paths after `merge --abort`. New code, because the lane's merge is an agent's hand and the hand route's (`branch.PrepareLanding`) builds a receipt-bound candidate.
- The seat's launch seam starts the detached `landing prove` from the seat's installation, not from the landing tree, so the child resolves the seat.

## 2b. The lane wait stops no goal: the clock stops at the mark (Unit 1b)

Today the clock runs through the lane wait.

- **Every hand-in marks the goal, after its line.** `handIn` (`cmd/metasystem/landing_plain.go:69`) writes the queue line (`plain.HandIn`), then marks (`goal.LandReady`). Before the line it checks, read-only on the goal's file in the projection the gate just read (`landing_gate.go:48-65`), that the mark would be taken: the seat's own claim and no stop fence, `LandReady`'s own conditions (`verbs.go:2237-2245`) as one shared function. So a stopped goal still hands nothing in. A mark that fails after the line is repaired by the repeat: at the handed-in commit it returns from `laneQueueState` before the gate (`intent_delivery.go:1851`), and its "waiting" answer marks, or moves the mark as the hand-in would have.
- **A person's word binds to its tip, never to a mark.** `Gate` (`landgate.go:240-270`) takes the newest word given at the tip it checks, over the whole claim (from the history's newest `claim` line on, `verbs.go:1562`), not the newest since the mark (`newestWord`, `:208-233`): a `--through` hand-in's mark would hide the word for the unchanged tip. A word at another tip still says nothing about this one, and a send-back at this tip still refuses; the refusal's sentences stay. `LandedUnder` (`:558-564`) words the landed line from the same read. The mark's window stays for what shows the review's state (`VerdictsOf`, `ReadGate` for the card, `LandingDue`): there a new hand-in still reads as a new review. The line-before-mark order stays (it keeps a stopped goal out of the queue) but no longer carries the word's safety.
- **The one landing slot goes.** It capped at one the claims a machine holds outside the quota. With it, a seat's second goal is refused the mark and waits in the lane with its clock running: the defect. Removed: the loop at `verbs.go:2249-2255`, `landingByMachine` at `validate.go:430-457`, its explanation at `intent_planning.go:1323-1345`. Pinned by `landing_test.go:78`, `:463`, `:842-854`, `TestGoalCLILedgerLandingSlot`, `TestIntentQueueOnly`. Left open: `--queue-only` on unbuilt work, now for several goals.
- **The clock: one rule for every wait, whatever its outcome.** The clock stops at the mark (`Landing.At`) and runs again at the first of:
  1. the goal's own next job or proof being active: the first job record or proof attempt naming the goal that starts after the mark. One still running at the mark ends the wait at once.
  2. the next hand-in, which moves the mark.
  3. the act that lifts the mark: `LandReturn`, `goal done`, a release or a park.

  That span is the wait. `dispatch.WaitSpan` (new, in `internal/dispatch/budget.go`) computes it from what `ProjectBudget` already reads: the goal's job records (`:369-555`) and proof attempts (`:556` on), with their times. While the goal is marked, the projection subtracts the open span from elapsed (`:362`); the span starts no earlier than the budget.
- **The credit.** Every act that moves or lifts the mark adds the same span to `Claimed.IdleSeconds`: the move, `LandReturn`, and a release or park, which keep idle in the episode (`verbs.go:499-503`). The goal package cannot read job records, so the act takes the span's end from its caller as one argument, recorded with the intent so a replay credits the same seconds; it refuses an end before `Landing.At` or after its own time. `goal done` drops the episode (`:2742-2745`): nothing is left to write to.
- **A wait that ends in a landing is credited like a return.** The landed goal stays marked, so its wait stays subtracted; when it goes on, its next hand-in writes the credit.
- **The bound.** Work the seat does on the goal without a job (reading, writing a brief) during a wait stays uncharged. After a job of the goal's own, the clock runs until the next hand-in.
- **The exemptions go.** A pure wait no longer grows elapsed, so `landingStopReason` and `admissionBreachesFor` (`internal/dispatch/admission.go:509`, `:520`; calls at `:328`, `:333`) go, and `stopReasonFor` (`:500`) with them: its callers (`:325`; `stop.go:217`, `:337`, `:442`) read `liveStopReason`. A marked goal that works past its limit is refused and stopped like any other. The other `IsLandingClaim` reads are unchanged: the one-claim quota (`validate.go`), the current claim (`goalverbs.go:935`), the machine-wide admission skip (`admission.go:146`), `LandingDue` and `LandingClaimLines`.
- **`LandingOverdue`** (`turnverdict.go:1757-1775`) uses the same computation: its callers (`turnverdict.go:1833`, `internal/channel/report.go:131`) hand `LandingClaimLines` the open span. It prints, never stops.
- **Each wait starts at its own hand-in.** A hand-in that adds a line for a marked goal moves the mark to now when the queue holds an earlier line for the goal, or when the wait has ended under rule 1. The act is `goal.LandReady` given the span's end: instead of answering `AlreadyHolds` (`verbs.go:2246-2248`) it adds the span to idle and writes the record again (`:2256-2257`). A `--queue-only` mark still open, with no earlier line, stays: one wait. The move follows the same order. After it the grace clock (`landgate.go:320`), `VerdictsOf` and the card's word start at the new line (`review.go:526-536`), as after any later land-ready.
- **The return.** A new act, `goal.LandReturn`: the claim holder's own, `LandReady` reversed. It lifts the `Landing` record, adds the span to idle and writes the card `board.StageReturned`; with no mark it changes nothing. New, because nothing lifts the record while the claim stays. `work land G` runs it first whenever the goal's newest hand-in is returned, at any commit, then goes on: a new commit is handed in again. Today a return is read only at the returned commit (`landing_plain.go:48`). Lifting the record puts the goal back in the one-claim quota (`validate.go:463-481`): a seat busy with its next goal is refused, keeps the mark, and reads which goal to hand in or park first. The wait stays open meanwhile.
- **The holder's Stop.** A marked goal below the tier, past the gate's grace, is a due landing on its holder's Stop (`LandingDue`, `landgate.go:345`), which runs `work land G --json` and prints any answer not refused or failed as `LANDED G` (`cmd/metasystem/holder_step.go:27-59`). Changed: `LANDED` only when the answer carries a landing or a landed queue line, else `LANDING G: SUMMARY`, once, as today's `LANDING REFUSED G`. The Stop taking a return is wanted: it runs under the holder's own session (`:20-22`) and is the first moment the seat can learn of the return.
- **One rule changes.** `budget.go:359` drops all idle when the budget starts at or after the claim: every claim never released. It becomes "after".
- **No lane.** The clock is left as it is: marking the seat's own proof needs `LandReturn` on red and conflict.

## 3. The other forms of `work land`

| Form | What happens to it |
| --- | --- |
| `--through COMMIT` | Kept on the one mechanism: the merged SHA is that commit, as the hand-in already does. |
| `--queue-only` | Kept as is. It is `goal.LandReady` (`intent_planning.go:1318-1358`) and reaches none of the deleted code. |
| `j2:J` | Deleted. It lands nothing today: refused with a lane; without one it asks for `landing.batch-root` (`landJob`, 1632-1651), whose owner is deleted. |
| `--message --staged --local` | Kept as is. It is the commit boundary (`landpath.Commit`, `intent_land_staged.go:191-213`), which the pre-commit guard expects. It publishes nothing. |
| `--message` that publishes (`--staged`, `--path`, `--chain`, `--recertification`, `--direct-fix`, `--tests`) | Deleted in U5 (Q1). A hand-made change then goes on a goal branch and lands with `work land G`. |
| `--exception`, `--using-exception` | Kept (Q2). |

## 4. Questions

**Answered by Wido, 2026-10-03, recorded on the goal.**

- **Q1, delete the publishing `--message` forms:** "yes". D2 is unit U5.
- **Q2, delete `--exception` and `--using-exception`:** "not-now and explain in the new goal why this is delayed and why we should do it". Goal `landing-exception-gets-a-successor`, blocked by this goal, owns it.
- **Q4, a seat's conflict:** "this is too simplistic. This needs a proper design and review round. The goal is unattended functionality". Goal `conflicts-resolve-unattended` (tier 3) owns it.

**Open: Q3. Which proof command do this project's seats use?** The lane's value is the VM suite, in the lane checkout's local configuration, and the local rules start a VM run only on Wido's request. Recommended: the lane's value, set once per seat. The witness below uses a stand-in command. No unit waits on it.

## 5. What is deleted

**D1: what only the goal's hand route reaches.**

- `cmd/metasystem/intent_delivery.go`: `landByHand`, `prepareReceipt`, `receiptProves`, `resumeSweep`, `latestLanded`, `writeHandLandingCard`, `landJob`, the owner fields `landPrep`, `landPush` and `sweep`, and the `landing-intent/G` reads at the top of `landGoalRoute`.
- `cmd/metasystem/intent_land_release.go`: `recordReleaseSet`. The rest stays while the staged and exception forms use it.
- `cmd/metasystem/goal_branch.go`: `goalBranchLandPushRun`, `goalBranchSweepLanded`, `goalBranchSweepLandedAt`, and the full (non-candidate) mode of `goalBranchLandPrepRun`.
- `cmd/metasystem/intent_owner_calls.go`: the `landingTestReceipt` owner call.
- `internal/goal/branch`: `publish.go`, `publish_repository.go`, `publish_test.go`, `publish_policy_test.go` (`LandPush`).
- The tests of exactly these.

What each defended against:

- **Landing test receipt, testing-policy plan, budget episode:** an agent claiming a proof it did not run. Outside the model. The proof is now the project's command on the merged tree.
- **Parity check (candidate against working tree):** a proof that ran in the live working tree. The cause is gone.
- **Prepared landing, `landed.json`, `resumeSweep`:** a crash between the steps of a publication. In the model, but nothing is left to resume: landed is derived from main.
- **Sweep, release set and board card at landing:** cleanup and display, not a defence. Moved effects names their new owners.

**D2, unit U5 (Q1: yes).** The publishing branch of `runIntentLandStaged` (`runLandPath`, the gate and release wiring at 96-116, `laneRegistered`); the staged steps of `landpath/land.go`; their owners in `cmd/metasystem/landing_path.go`; `internal/landing/receiptline.go` and `synctransport.go`, whose only caller outside tests is `landing_path.go`; register rows 432-433.

**D3** left this page: goal `landing-exception-gets-a-successor` owns deleting the exception forms and what only they reach.

**Kept, with the caller that needs it.**

- `internal/landing/plain`, `lane`, `batchowner`: the lane and section 2.
- `landpath/precommit.go`, `commit.go`, `commit_record.go`, `token.go`, `stop.go`, `owners.go`, and `landing.Observe` behind them: the pre-commit guard (`precommit_entry.go`) and the commit boundary.
- `internal/landing/receipt.go`, `testing.go`, `registers.go`, `fastforward.go`: `test run` and `internal/testrun`. So `landing.receipt-bound-min` stays.
- `branch.Push`, `branch.Sweep` and the branch reads: goal work.
- `noteLanded` and `intentLanded`: the new route feeds them.

No setting and no registered refusal code dies in D1: the register's `landpath` rows belong to the commit boundary, the staged form and the exception. After D1 the receipt, policy and budget code is still reached by the publishing `--message` forms and by `test run`. U5 takes it out of landing.

**Rule for every deletion unit.** Delete the caller first, then only what the compiler and the unused-code check then name. A file with another caller stays. No test of kept code is deleted. No floor in `testing-coverage-floors.json` is lowered; if a deletion would drop a kept package under its floor, the unit adds tests of kept code or stops and asks.

## Moved effects

What the hand route did at a landing, and who does it after U4. A new symbol is named with the existing file that will hold it.

| Effect | From | To | Code |
| --- | --- | --- | --- |
| The merge onto main | `landByHand`: the candidate (`goalBranchLandPrepRun`, `branch.PrepareLanding`) | `landAlone`: `plain.MergeOne` (new, in `push.go`) | `metasystem/cmd/metasystem/goal_branch.go`, `metasystem/internal/goal/branch/land.go`, `metasystem/internal/landing/plain/push.go` |
| The proof | the landing test receipt (`prepareReceipt`, `landingTestReceipt`) | `plain.Start` and `plain.Run`, through `landing prove` | `metasystem/cmd/metasystem/intent_owner_calls.go`, `metasystem/internal/landing/plain/prove.go`, `metasystem/cmd/metasystem/intent_landing_prove.go` |
| The push | `goalBranchLandPushRun` (`branch.LandPush`) | `plain.Push` | `metasystem/internal/goal/branch/publish.go`, `metasystem/internal/landing/plain/push.go` |
| The landed record | `landed.json`, read by `latestLanded` and `resumeSweep` | derived from main (`plain.ContainedIn`); `noteLanded` still writes the landed line | `metasystem/cmd/metasystem/intent_delivery.go`, `metasystem/internal/landing/plain/push.go`, `metasystem/cmd/metasystem/landing_gate.go` |
| The branch sweep | `goalBranchSweepLanded`, after the push | `goal done`, the one verb that sweeps: `goal.Done` calls `SweepBranch` (`verbs.go:2317`) | `metasystem/cmd/metasystem/goal_branch.go`, `metasystem/internal/goal/verbs.go`, `metasystem/cmd/metasystem/goalsync_mutations.go` |
| The release set | `recordReleaseSet` and `runReleaseSet` | `goal done`: `steward.SweepGoalWorktrees` releases the goal's worktrees; one still in use waits for the disk pass | `metasystem/cmd/metasystem/intent_land_release.go`, `metasystem/internal/steward/disk_goal_worktrees.go` |
| The board card at landing | `writeHandLandingCard` | no card at landing, as on the lane route: the landed line is what `ReadGate` reads (`landgate.go:312-316`), and `goal done` writes the released card | `metasystem/cmd/metasystem/intent_delivery.go`, `metasystem/internal/goal/landgate.go`, `metasystem/internal/goal/verbs.go` |
| The delivered sentence | `landed.json`, from the pushing run's `--delivered` | the seat's own hand-in line (`plain.HandIn` at the first run, `plain.Say` on a repeat), so the pushing run posts the sentence given at the first run | `metasystem/internal/landing/plain/queue.go`, `metasystem/cmd/metasystem/landing_gate.go` |

## 6. Units, in landing order

Each lands by itself through this computer's lane. At most 300 changed lines, except U4 and U5. The Unit 1b units go first: no unit marks more goals than today (`--queue-only` only) before the exemptions are gone and the credit is in.

**U1b.1. The clock.** `WaitSpan`; the projection and `LandingOverdue` subtract the open wait; the exemptions go; the idle rule; a release or park of a marked goal writes the credit. Witness, stubbed records: `TestAWaitDoesNotGrowElapsed` (marked longer than the breach limit: `FindBreachStops` does not name it); `TestTheGoalsOwnJobEndsTheWait` (starting after the mark: elapsed grows from its start; running at the mark: no span); `TestAMarkedGoalThatWorksPastItsLimitIsRefusedAndStopped`; `TestAReleasedWaitIsCreditedAtTheNextClaim`; `TestIdleCountsForAClaimNeverReleased`. Mutations: drop the subtraction; ignore the job's start; restore either exemption; release without the credit; restore "at or after". Past 300 lines, the release and park credit lands second, ahead of U1b.2. Until U1b.3, a `--queue-only` goal that worked after its mark is charged for a later wait; today it is exempt.

**U1b.2. The slot rule goes,** with its pinned tests. Witness: `TestTwoGoalsOfOneMachineWaitToLand` (both marked longer than the breach limit; `FindBreachStops` names neither). Mutation: restore the loop.

**U1b.3. Every hand-in marks, after its line; the word binds to its tip; the mark moves; the Stop's line.** Witness, stubbed: `TestWorkLandWritesTheQueueLineBeforeTheMark`; `TestAStoppedGoalHandsNothingIn`; `TestAThroughHandInKeepsThePersonsWordForTheTip` (at the tier, a word at the tip: after a `--through` hand-in the tip's hand-in passes the gate); `TestLandedUnderNamesTheSettingOrTheWord` (existing: it gains a word given before the mark); `TestAWaitingRepeatMarksTheGoal`; `TestASecondHandInMovesTheMark` (the span goes to idle, the review shows no earlier word); `TestAQueueOnlyMarkStaysAtTheFirstHandIn`; `TestAWaitThatEndsInALandingIsCredited` (landed through a prefix: the next unit's job is admitted); `TestTheStopSaysLandingForAWaitingGoal`. Mutations: mark before the line; drop the check before the line; read the word only since the mark; drop the call in `handIn`; drop the repeat's mark; move without the credit; keep the old `Landing.At`; print `LANDED` for every taken answer. Past 300 lines, the moved mark lands right after, as U1b.3b.

**U1b.4. `LandReturn`, at any commit.** Witness: `TestLandReturnAddsTheWaitToIdle`; `TestAReturnedGoalIsStoppedOnlyAfterItsRemainingTime` (never released, waited longer than the breach limit: no stop at the return, a stop once its remaining time is used); `TestWorkLandTakesAReturnedGoalBack` (the mark stays beside another working claim); `TestWorkLandTakesTheReturnAtANewCommit` (then hands in again). Mutations: lift without adding the span; drop the call; read the return only at the returned commit.

**U1. `plain`: the proof folder and `MergeOne`.** Witness: one real-git test, `TestRealGitSeatTreeMergesAndProvesFromTheInstallationsFolder`; git is the claim. With an installation outside the landing tree: a clean merge has parents main and SHA, a stand-in command's `pwd` is the proof worktree plus the installation's prefix, and a conflict returns the paths and leaves a tree the next merge can use. Mutations: restore `filepath.Rel(checkout, install)` (the `pwd` is the worktree top); drop `merge --abort` (the next merge fails).

**U2. `landing prove` on a seat with no lane.** Witness, git stubbed through the existing seams: `TestLandingProveWithNoLaneProvesTheSeatsLandingTree` (launched from the seat's installation, the result in the seat's records), the two refusals above, and `landing push` with no lane still refusing. Mutation: remove the fallback in the admission.

**U3. `work land G` with no lane: the loop, and the switch.** `landAlone` replaces the call to `landByHand` at line 1873. Witness, stubbed with per-test owners: one test per state (`TestWorkLandWithNoLane...` `StartsTheProof`, `WaitsOutALiveProof`, `SaysStillProvingAtTheBound`, `RestartsADeadProof`, `PushesAGreenTree`, `RefusesARed`, `RefusesAConflict`, `SaysLanded`, `ProvesAgainWhenMainMoved`), plus `PostsTheFirstRunsSentence`, `WritesNoReceiptOrLandingIntent`, `ReadsTheGateBeforeThePush` and `RefusesAThroughOfAnotherGoal`. `SaysLanded` reads a branch whose tip main already contains, so its unit list is empty, not stubbed. `RefusesAThroughOfAnotherGoal` names another goal's commit that main holds: refused, not answered as landed. In `RestartsADeadProof` a dead proof's `running.json` makes the next `work land G` start the proof again. Mutations: take the newest result whatever its tree (main-moved pushes a stale green); push before `admitLanding`; call `prepareReceipt`; check landed after `handLandingSubject`; answer landed for any `--through` commit main contains; wait on the file alone. The verb's help and the docs that describe the hand route (`docs/working-with-agents.md`, `docs/project-rules.md`) change here. Past 300 lines, the green, red and landed rows become U3b and the switch moves there.

**U4. D1 and the `j2:J` form, a pure deletion.** Witness: U3's tests and the lane hand-in tests (`TestWorkLandHandsInToThePlainLane`, `TestWorkLandKeepsTheReadGateBeforeTheHandIn`) stay green; `go run ./cmd/devgate static` passes. Mutation: a deleted call that returns turns U3's no-receipt test red.

**U5. D2, a pure deletion.** `--message` without `--local` then refuses: nothing was landed; line 2 is `metasystem work land GOAL`. Witness: that refusal; the `--message --staged --local` tests stay green; `go run ./cmd/devgate static` passes.

**Last witness: one real landing on a seat with no lane, in a bed.** The lane record is read from the shell's home directory (`intent_landing.go:185-194`), so a scratch `HOME` has no lane and this computer's lane stays registered.

1. `export HOME=$(mktemp -d)`; make a bare origin and a clone of this repository at the built commit; `metasystem landing status` there says no lane.
2. In the clone: a tier 1 goal, claimed, with one pushed commit on `goal/G` that adds a file.
3. `metasystem settings set landing.prove.command 'test -f FILE'`.
4. `metasystem work land G --delivered "..."` says proving in the background. `work land G` again waits, pushes and says landed COMMIT; the channel carries the first run's sentence. A third run says landed. Origin's main holds the merge of SHA, `results.jsonl` has one green line, and there is no `landing-intent` folder and no receipt.
5. A second goal that fails the command shows the red lines; a third that conflicts shows the conflict row's two lines.

## 7. Out of scope

- Conflict recovery: for a seat without a lane, for the lane's conflict returns (`landing_plain.go:60`) and for moving a goal branch onto a newer main. Owner: goal `conflicts-resolve-unattended`.
- The 2026-10-01 hand-in frictions.
- The lane's own agent, keeper and launch (goal `old-lane-plumbing-is-deleted`), with `landing.batch-root` and `landing.batch-max-wait`.
- Receipts written by the landing (goal `land-verb-writes-the-receipt`).
- Who concludes a landed goal.

## 8. Deferred

- Thinning the commit boundary.

## 9. Not checked

- That `laneCheck`'s reader (`batchowner.ProductionLandingLaneSeams().BatchRoot`) reads the lane record from the same home directory as `laneContext`. Step 1 of the bed run shows it.
- The test-file list for D1 and every symbol in D2: the deletion rule finds them.
- Whether `internal/testpolicy/protection.go:38-47`, which names the `landpath` path, changes after D2.
- Line numbers outside the changed parts.
- For Unit 1b, the ledger: the one-slot rule's origin (its purpose is read from `validate.go:393-397`); whether `work land G` admits a non-holder today (the check before the line refuses one); what a new ledger verb or argument needs beyond `internal/goal/recover.go:543` (the steward reads `land-ready` rows: `seat_start.go:305`, `handoff_state.go:321`); an obligation discharged inside the hold drops all idle, so that goal's credited wait counts at once.
- The gate's word: that every claim starts with a `claim` history line; the landed line's tip once the branch has moved.
- For Unit 1b, the clock: which callers of release and park must pass the span's end; that both callers of `LandingClaimLines` can be handed the span; that no proof of the lane's or of `plain`'s writes a record naming the goal in the seat's installation (one would end the wait).
- What the Stop's hook does while its `work land G --json` waits out a proof; the wait holds no lock.
- That the steward's silent-lane check (`internal/steward/lane_silent.go:113`) and the keeper's wake read only the registered lane's queue.
- That `steward.SweepGoalWorktrees` covers every store a release set selects.

## Dispositions (critique design-critic-46437605ef0c580fea7461d1)

Written by metasystem design review when critique design-critic-46437605ef0c580fea7461d1 closed: every answered round's decisions, as the author made them.

| Round | Finding id | Finding | Disposition | Reasoning and evidence | Amendment |
| --- | --- | --- | --- | --- | --- |
| 1 | RULING-R-13 | Automatic marking suspends the elapsed budget for later implementation, not just the lane wait. Ruling R-13 says that “dispatch admission refuses at a limit boundary with structured evidence.” The design marks every hand-in, preserves partial landings, and clears the mark only on return or goal completion. After the first successful unit lands, work on subsequent units remains exempt. Materiality: the mark's lifecycle must change; step 1 is not SAFE because active work escapes its approved elapsed limit. | accepted | Re-read at eee6cc25a: `admissionBreachesFor` (`internal/dispatch/admission.go:518-530`) drops the elapsed breach from the goal's own work admission whenever the claim carries a `Landing` record, and nothing lifts that record before `goal done` (`RecordLanded`, `internal/goal/landgate.go:507-548`, leaves it). With a hand-in marking every unit, a goal whose first unit was handed in builds every later unit outside its elapsed limit. Also confirmed: a repeat of `work land G` at the handed-in commit returns from `laneQueueState` (`cmd/metasystem/intent_delivery.go`, `landGoalRoute`) before `handIn`, and a return is read only at the returned commit (`landing_plain.go:47-49`). | The mark covers a wait, never the goal's next work. (a) While a goal is marked, its own work admission keeps the elapsed limit: `admissionBreachesFor` no longer drops elapsed for the goal's own admission, so later work is refused at the limit with the structured breach; the breach stop (`stopReasonFor`) and the machine-wide check of the other claims still skip a marked claim's elapsed, so a waiting goal is never stopped. Say whether the landing's own proof keeps an exemption and why. (b) Each wait starts at its own hand-in: a hand-in that appends a new queue line while the goal is marked from an earlier one moves the mark to the hand-in's time, and the span before it stays charged, so `LandReturn` credits only the latest wait. Say the act that moves it and what it does to the landing gate's clock and verdicts (`ReadGate` and `VerdictsOf` read from the standing `Landing`). (c) A `work land G` that finds the goal's newest hand-in returned, at any commit, runs `LandReturn` before anything else, then goes on (a new commit is handed in again). (d) Every `work land G` that ends with the goal waiting in the lane leaves it marked, a repeat at the same commit included. State the remaining imprecision on the page: the credit ends at the seat's act, so work the seat did on the same goal between the return and that act is credited; it is bounded by one lane wait. |
| 1 | RULING-R-129-ui | A normal repeat after successful standalone landing is refused before reaching the proposed “landed” case. Ruling R-129-ui says: “An act whose effect already holds is success, not a refusal.” Materiality: containment must be recognized before validation that requires unlanded units; step 1 does not WORK on its first post-landing repeat or recovery after a successful push. | accepted | Confirmed: `handLandingSubject` refuses "has no committed work to land" when the branch has no unit left (`cmd/metasystem/intent_delivery.go:2053-2055`), and a branch whose tip main contains has none. The lane route avoids this only because `laneQueueState` answers first. | With no lane, the landed check comes before `handLandingSubject`, right after the branch read, in the same place the lane route asks `laneQueueState`: when main contains the selected commit (the branch tip, or the `--through` commit) `work land G` says landed and offers `goal done`. The `SaysLanded` witness reads a branch whose tip main already contains, so the unit list is empty, not stubbed. |
| 1 | SEAT-CONFLICT-RECOVERY | The prescribed conflict repair makes the goal branch invalid. Merging origin/main into the goal branch creates a commit with two parents, while the retained branch reader requires one. Materiality: the recovery instructions and their test must change; step 1 does not WORK after the first conflict because following its instructions prevents another landing attempt. | accepted | Confirmed: the branch reader refuses a goal-branch commit with two parents (`internal/goal/branch/range.go:270-272`), so "merge origin/main into goal/G" makes the branch unreadable. | The conflict row's second line is the goal's normal correction, the same route a lane return takes: `metasystem work revise G --brief FILE`, the brief naming the conflicting paths. A follow-up rebases the goal's work onto main when main touched its paths (`internal/dispatch/followup_rebase.go`); then `work review G`, then `work land G`. Never a merge of main into the goal branch. The page checks that this route really rebases onto the new main and says so with its citation. The conflict witness adds the recovery: after the branch is rebased onto the moved main (one parent per commit), the next `work land G` merges cleanly. |
| 1 | SEAT-DEAD-PROOF-WAIT | The prescribed wait cannot recover when the proof process dies. The abandoned running.json remains present, and every timeout directs the seat back into the same presence-based wait. The work-land invocation that detects death is never reached by that continuation. Materiality: the timeout or completion transition must change; step 1 does not WORK under the explicitly included crash scenario. | accepted | Confirmed: `work wait --path` observes presence only and its timeout continues the same wait; a proof that died leaves `running.json`, so the printed loop never reaches the `work land G` that would restart it (`internal/landing/plain/prove.go:119-134` holds the liveness read). | The seat's loop has one command. In the proving state `work land G` itself waits, bounded (a few minutes, said on the page), until `running.json` is absent or its process is dead (`plain.ReadRunning`), then reads the result once: green pushes, red refuses, dead restarts the proof. Still proving at the bound: line 1 says so, line 2 is this command. No `work wait --path` in the table. Witness: a dead proof's `running.json` makes the next `work land G` start the proof again. |
| 1 | MOVED-EFFECTS-SEAT-LANDING | The required Moved effects inventory is absent, and delivery-text retention has no replacement owner. The design transfers merge, proof, publication and cleanup responsibilities, but omits the required table. Its normal two-command example also loses the delivery sentence supplied before background proof. Materiality: ownership and retention must be specified; step 1 does not WORK as promised when the successful push silently omits that notice. | accepted | Confirmed: the page has no Moved effects section (`design review --check-only`: inventory absent), and the second run of the bed (`work land G` without `--delivered`) would push with no sentence: the hand route kept it in `landed.json` (`intentLanded.Delivered`), which the page deletes, and `plain.Running` and `plain.Result` carry none. | Add a "Moved effects" section with the four-column table the validator reads (Effect, From, To, Code), one row per moved owner, Code paths starting `metasystem/` and present at the tip: the merge onto main, the proof, the push, the landed record (derived from main), the branch sweep and the release set (say the verb that does them once the landing no longer does; the page's section 9 leaves this unchecked, so check it), the board card at landing, and the delivered sentence. The sentence's owner: with no lane the seat keeps the lane's own hand-in line in its records folder (`plain.HandIn`, and `plain.Say` on a repeat), so the sentence given at the first run is the one the pushing run posts. |
| 2 | RULING-R-142-m1e | The new landing mark can discard the human approval needed to retry the same hand-in. The human-authority ruling R-142-m1e says: "A machine rule that stops a person's decision from taking effect is a defect to fix at once, never a reason to refuse." For a goal requiring human approval, the gate accepts the approved commit, then handIn writes a newer landing mark. If the queue write fails or the process crashes there, the promised retry sees no approval and demands another human act. Materiality: the design must change how the mark preserves approval for the same commit. Without that change, the first interrupted hand-in fails WORK and contradicts the human's authority. | accepted | Confirmed at 4babb05ec: the landing gate reads a person's word only since the standing `Landing` record (`since` and `newestWord`, `internal/goal/landgate.go:208-230`), and the word is already bound to the tip it was given at (`:266`). A mark written by the hand-in after the gate hides the word that admitted it; if the queue line then fails, the retry at the same commit is refused for a word the person already gave. | `handIn` writes the queue line first and marks after it. Before the line it checks, read-only from the projection the gate already read, that the mark would be taken (the seat's own claim, no stop fence), so a stopped goal still hands nothing in. A mark that fails after the line is repaired by the repeat: its "waiting" answer marks (or moves the mark). The moved mark follows the same order. Then no gate read ever falls between a hand-in's mark and its line, and a person's word for the tip stays in force for every retry. Say on the page that the word's window is otherwise unchanged (a later land-ready still opens a new review), and drop the matching item from section 9. |
| 2 | RULING-seat-path-lands-without-help | A successful lane wait still consumes the next unit's work budget. The goal's NextStep requires that "a hand-in to the lane suspends the goal's elapsed clock exactly as goal land-ready does". The revised page instead explicitly never credits a wait ending in successful landing. A goal can hand in its first unit before its limit, wait beyond the limit, land successfully, and then be refused admission for its second unit solely because the wait remains charged. Materiality: this changes the elapsed-accounting contract and its fixtures. Without a correction, WORK fails for the page's own sequence of separately landed units. | accepted | Confirmed: revision 3 credits a wait only at a return, so a unit that waited past the limit and landed leaves the next unit refused for time nobody worked. The goal record asks for the land-ready behaviour (the clock suspended while the work waits); R-13 asks that the goal's own work be refused at its limit. Both hold if the wait ends when the goal's own work starts again. | One rule for every wait, whatever its outcome: the clock stops at the mark and runs again at the first of (1) the goal's own next job or proof being active (a job already running at the mark ends the wait at once), (2) the next hand-in, which moves the mark, and (3) the act that lifts the mark (`LandReturn`, or `goal done`). While marked, the budget projection subtracts that open span from elapsed; it already reads the goal's job and proof records with their start times (`internal/dispatch/budget.go`, `ProjectBudget`, 269-370 and the job loop after it). Every act that moves or lifts the mark adds the same span to `IdleSeconds`. A wait that ends in a landing is credited like a return. Work the seat does on the goal without a job (reading, writing a brief) during a wait stays uncharged: state that bound on the page. Because a pure wait no longer grows elapsed, the elapsed exemptions `landingStopReason` and `admissionBreachesFor` can both go (subtraction): a marked goal that works past its limit is refused and stopped like any other. `LandingOverdue` uses the same computation. Order the units so that no unit marks more goals than today before the exemption is gone and the credit is in. |
| 2 | SEAT-CONFLICT-RECOVERY | The accepted conflict-recovery finding remains open. The page now correctly rejects the earlier recovery premise, but leaves the implementer choosing between preserving a conflicted landing tree, replaying the goal branch, or providing no recovery. These choices change control flow and the recovery fixture; they are not interchangeable help text. Materiality: without a selected route, WORK fails at the first conflicting landing because repeating the specified abort-and-rebuild loop reproduces the conflict. | accepted | Wido ruled on 2026-10-03 (answer to question PTHRDZKHX9W0R5PVEQZT5E3K9W, recorded in the intent of goal `conflicts-resolve-unattended`, opened the same day): "this is too simplistic. This needs a proper design and review round. The goal is unattended functionality". That goal owns conflict resolution for a seat without a lane, for the lane's conflict returns and for moving a goal branch onto a newer main. | Remove Q4 and its options. The conflict row stays a refusal: line 1 names the paths and says nothing was proven; line 2 is `metasystem goal show conflicts-resolve-unattended`, the goal that owns the recovery. Section 7 lists conflict recovery as out of scope with that goal as its owner. U1 keeps `merge --abort` and the conflict's paths; the recovery witness is dropped from U3 and from the bed run. |
| 2 | SEAT-LANDED-SUBJECT | The early landed check accepts a --through commit without establishing that it belongs to the goal. An operator can accidentally paste another goal's commit already on main and receive a landed answer plus an invitation to conclude this goal, while this goal's work remains unlanded. Materiality: the design must retain subject validation before claiming success, while still accepting legitimate already-landed prefixes. Without it, SAFE fails through a silent false completion answer under the stated operator-mistake threat model. | accepted | Confirmed: `handLandingSubject` checks that a `--through` commit is one of the goal's units (`cmd/metasystem/intent_delivery.go:2074-2083`), and `plain.ContainedIn` checks only containment (`internal/landing/plain/push.go:119-126`), so the early landed check would accept any commit main holds. | The landed check's subject is this goal's work: the branch tip, or a `--through` commit that the branch reader classifies as a unit of goal G (its unit trailer names G: `branch.KindOf`, `internal/goal/branch/range.go:129`) and that is the tip or an ancestor of it. Any other `--through` falls through to `handLandingSubject`'s refusal. Witness: `--through` naming another goal's commit that main holds is refused, not answered as landed. |
| 3 | RULING-R-142-m1e | Automatic hand-in marking still cancels approval for unchanged work when landing in parts. Ruling R-142-m1e says: "A machine rule that stops a person's decision from taking effect is a defect to fix at once, never a reason to refuse." Consider a goal requiring human approval, with two reviewed units and no landing mark. A person approves its branch tip. Handing in only the first unit with --through passes the gate, then creates a mark that hides that approval. Handing in the remaining work, without changing the approved branch tip, misses the first queue entry and is refused for missing human approval. Writing the queue first fixes the interrupted-write case but does not fix this supported flow. Materiality: the approval-window contract and partial-landing fixture must change. Without that change, WORK fails through unnecessary human intervention, and SAFE fails by overriding the existing decision. | accepted | Confirmed at 4babb05ec: `admitLanding` checks the word against the branch tip (`cmd/metasystem/intent_delivery.go:1867`), the word is already bound to the tip it was given at (`internal/goal/landgate.go:266`), and `newestWord` reads only since the standing mark (`:208-230`). A `--through` hand-in marks the goal, and the next hand-in at the unchanged, approved tip no longer sees the person's word. Ordering the line before the mark cannot fix this: the hiding is the window itself. | A person's word binds to its tip, never to a mark. The gate (`Gate`, `landgate.go:240-270`) takes the newest word given at the tip it checks, over the whole claim; a word at another tip still says nothing about this one, and a send-back at this tip still refuses. The mark's window stays for what shows the review's state (`VerdictsOf`, `ReadGate` for the card, `LandingDue`), so a new hand-in still reads as a new review there. With this, the line-before-mark order stays (it keeps a stopped goal out of the queue) but no longer carries the word's safety. Witness: `TestAThroughHandInKeepsThePersonsWordForTheTip` (a tier at or above human-from-tier, the word given at the tip, a `--through` hand-in, then the hand-in of the tip passes the gate); mutation: read the word only since the mark. It lands no later than the unit that makes every hand-in mark. |
