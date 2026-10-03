# Design for seat-path-lands-without-help

- Kind: design
- Id: 01M3Z6EXJH4CCXWDNZA9DWB8X2
- Status: draft
- Goals: seat-path-lands-without-help

Revision 3: answers critique round 1 (five findings). One ruling failed its check, so the conflict's recovery is Q4. Author: seat m1g (Fable 5.1). Cites in the changed parts were read at `eee6cc25a`, the rest at `f85f27efe`; paths are under `metasystem/`.

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

1. **Landed.** Right after the branch read, where the lane route asks `laneQueueState` (`intent_delivery.go:1851`): when main contains the selected commit (the branch tip, or the `--through` commit; `plain.ContainedIn`), say so and offer `goal done`. It comes before `handLandingSubject`, which refuses a landed branch: "no committed work to land".
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
| conflict | goal G at SHA does not merge on main M: both changed PATHS; nothing was proven | waits on Q4 |

**How the seat learns the proof ended.** The loop has one command. In the proving state `work land G` itself waits, at most 4 minutes, until `plain.ReadRunning` (`prove.go:119-135`) says no proof is recorded or its process is dead. Then it runs steps 4 and 5 once: green pushes, red refuses, and a dead proof left no result, so `plain.Start` proves again. Still proving at 4 minutes: line 1 says so, line 2 is this command. The table prints no `work wait --path`: a dead proof leaves `running.json`, so that wait never ended.

**The conflict.** The lane agent fixes a small conflict in the lane checkout and returns the rest (`skills/landing-agent/SKILL.md:40-41`). What a seat does is Q4.

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

## 2b. The lane wait stops no goal, and the mark covers only the wait (Unit 1b)

Today the clock runs through the lane wait.

- **Every hand-in marks the goal.** `handIn` (`cmd/metasystem/landing_plain.go:69`) first calls `inv.landReady` (`goal.LandReady`), then writes the line. Already marked counts as marked. A refused mark hands nothing in. A line that fails after the mark leaves today's `--queue-only` state; the repeat writes it. A repeat at the handed-in commit returns from `laneQueueState` before `handIn` (`intent_delivery.go:1851`), so its "waiting" answer marks too: every `work land G` that ends with the goal waiting leaves it marked.
- **The one landing slot goes.** It capped at one the claims a machine holds outside the quota and the elapsed fence. With it, a seat's second goal is refused the lane, or waits there with its clock running: the defect. Removed: the loop at `verbs.go:2249-2255`, `landingByMachine` at `validate.go:430-457`, its explanation at `intent_planning.go:1253-1272`. Pinned by `landing_test.go:78`, `:463`, `:842-854`, `TestGoalCLILedgerLandingSlot`, `TestIntentQueueOnly`. Left open: `--queue-only` on unbuilt work, now for several goals.
- **The mark covers a wait, never the goal's next work.** `admissionBreachesFor` (`internal/dispatch/admission.go:520`) goes, with its calls at `:328` and `:333`: a marked goal's own work is refused at the elapsed limit with the structured breach. `stopReasonFor` (`:500`) stays for all its callers (`:325`; `stop.go:217`, `:337`; `:442`, the machine-wide read under `FindBreachStops`), so a waiting goal is never stopped. The landing's own proof needs no exemption: with a lane it is the lane agent's, on another seat; with no lane the hand-in marks nothing, and `plain`'s proof asks no goal admission.
- **Each wait starts at its own hand-in.** A hand-in that adds a line (`plain.HandIn`) for a marked goal with an earlier line in the queue moves the mark to now. The act is `goal.LandReady` with one argument, `anew`: instead of answering `AlreadyHolds` (`verbs.go:2246-2248`) it writes the record again (`:2256-2257`). It adds no idle, so the span before stays charged and `LandReturn` credits only the latest wait. A `--queue-only` mark with no earlier line stays: one wait. The gate admitted this hand-in before the move. After it the grace clock starts at the hand-in (`landgate.go:320`), and `VerdictsOf` and the gate's word read from the new line (`review.go:526-536`, `landgate.go:208-213`): a verdict on the earlier commit says nothing now, the existing rule for a later land-ready.
- **The return.** A new act, `goal.LandReturn`: the claim holder's own, `LandReady` reversed. It lifts the `Landing` record, adds its own time minus `Landing.At` to `Claimed.IdleSeconds`, and writes the card `board.StageReturned`; with no mark it changes nothing. New, because nothing lifts the record while the claim stays; release-and-claim counts only its own gap. `work land G` runs it first whenever the goal's newest hand-in is returned, at any commit, then goes on: a new commit is handed in again. Today a return is read only at the returned commit (`landing_plain.go:48`), so a seat that pushed its fix first is never credited.
- **The span ends at the seat's act, not at `ReturnedAt`.** Lifting the record puts the goal back in the one-claim quota (`validate.go:463-481`). A seat busy with its next goal is refused, keeps the mark, and reads which goal to hand in or park first. Until then the goal is not stopped and prints as overdue. Remaining imprecision: work the seat did on the goal between the return and its act is credited, bounded by one lane wait. And a wait that ends in a landing is never credited: the next hand-in moves the mark.
- **The holder's Stop.** A marked goal below the tier, past the gate's grace, is a due landing on its holder's Stop (`LandingDue`, `landgate.go:345`), which runs `work land G --json` and prints any answer not refused or failed as `LANDED G` (`cmd/metasystem/holder_step.go:27-59`). The smallest change: `LANDED` only when the answer carries a landing or a landed queue line, else `LANDING G: SUMMARY`. The Stop taking a return is wanted: it runs under the holder's own session (`:20-22`), it is the first moment the seat can learn of the return, and it shortens the credited span. It prints once, as today's `LANDING REFUSED G`.
- **`LandingOverdue`** (`turnverdict.go:1757-1775`) needs no change: it subtracts `IdleSeconds`, and it prints, never stops.
- **One rule changes.** `budget.go:359` drops all idle when the budget starts at or after the claim: every claim never released. It becomes "after".
- **Landed.** Nothing: `goal done` lifts the record with the claim.
- **No lane.** The clock is left as it is: marking the seat's own proof needs `LandReturn` on red and conflict.

## 3. The other forms of `work land`

| Form | What happens to it |
| --- | --- |
| `--through COMMIT` | Kept on the one mechanism: the merged SHA is that commit, as the hand-in already does. |
| `--queue-only` | Kept as is. It is `goal.LandReady` (`intent_planning.go:1245-1285`) and reaches none of the deleted code. |
| `j2:J` | Deleted. It lands nothing today: refused with a lane; without one it asks for `landing.batch-root` (`landJob`, 1632-1651), whose owner is deleted. |
| `--message --staged --local` | Kept as is. It is the commit boundary (`landpath.Commit`, `intent_land_staged.go:191-213`), which the pre-commit guard expects. It publishes nothing. |
| `--message` that publishes (`--staged`, `--path`, `--chain`, `--recertification`, `--direct-fix`, `--tests`) | Wido's decision: Q1. |
| `--exception`, `--using-exception` | Wido's decision: Q2. |

## 4. Open questions

**Q1. Delete the publishing `--message` forms?** Recommended: yes. With a lane they are already refused, except a recertified chain (`intent_land_staged.go:126-134`). They are the staged driver in `landpath/land.go`, with the receipt line, the test receipts, the recertification park and the transport mirror: the "landpath receipts" of the ruling. A hand-made change then goes on a goal branch and lands with `work land G`.

**Q2. Delete `--exception` and `--using-exception`?** Recommended: not in this goal. They work with a lane today, and they are the override the commit boundary's refusals name (`internal/refusal/register.go:434-450`, `Override: CarriedLanding`). Deleting them means deciding what a person does instead.

**Q3. Which proof command do this project's seats use?** The lane's value is the VM suite, in the lane checkout's local configuration, and the local rules start a VM run only on Wido's request. Recommended: the lane's value, set once per seat. The witness below uses a stand-in command.

**Q4. How does a goal branch get onto a moved main after a conflict?** The seat ruled `metasystem work revise G --brief FILE`, reading that a follow-up rebases the goal's work onto main. Checked, it does not:

- The follow-up moves the delegate's job worktree, not the branch: it stashes the round's uncommitted changes, fast-forwards the worktree to the seat checkout's HEAD and applies the stash again (`internal/delegation/followup.go:209-222`, `:838-894`). `internal/dispatch/followup_rebase.go` only plans it.
- A unit commit goes on the branch's own tip, and a correction rewrites the unit on its old parent (`internal/goal/branch/commit.go:147-187`, `:518-623`). Nothing there carries a branch onto main.

So the branch still forks from the old main, and lines both sides changed conflict again, whatever is committed on top. The lane's return line has the same gap (`landing_plain.go:60`). The choices:

1. The seat resolves in its landing tree, as the lane agent does in its checkout: the conflicted merge stays, the seat resolves and commits it, and the next run proves that merge instead of rebuilding the tree. Cost: the goal's reads do not see the resolution; the proof does.
2. A new act that replays the branch onto main, as `amendUnit` replays a suffix. Cost: a new mechanism, the reads are taken again, and a replay that conflicts still needs a place to resolve.
3. Neither here: the row refuses, and a goal of its own decides.

Recommended: 1, the route Wido's target names. Only the conflict row's second line and its recovery witness wait on Q4; no unit waits on Q1 to Q3.

## 5. What is deleted

**D1, no question needed: what only the goal's hand route reaches.**

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

**D2, only after a yes on Q1.** The publishing branch of `runIntentLandStaged` (`runLandPath`, the gate and release wiring at 96-116, `laneRegistered`); the staged steps of `landpath/land.go`; their owners in `cmd/metasystem/landing_path.go`; `internal/landing/receiptline.go` and `synctransport.go`, whose only caller outside tests is `landing_path.go`; register rows 432-433.

**D3, only after a yes on Q2.** The rest of `landpath/land.go`, `landpath/carried.go`, `intent_exception.go`, `intent_exception_release.go`, the rest of `intent_land_release.go`, `internal/landing/carried.go` and `carried_prepare.go`, `internal/goal/branch/land.go` and `land_repository.go` (`PrepareLanding`, which the exception uses for its candidate), register rows 436-442.

**Kept, with the caller that needs it.**

- `internal/landing/plain`, `lane`, `batchowner`: the lane and section 2.
- `landpath/precommit.go`, `commit.go`, `commit_record.go`, `token.go`, `stop.go`, `owners.go`, and `landing.Observe` behind them: the pre-commit guard (`precommit_entry.go`) and the commit boundary.
- `internal/landing/receipt.go`, `testing.go`, `registers.go`, `fastforward.go`: `test run` and `internal/testrun`. So `landing.receipt-bound-min` stays.
- `branch.Push`, `branch.Sweep` and the branch reads: goal work.
- `noteLanded` and `intentLanded`: the new route feeds them.

No setting and no registered refusal code dies in D1: the register's `landpath` rows belong to the commit boundary, the staged form and the exception. After D1 the receipt, policy and budget code is still reached by the publishing `--message` forms and by `test run`. Only a yes on Q1 takes it out of landing.

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

Each lands by itself through this computer's lane. At most 300 changed lines, except U4.

**U1b.1 (ahead of U1; waits on no question). Every hand-in marks; the slot rule and the admission exemption go; the Stop's line.** Witness, stubbed: `TestWorkLandMarksTheGoalBeforeTheQueueLine` (a refused mark writes no line); `TestAWaitingRepeatMarksTheGoal`; `TestTwoGoalsOfOneMachineWaitToLand` (both past the breach limit; `FindBreachStops`, through `stopReasonFor`, names neither); `TestAMarkedGoalsNextWorkIsRefusedAtTheElapsedLimit`; `TestTheStopSaysLandingForAWaitingGoal`. Mutations: drop the call in `handIn`; restore the loop; restore `admissionBreachesFor`; print `LANDED` for every taken answer. Past 300 lines, the slot rule's removal lands first.

**U1b.2 (waits on no question). `LandReturn`, at any commit; the mark moves.** Witness: `TestLandReturnAddsTheWaitToIdle`; `TestAReturnedGoalIsStoppedOnlyAfterItsRemainingTime` (never released, waited past the breach limit: no stop at the return, a stop once its remaining time is used); `TestWorkLandTakesAReturnedGoalBack` (the mark stays beside another working claim); `TestWorkLandTakesTheReturnAtANewCommit` (then hands in again); `TestASecondHandInMovesTheMark` (idle unchanged, a return credits only the second wait, no earlier verdict stands). Mutations: lift without adding the span; restore "at or after"; drop the call; read the return only at the returned commit; keep the old `Landing.At`. Past 300 lines, the moved mark lands second, as U1b.3.

**U1. `plain`: the proof folder and `MergeOne`.** Witness: one real-git test, `TestRealGitSeatTreeMergesAndProvesFromTheInstallationsFolder`; git is the claim. With an installation outside the landing tree: a clean merge has parents main and SHA, a stand-in command's `pwd` is the proof worktree plus the installation's prefix, and a conflict returns the paths and leaves a tree the next merge can use. Mutations: restore `filepath.Rel(checkout, install)` (the `pwd` is the worktree top); drop `merge --abort` (the next merge fails).

**U2. `landing prove` on a seat with no lane.** Witness, git stubbed through the existing seams: `TestLandingProveWithNoLaneProvesTheSeatsLandingTree` (launched from the seat's installation, the result in the seat's records), the two refusals above, and `landing push` with no lane still refusing. Mutation: remove the fallback in the admission.

**U3. `work land G` with no lane: the loop, and the switch.** `landAlone` replaces the call to `landByHand` at line 1873. Witness, stubbed with per-test owners: one test per state (`TestWorkLandWithNoLane...` `StartsTheProof`, `WaitsOutALiveProof`, `SaysStillProvingAtTheBound`, `RestartsADeadProof`, `PushesAGreenTree`, `RefusesARed`, `RefusesAConflict`, `SaysLanded`, `ProvesAgainWhenMainMoved`), plus `PostsTheFirstRunsSentence`, `WritesNoReceiptOrLandingIntent` and `ReadsTheGateBeforeThePush`. `SaysLanded` reads a branch whose tip main already contains, so its unit list is empty, not stubbed. In `RestartsADeadProof` a dead proof's `running.json` makes the next `work land G` start the proof again. Mutations: take the newest result whatever its tree (main-moved pushes a stale green); push before `admitLanding`; call `prepareReceipt`; check landed after `handLandingSubject`; wait on the file alone. The verb's help and the docs that describe the hand route (`docs/working-with-agents.md`, `docs/project-rules.md`) change here. Past 300 lines, the green, red and landed rows become U3b and the switch moves there.

**U4. D1 and the `j2:J` form, a pure deletion.** Witness: U3's tests and the lane hand-in tests (`TestWorkLandHandsInToThePlainLane`, `TestWorkLandKeepsTheReadGateBeforeTheHandIn`) stay green; `go run ./cmd/devgate static` passes. Mutation: a deleted call that returns turns U3's no-receipt test red.

**Last witness: one real landing on a seat with no lane, in a bed.** The lane record is read from the shell's home directory (`intent_landing.go:185-194`), so a scratch `HOME` has no lane and this computer's lane stays registered.

1. `export HOME=$(mktemp -d)`; make a bare origin and a clone of this repository at the built commit; `metasystem landing status` there says no lane.
2. In the clone: a tier 1 goal, claimed, with one pushed commit on `goal/G` that adds a file.
3. `metasystem settings set landing.prove.command 'test -f FILE'`.
4. `metasystem work land G --delivered "..."` says proving in the background. `work land G` again waits, pushes and says landed COMMIT; the channel carries the first run's sentence. A third run says landed. Origin's main holds the merge of SHA, `results.jsonl` has one green line, and there is no `landing-intent` folder and no receipt.
5. A second goal that fails the command shows the red lines; a third that conflicts shows the conflict line.

**After the answers.** U5 is D2 (Q1 yes) and U6 is D3 (Q2 yes): pure deletions under the rule above. Q4's answer adds the conflict's recovery and its witness to U3, or to a unit after it.

## 7. Out of scope

- The 2026-10-01 hand-in frictions.
- The lane's own agent, keeper and launch (goal `old-lane-plumbing-is-deleted`), with `landing.batch-root` and `landing.batch-max-wait`.
- Receipts written by the landing (goal `land-verb-writes-the-receipt`).
- Who concludes a landed goal.

## 8. Deferred

- Thinning the commit boundary, and with it the exception forms.

## 9. Not checked

- That `laneCheck`'s reader (`batchowner.ProductionLandingLaneSeams().BatchRoot`) reads the lane record from the same home directory as `laneContext`. Step 1 of the bed run shows it.
- The test-file list for D1 and every symbol in D2 and D3: the deletion rule finds them.
- Whether `internal/testpolicy/protection.go:38-47`, which names the `landpath` path, changes after D2 or D3.
- Line numbers outside the changed parts (read at `f85f27efe`).
- For Unit 1b: the one-slot rule's origin (its purpose is read from `validate.go:393-397`); whether `work land G` admits a non-holder today (the mark refuses one); what a new ledger verb or argument needs beyond `internal/goal/recover.go:543` (the steward reads `land-ready` rows: `seat_start.go:305`, `handoff_state.go:321`); an obligation discharged inside the hold drops all idle, so that goal's returned wait counts at once.
- What the card shows for a goal at or above the tier marked after the person's word: the word then stands before the `Landing` record.
- Whether the hand route's landing test asks the goal's own admission for a `--queue-only` goal; if so, it is refused at the limit until U4.
- What the Stop's hook does while its `work land G --json` waits out a proof; the wait holds no lock.
- That the steward's silent-lane check (`internal/steward/lane_silent.go:113`) and the keeper's wake read only the registered lane's queue.
- That `steward.SweepGoalWorktrees` covers every store a release set selects.
- For Q4: whether a branch rebased by hand passes the branch reader and the leased push.
