# Design for seat-path-lands-without-help

- Kind: design
- Id: 01M3Z6EXJH4CCXWDNZA9DWB8X2
- Status: draft
- Goals: seat-path-lands-without-help

Revision 2: adds Unit 1b from goal revision 23 (section 2b; U1b in section 6). Author: seat m1g (Fable 5.1). Cites were read at `f85f27efe`; paths are under `metasystem/`.

Wido, 2026-10-01:

> TARGET: one landing mechanism. A seat without a lane lands exactly as the lane agent does for a batch of one [...] on its own landing tree. [...] Then delete the old seat self-land plumbing.

Threat model: our own agents and operators make mistakes and crash. Nobody attacks. What defends against anything else is removed, not fixed.

## 1. What exists

- `work land G` (`landGoalRoute`, `cmd/metasystem/intent_delivery.go:1732-1805`) reads the goal branch, checks the reads (`handLandingSubject`) and the gate (`admitLanding`). With a lane it hands in (`handIn`). With no lane it runs `landByHand` (1853-1935): candidate, landing test receipt, prepared landing, push, sweep, `landed.json`, board card.
- `internal/landing/plain` takes its folders as parameters. `Start` and `Run` prove `checkout`'s HEAD in a fresh detached worktree and record the result. `Push` puts HEAD on main only when that exact tree is green and HEAD contains origin's main.
- Two things tie `plain` to a lane today:
  1. `Start` launches `metasystem landing prove --wait --attempt ID`, and that verb admits only a registered lane (`admitLane`, `intent_landing_prove.go:52-65`).
  2. `proveInWorktree` picks the command's folder with `filepath.Rel(checkout, install)` (`prove.go:331-334`), which works only when the installation is inside the checkout.

## 2. What changes: a seat with no lane is its own lane, for a batch of one

### Folders

- **Records:** the seat's own installation, `<seat install>/artifacts/agents/landing/` (`plain.Dir`): `results.jsonl`, `running.json`, `pushes.jsonl`, `proofs/`, `proof-trees/`. The files a lane keeps, without a queue.
- **Landing tree:** `<seat install>/artifacts/agents/landing/tree`, a detached git worktree of the seat's repository (`artifacts/` is ignored). Every run rebuilds it, so a crash or a stray file in it breaks nothing.

### Who merges

`work land G` does, as the lane agent's loop step 3 (`skills/landing-agent/SKILL.md`): in the landing tree, `checkout --detach` origin's main, then `merge --no-ff SHA`. SHA is what `handLandingSubject` returns: the branch tip, or the `--through` commit.

### The loop

With no lane, after today's reads and gate, every run of `work land G` does this in order:

1. **Landed.** Main contains SHA (`plain.ContainedIn`): say so and offer `goal done`.
2. **Proving.** `running.json` names a live proof: say so and touch nothing.
3. **Merge** SHA on the latest main in the landing tree. On a conflict: `merge --abort`, refuse.
4. **Read the newest result for the merged tree** (`plain.ResultFor`):
   - none, or the last proof died: `plain.Start`; say "proving in the background".
   - green: `plain.Push`. Main moves; `noteLanded` posts the `--delivered` sentence and writes the landed line, as today.
   - red: refuse.

"Main moved" needs no case of its own. A new main gives a new merged tree, that tree has no result, so step 4 proves it. If main moves between this run's fetch and its push, `plain.Push` refuses (not fast-forward) and the next run merges on the new main. The merge commit differs per run but the tree does not, and the rail is per tree, as on the lane.

The gate is read at the start of every run, so the run that pushes has just read it.

`work land G` holds `tree.lock` in the records folder (the existing `lock.File`) from the merge to the start or the push, so two sessions on one seat take turns. A seat proves one tree at a time: a second goal's `work land` during a proof reads "proving" and waits.

### What a repeat says

| State | Line 1 | Line 2 |
| --- | --- | --- |
| proving | goal G at SHA merged on main M is being proven since T | `metasystem work wait --path RECORDS/running.json --until absent --timeout 4m`, then this command |
| green | landed COMMIT on main; goal G stays open until done | `metasystem goal done G --reason TEXT` |
| landed | goal G at SHA is on main; it stays open until done | the same |
| main moved | goal G at SHA merged on the new main M; proving in the background | the same wait |
| red | goal G at SHA merged on main M proved red; nothing was pushed; the log is PATH | `metasystem work land G`: after the fix is pushed to goal/G. When the red is not this goal's: `metasystem landing prove`, then this command |
| conflict | goal G at SHA does not merge on main M: both changed PATHS; nothing was proven | `metasystem work land G`: after origin/main is merged into goal/G and pushed |

**How the seat learns the proof ended.** `work wait --path ... --until absent` exists (`intent_work.go:243`), and `plain.Run` removes `running.json` when it writes the result. A timeout prints the command that continues the wait. A proof that died leaves `running.json`; the next `work land G` sees the dead process and starts the proof again (`Start`, `prove.go:166-172`).

**One difference from the lane, on purpose.** The lane agent may fix a small conflict in the lane checkout. A seat fixes it on its goal branch, where its reads see the fix.

### The surface

`work land G` is the seat's surface. One lane verb also runs on a seat: `landing prove`. It has to, because the detached proof *is* `landing prove --wait --attempt ID`. So `landing prove` admits "the registered lane, else this seat's landing tree", and the same verb proves a red tree again. No new flag.

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

## 2b. The lane wait stops no clock (Unit 1b)

Today the clock runs through the lane wait.

- **The hand-in marks the goal.** `handIn` (`cmd/metasystem/landing_plain.go:69`) first calls `inv.landReady` (`goal.LandReady`), then writes the line. Already marked (as after `--queue-only`) counts as marked. A refused mark hands nothing in. A line that fails after the mark leaves today's `--queue-only` state; the repeat writes it.
- **The one landing slot goes.** It capped at one the claims a machine holds outside the quota and the elapsed fence. With it, a seat's second goal is refused the lane, or waits there with its clock running: the defect. U4 changes none of this. Removed: the loop at `verbs.go:2249-2255`, `landingByMachine` at `validate.go:430-457`, its explanation at `intent_planning.go:1253-1272`. Pinned by `landing_test.go:78`, `:463`, `:842-854`, `TestGoalCLILedgerLandingSlot`, `TestIntentQueueOnly`. Left open: `--queue-only` on unbuilt work, now for several goals.
- **The return.** A new act, `goal.LandReturn`: the claim holder's own, `LandReady` reversed. It lifts the `Landing` record, adds its own time minus `Landing.At` to `Claimed.IdleSeconds`, and writes the card `board.StageReturned`. New, because nothing lifts the record while the claim stays; release-and-claim counts only its own gap. `work land G` runs it when `laneQueueState` reads "returned"; the lane agent is another seat.
- **The span ends at the seat's act, not at `ReturnedAt`.** Lifting the record puts the goal back in the one-claim quota (`validate.go:463-481`). A seat busy with its next goal is refused, keeps the mark, and reads which goal to hand in or park first. Until then the goal is not stopped and prints as overdue. Charging that time would stop a goal for a wait again.
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

## 4. Open questions for Wido

**Q1. Delete the publishing `--message` forms?** Recommended: yes. With a lane they are already refused, except a recertified chain (`intent_land_staged.go:126-134`). They are the staged driver in `landpath/land.go`, with the receipt line, the test receipts, the recertification park and the transport mirror: the "landpath receipts" of the ruling. A hand-made change then goes on a goal branch and lands with `work land G`.

**Q2. Delete `--exception` and `--using-exception`?** Recommended: not in this goal. They work with a lane today, and they are the override the commit boundary's refusals name (`internal/refusal/register.go:434-450`, `Override: CarriedLanding`). Deleting them means deciding what a person does instead.

**Q3. Which proof command do this project's seats use?** The lane's value is the VM suite, in the lane checkout's local configuration, and the local rules start a VM run only on Wido's request. Recommended: the lane's value, set once per seat. The witness below uses a stand-in command.

Units 1 to 4 and the last witness wait on none of these.

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
- **Sweep, release set and board card at landing:** cleanup and display, not a defence. As on the lane route, the branch stays until the goal is concluded.

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

## 6. Units, in landing order

Each lands by itself through this computer's lane. At most 300 changed lines, except U4.

**U1b.1 (ahead of U1; waits on no question). The hand-in marks the goal; the slot rule goes.** Witness, stubbed: `TestWorkLandMarksTheGoalBeforeTheQueueLine` (a refused mark writes no line) and `TestTwoGoalsOfOneMachineWaitToLand` (both past the breach limit; `FindBreachStops` names neither). Mutations: drop the call in `handIn`; restore the loop.

**U1b.2. `LandReturn`.** Witness: `TestLandReturnAddsTheWaitToIdle`; `TestAReturnedGoalIsStoppedOnlyAfterItsRemainingTime` (never released, waited past the breach limit: no stop at the return, a stop once its remaining time is used); `TestWorkLandTakesAReturnedGoalBack` (the mark stays beside another working claim). Mutations: lift without adding the span; restore "at or after"; drop the call.

**U1. `plain`: the proof folder and `MergeOne`.** Witness: one real-git test, `TestRealGitSeatTreeMergesAndProvesFromTheInstallationsFolder`; git is the claim. With an installation outside the landing tree: a clean merge has parents main and SHA, a stand-in command's `pwd` is the proof worktree plus the installation's prefix, and a conflict returns the paths and leaves a tree the next merge can use. Mutations: restore `filepath.Rel(checkout, install)` (the `pwd` is the worktree top); drop `merge --abort` (the next merge fails).

**U2. `landing prove` on a seat with no lane.** Witness, git stubbed through the existing seams: `TestLandingProveWithNoLaneProvesTheSeatsLandingTree` (launched from the seat's installation, the result in the seat's records), the two refusals above, and `landing push` with no lane still refusing. Mutation: remove the fallback in the admission.

**U3. `work land G` with no lane: the loop, and the switch.** `landAlone` replaces the call to `landByHand` at line 1804. Witness, stubbed with per-test owners: one test per row of the table (`TestWorkLandWithNoLane...` `StartsTheProof`, `SaysProving`, `PushesAGreenTree`, `RefusesARed`, `RefusesAConflict`, `SaysLanded`, `ProvesAgainWhenMainMoved`), plus `WritesNoReceiptOrLandingIntent` and `ReadsTheGateBeforeThePush`. Mutations: take the newest result whatever its tree (main-moved pushes a stale green); push before `admitLanding`; call `prepareReceipt`. The verb's help and the docs that describe the hand route (`docs/working-with-agents.md`, `docs/project-rules.md`) change here. Past 300 lines, the green, red and landed rows become U3b and the switch moves there.

**U4. D1 and the `j2:J` form, a pure deletion.** Witness: U3's tests and the lane hand-in tests (`TestWorkLandHandsInToThePlainLane`, `TestWorkLandKeepsTheReadGateBeforeTheHandIn`) stay green; `go run ./cmd/devgate static` passes. Mutation: a deleted call that returns turns U3's no-receipt test red.

**Last witness: one real landing on a seat with no lane, in a bed.** The lane record is read from the shell's home directory (`intent_landing.go:185-194`), so a scratch `HOME` has no lane and this computer's lane stays registered.

1. `export HOME=$(mktemp -d)`; make a bare origin and a clone of this repository at the built commit; `metasystem landing status` there says no lane.
2. In the clone: a tier 1 goal, claimed, with one pushed commit on `goal/G` that adds a file.
3. `metasystem settings set landing.prove.command 'test -f FILE'`.
4. `metasystem work land G --delivered "..."` says proving. Run the wait it prints. `work land G` again says landed. Origin's main holds the merge of SHA, `results.jsonl` has one green line, and there is no `landing-intent` folder and no receipt.
5. A second goal that fails the command shows the red lines; a third that conflicts shows the conflict lines.

**After the answers.** U5 is D2 (Q1 yes) and U6 is D3 (Q2 yes): pure deletions under the rule above.

## 7. Out of scope

- The 2026-10-01 hand-in frictions.
- The lane's own agent, keeper and launch (goal `old-lane-plumbing-is-deleted`), with `landing.batch-root` and `landing.batch-max-wait`.
- Receipts written by the landing (goal `land-verb-writes-the-receipt`).
- Who concludes a landed goal.

## 8. Deferred

- Thinning the commit boundary, and with it the exception forms.

## 9. Not checked

- That `laneCheck`'s reader (`batchowner.ProductionLandingLaneSeams().BatchRoot`) reads the lane record from the same home directory as `laneContext`. Step 1 of the bed run shows it.
- Which goal verbs run the branch sweep (`goalsync_mutations.go:1846-1858`).
- The test-file list for D1 and every symbol in D2 and D3: the deletion rule finds them.
- Whether `internal/testpolicy/protection.go:38-47`, which names the `landpath` path, changes after D2 or D3.
- For Unit 1b: the one-slot rule's origin (its purpose is read from `validate.go:393-397`); whether `work land G` admits a non-holder today (the mark refuses one); what a new ledger verb needs beyond `internal/goal/recover.go:543` (the steward reads `land-ready` rows: `seat_start.go:305`, `handoff_state.go:321`); an obligation discharged inside the hold drops all idle, so that goal's returned wait counts at once.
