# seat-path-lands-without-help: code map of `work land`

Working file of goal `seat-path-lands-without-help`, made by seat m1g on 2026-10-02 from a read-only inventory at main 785e68bf5. Everything here was read, nothing was run. It is evidence for the design page and the build briefs, not policy; delete it when the goal ships.

## Route choice

`runIntentLand`, `cmd/metasystem/intent_delivery.go:1512-1585`. `landGoalRoute` (1732-1805) chooses at 1745: `laneCheck` reads `~/.metasystem/host/landing-lane.json` through `batchowner` and `lane.Resolve` (`internal/landing/lane/lane.go:202-221`). Lane registered: `handIn` (1801-1803). No lane: `landByHand` (1804).

| Form | With a lane | Without a lane |
| --- | --- | --- |
| `work land G [--through C] [--delivered T]` | hand-in to the queue | hand route, `landByHand` |
| `G --queue-only` | `goal.LandReady` only, the lane is not read | same |
| `--message FILE (--staged or --path)` | refused, `intent_land_staged.go:129-134` | `landpath.Land` |
| `--message ... --staged --local` | allowed: `landpath.Commit` only, nothing pushed | same |
| `--message ... --chain J --recertification R` | allowed: full `landpath.Land` with push | same |
| `G --exception ...` / `--using-exception ID` | allowed: no lane check in `intent_exception.go` | same |
| `j2:J` | refused, `laneRegistered` at 1645 | refused at 1648-1650: it lands nothing on either route |

## Lane hand-in: checks in order

`selectRoot`, goal id, `resumeSweep` (leftover of the hand route, the one place this route can reach a lease check), `laneCheck`, `branchState`, the lane installation, `plain.Say` for `--delivered`, `laneQueueState` (a repeat at the same sha answers here and re-evaluates nothing), branch absent, `handLandingSubject` (reads, with the tier-1 waiver `goal.ReadsWaived`, `internal/goal/landgate.go:661`), `admitLanding` (`goal.Gate`), `handIn`.

Not checked on this route: receipt, test plan or test-run evidence, budget or episode admission, the lease, the claim holder, approval, author binding, land-ready, engine enrollment, delivery parity.

## No lane, `work land G`: the hand route

`landByHand`, `cmd/metasystem/intent_delivery.go:1853-1935`. It does not use `internal/landing/landpath`.

1. Candidate: `goalBranchLandPrepRun` (`cmd/metasystem/goal_branch.go:362-455`): lease and claim (`goalBranchClaimCheckWith`, 469-493), approval, then `branch.PrepareLanding` (`internal/goal/branch/land.go:579-811`): author binding (`GOAL_LAND_AUTHOR_UNBOUND`), reads (`GOAL_LAND_UNPROVEN`), land-ready required (`GOAL_LAND_PARTIAL`: "--last requires the goal queued to land"), composition in a scratch worktree on the endpoint tip (`GOAL_UNIT_REREAD`).
2. Receipt: `prepareReceipt` (2019-2042) runs a delivery-purpose `test run --tree CANDIDATE --goal G` through `landingTestReceiptTo` (`cmd/metasystem/landing_verbs.go:175-267`). `testrun.Prepare` (`internal/testrun/prepare.go:253-485`) demands the goal's risk and budget episode, an engine re-arm, the landing ref, that the candidate equal the checkout's real index (340-342), an enrolled policy engine, a selected plan, and parity with the working tree (449-453, text at 831). `admitProofLaunch` (`cmd/metasystem/proof_run.go:850-1290`) then charges the goal's budget (`BUDGET_REFUSED` at 2261).
3. `landPrep`: receipt match, `GOAL_LAND_RETRY`, pushes `landing/<goal>` to origin.
4. Gate again, `recordReleaseSet`, `landPush` (`branch.LandPush`, `internal/goal/branch/publish.go:69-138`, `GOAL_LAND_TRUNK_MOVED`), sweep of the merged branch, `noteLanded` (channel post and the `landed under` ledger line).

Read, not run: the candidate is composed in a scratch worktree, yet step 2 demands that it equal the seat checkout's real index and match its working tree, and no bypass for the hand route was found. Every `work land` test fakes this proof (`cmd/metasystem/intent_delivery_owner_test.go:186-190`), so no test exercises the hand route end to end.

## No lane, `--message`: the staged route

`runIntentLandStaged` (`cmd/metasystem/intent_land_staged.go:42-159`) into `landpath.Land` (`internal/landing/landpath/land.go:70`): gate, lane refusal, lease, coordinator fence, staging checks, the receipt line (`landing.ObserveReceiptLine`), a supplied test receipt, the commit boundary (`landpath.Commit`, `commit.go:83`, with `owners.Verify` and the class decision of `landing.Observe`), then rebase (`landing.Advance`), held check, up to three pushes each behind `gateBeforePush`, transport sync.

Dead: `LandRequest.CommitOnly` and `CommitRequest.LaneJoin` (`landpath/land.go:47-49, 484-487`; `commit.go:43-45, 312-331`) are set by nothing in `cmd/`.

## The exception route

`landException` (`cmd/metasystem/intent_exception.go:39-309`): gate, composition through `landCandidate`, the carry word, `landing.StageCarriedCandidate`, then `landpath.Land` with `Carried`. No lane check.

## What still needs `landpath` with a lane

The pre-commit guard refuses an agent commit outside the commit boundary (`internal/landing/landpath/precommit.go:130-134`, fence rule 279-325) and names `work land --message FILE --staged` as the way. `precommit.go` (325 lines), `token.go` (22), `commit.go` and `commit_record.go` (993) serve the hook and `--local`; `carried.go` (484) the exception route. About 1,440 lines (`land.go`, `owners.go`, `stop.go`) are staged-route only.

## The lane's mechanism

`landing prove`, `landing push`, `landing status`, `landing return` all pass `admitLane` and refuse on a seat with no lane: "no landing lane is registered on this computer; nothing was done" (`cmd/metasystem/intent_landing.go:205-208`). With a lane they act on the registered lane checkout, never the caller's directory. The only callers of `plain.Start`, `plain.Run` and `plain.Push` are those verbs; the three functions take plain `(install, checkout)` arguments and import nothing of the lane.

`landing.prove.command` is read from the lane installation's `metasystem.conf` chain (`cmd/metasystem/intent_landing_prove.go:113`). On this computer it is set only in the lane checkout's uncommitted local configuration. Seat m1g has no value.

## The elapsed clock

Computed in `internal/dispatch/budget.go:346-368`: elapsed is now minus the episode start minus `Claimed.IdleSeconds`. The stop fires at the breach limit (the box plus 50 percent grace by default, so 1h30m for a tier-1 box); admission closes at the plain limit. It is fired by the steward tick (`internal/steward/tick.go:251`, `dispatch.FindBreachStops`, `internal/dispatch/stop.go:387-456`).

Only `IdleSeconds` is subtracted: the gap of an own-pair release or park followed by the same pair's claim (`internal/goal/verbs.go:531-552`). A claim with a `Landing` record (`goal.LandReady`, `internal/goal/verbs.go:2206-2262`) keeps counting, but its elapsed stop is not enforced (`internal/dispatch/admission.go:495-530`) and it leaves the one-claim quota. The ledger allows one such claim per machine: `LandReady` refuses a second (`verbs.go:2249-2255`) and tree validation refuses two (`internal/goal/validate.go:428-456`). The record is lifted only with the claim (`verbs.go:465`, `609`; `abandon.go:332`).

`handIn` writes only the lane's `queue.jsonl`; `plain.Return` only appends to it. So time in the queue counts, and time after a return counts.

## Sizes (production lines; test lines in brackets)

| Area | Lines | Reached only without a lane? |
| --- | --- | --- |
| `internal/landing/landpath` | 3,266 [4,596] | No, see above. About 1,440 are staged-route only. |
| `internal/landing`: observe, held, park, receiptline, tierone, advance, drift, attested, synctransport | 3,918 | Yes, through the landpath owners. |
| `internal/landing`: carried, carried_prepare | 988 | Exception route. |
| `internal/landing`: receipt, testing | 1,140 | No: `test run` writes receipts through them. |
| `internal/goal/branch`: land, land_repository, publish, red, landing_event | 1,403 | Hand route, except that the exception route reuses `PrepareLanding`. |
| `cmd/metasystem`: landing_path, landing_verbs, intent_land_staged, intent_land_release, landing_stop | 1,520 | Yes. |
| `cmd/metasystem`: intent_exception and its release | 568 | Exception route. |
| hand-route parts of `intent_delivery.go` and `goal_branch.go` | about 340 | Yes. |
| `internal/landing/batch`, `batchowner` | 1,031 [587] | No: lane resolution, board, lane status. |
| `testrun`, `testpolicy`, `proofrun`, `proof_run.go`, `test.go` | large | Shared with `metasystem test run`. |

Rough total only the no-lane routes use: about 9,000 production lines in about 25 files, about 10,200 with the exception route.

## Tests by route (all under `cmd/metasystem/` unless noted)

- Lane hand-in: `landing_plain_handin_test.go`, `intent_delivery_owner_test.go` (`TestWorkLandHandsInOverRealGit`), `landed_notice_test.go`, `landing_lane_test.go`, `landing_lane_refuses_land_test.go`; lane side `landing_plain_verbs_test.go`, `landing_plain_keeper_test.go`, `landing_run_test.go`, `internal/landing/plain/`.
- Hand route: `intent_delivery_test.go` (`TestIntentLandRouteEvidence`, `TestIntentLandRecovery`, `TestIntentLandByHandWritesLandingThenLanded`, `TestWorkLandTierOneUnitsNeedNoRead`), `intent_delivery_owner_test.go`, `intent_land_release_test.go`, `intent_landing_gate_test.go`, `holder_step_test.go`, `intent_queue_wait_test.go`, `intent_idempotency_work_test.go`.
- `--message`: `message_landing_test.go`, `landing_verbs_test.go`, `landing_path_scope_test.go`, `landing_base_judge_test.go`, `landing_guard_helm_test.go`, `landing_guard_owners_test.go`, `internal/landing/landpath/*_test.go`.
- Exception: `intent_carried_test.go`, `intent_exception_release_test.go`, `intent_operations_test.go`, `internal/landing/landpath/port_carried_test.go`, `port_abandon_test.go`, `internal/goal/carry_lifecycle_test.go`.
