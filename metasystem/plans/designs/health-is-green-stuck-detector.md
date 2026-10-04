# Design for health-is-green-when-the-seat-is-healthy

- Kind: design
- Id: 01M43SSNG0MTG29PNXTDW7A3VX
- Status: accepted
- Goals: health-is-green-when-the-seat-is-healthy
- Critique: closed at round 2 on 0 material findings (Codex Astra, chain design-critic-0416d2dda92071e470e82ccc)

Accepted 2026-10-04 18:44 CEST by m1e in Wido's word (peer message d-0c874235a99bdf97cfafc1bb26), after Astra rounds 2/0; Wido may overturn.

Unit 2, the stuck detector. Revision 2, 2026-10-04; lines at e277d7da8 under `metasystem/`.

## 1. Scope

- Changes: every seat's steward reads the host's unit runs and launches each tick, opening one per-work alert per stuck unit of its claimed goals, closed when the condition clears.
- Changes: four settings carry the limits (build 45, proof 30, read 35 minutes; 3 rounds).
- Changes: `/api/board` stamps a stuck unit on its goal's card for every seat of this host; the Fleet page shows it there and in Needs you.
- Leaves alone: health roles and episode, the board's stall rule and card writers, the round limit, the lane's pattern pass and `patterns.json`, unit 1's page.
- Leaves alone: the coordinator's script (outside the repository).

## 2. What is true today

Facts 1 to 7 hold at their lines. Additions:

- Both stores are host-wide (launch root `internal/launch/record.go:128-134`, unit root `internal/launch/unit_run.go:837-843`); any seat reads all runs.
- `UpdatePatterns` writes `patterns.json` only when the step returns state (`internal/steward/pattern_episode.go:274-282`).
- `metasystem work stop` takes `j1:ID` for a launch (`cmd/metasystem/intent_references.go:53-58`).
- `steward` imports `launch` (`internal/steward/disk_role.go`); `board` must not (`internal/board/card.go:9-11`); `httpd` may.
- (revision 2) `metasystem alert clear` closes the episode and suppresses its work (`internal/steward/pattern_episode.go:386-395`); suppressed work refuses openings (`:211-213`) until a Clear observation releases it (`:335-340`); `AlertEpisodes` lists cleared episodes (`internal/steward/alert_episode.go:207-214`).
- (revision 2) Reads run in parallel when the plan declares no shared read outputs (`internal/launch/unit_run.go:432-436`, `internal/launch/read_sequence.go:234-240`, `:268-273`), so the last launched step is not always the running one.

## 3. The design

**Reader** (`internal/launch/unit_stuck.go`, new): `UnitStandings(unitRoot, store, limits, now) ([]UnitStanding, error)` lists `unitRoot/*/run.json`, keeps `state == "running"`, counts rounds as `len(Rounds)`, and (revision 2) reads every launch of the newest round. A launch runs while `state == running`; `kind` names its limit, age is `now − startedAt`. The standing is the running launch furthest over its limit (largest age minus limit), or none. Stuck: such a standing, or rounds at the round limit. A missing launch is an unstarted step; an unparsable run or launch sets `Unreadable`; an unlistable root is the error.

**Pass** (`internal/steward/stuck_units.go`, new): `StuckUnits{UnitRoot, Store, Claims, Deliver}.Run(repoRoot, now)`, a `TickConfig.StuckUnits` hook beside `runPatterns` (`internal/steward/tick.go:234`, `:292`), wired at `cmd/metasystem/steward_verbs.go:318`. Limits: `boundedConfig` (`internal/steward/health.go:1802`). `Claims` returns the machine name and its ledger-claimed goals, as the delivery role reads them (`internal/steward/delivery.go:50-65`); tests stub it. Seat filter (D1): the run's goal is among those claims. Exact: every run names its goal, one holder claims a goal, the ledger owns the claim; worktree paths are convention, so `Worktree` is evidence only.

**Alert** (D3): `UpdatePatterns`, owner `pattern:stuck-unit`, work `GOAL/UNIT`, a step returning nil state. Per cycle: a Finding per stuck standing (Since the launch's `startedAt`; evidence `run.json`, now, `build r2-s1 52 min, limit 45` or `rounds 3 of 4, limit 3`); (revision 2) a Clear for every stuck-unit work with an open or suppressed episode (both read through `AlertEpisodes`) whose run is readable and not stuck (step ended, run awaiting judgement or gone, goal left this machine's claims), so the first clean reading releases a dismissal and a later stall alerts again; Unknown for an unreadable run or launch, and for every such work when store, ledger or machine name is unreadable. ClearTicks is 1: one clean read is a complete negative; a person's stop closes the alert next tick. Message, fixed at opening (later ages go to evidence): "Seat m1l has run the build step of unit 2 of goal G for 52 minutes (limit 45, round 2 of 4): stop it with `metasystem work stop j1:RUN-r2-s1`, cut the unit, or land with notes (`metasystem work land --message`)." The round rule: "is on round 3 of unit 2 of goal G (limit 3)".

**Settings** (D4): `steward.stuck.build-min` 45, `steward.stuck.proof-min` 30, `steward.stuck.read-min` 35, `steward.stuck.rounds` 3; not proof inputs; positive integers, validated at `internal/config/validate.go:589-595`.

**Fleet page** (D5): `GoalView` gains `stuck` (`internal/board/view.go:105-114`): `{step, kind, launch string; minutes, rounds, limit int}`. `BoardSource` gains `Stuck func(now) ([]launch.UnitStanding, error)`, filled by `HostBoardSource` (`internal/landing/batchowner/pipeline.go:106-114`) with the serving seat's limits; `boardView` (`internal/ui/httpd/board.go:134-172`) stamps the matching goal's card on the seat the ledger's claim names (`claimsOf`). No ledger or failed read: no stamp, reason joins `Unreadable`. The browser (`api.ts:400-409`, `panel.ts:472-494`, `:803-813`) appends "build step 52 min" or "round 3 of 3" to the card and a Needs-you item with the `work stop j1:` command.

**Failure path**: nothing is raised or cleared on an unreadable store, record, ledger or machine name.

**Tests** (D6: Git stubbed; stores under `t.TempDir()` through `Store.Root` and `UnitRoot`):

- Reader rule: `internal/launch/unit_stuck_test.go` `TestUnitStandingsReadRunningRuns`: launches at 46/29/35 minutes, 3 rounds, awaiting-judgement, missing and corrupt launch. (revision 2) `TestUnitStandingsNamesTheRunningParallelRead`: two parallel reads, earlier running 40 minutes, later completed: stuck, naming the earlier.
- Pass: `internal/steward/stuck_units_test.go` `TestStuckUnitsOpensOneEpisodePerUnit`, `TestStuckUnitsClearsWhenTheStepEnds`, `TestStuckUnitsClearsWhenTheGoalLeavesTheSeat`, `TestStuckUnitsSkipsUnclaimedGoals`, `TestStuckUnitsHoldsUnknownOnUnreadableRecords`, `TestStuckUnitsLimitsComeFromSettings`; (revision 2) `TestStuckUnitsAlertsAgainAfterADismissal`: open, `ClearAlert`, clean reading, new stall: new episode. Fixture: temp roots, claims stub, Deliver capture.
- Tick hook: `internal/steward/pattern_tick_test.go` `TestTickRunsStuckUnitsAfterHealth`.
- Settings valid: `internal/config/validate_test.go` `TestStuckLimitsMustBePositive`.
- Board stamp: `internal/ui/httpd/board_test.go` `TestBoardRouteStampsStuckUnits`: Stuck stub, two seats, one claim.
- Page: `panel.test.ts` "a stuck unit is a Needs-you item with its stop command", "the card says the step and age".

## 4. Moved effects

No owner moves.

## 5. Deferred

- Automatic stop: on the episode's work and `work stop j1:`.
- Per-kind stall bounds on the board: on `BoardSource.Stall`.
- Other seats' alerts on this seat's Fleet page: on the `stuck` stamp.
- Retiring `scratchpad/seat-watch.py`: outside the repository.

## 6. Open questions for Wido

1. Round rule on `running` runs only (D2): a run awaiting judgement at the limit is not stuck. Recommended: keep; judgement is the board's.
2. Retire the coordinator's script once stewards alert? Recommended: after a day side by side.
3. The Fleet stamp uses the serving seat's limits; each alert its own seat's. Recommended: accept.

## 7. Estimate

| Unit | Production | Tests |
|---|---|---|
| 2, the stuck detector | ~440 (reader 130, pass 150, wiring 25, settings 15, board/httpd 60, browser 50; bundle regenerated) | ~510 |

## Dispositions (critique design-critic-0416d2dda92071e470e82ccc)

Written by metasystem design review when critique design-critic-0416d2dda92071e470e82ccc closed: every answered round's decisions, as the author made them.

| Round | Finding id | Finding | Disposition | Reasoning and evidence | Amendment |
| --- | --- | --- | --- | --- | --- |
| 1 | STUCK-SUPPRESSION-RESET | A dismissed alert can permanently silence later stalls of the same unit. The design emits clean observations only for open episodes, but dismissal closes and suppresses an episode. Recovery therefore never releases its suppression. The implementer must include suppressed work in recovery observations. Step 1 does not work without this: dismissal, recovery and a later stall produce no new alert. | accepted | Right: `ClearAlert` suppresses the work (`internal/steward/pattern_episode.go:386-395`), a suppressed work refuses openings (`internal/steward/pattern_episode.go:211-213`) and only a Clear observation releases it (`internal/steward/pattern_episode.go:335-340`); revision 1 sent Clear only for open episodes. | Revision 2 (section 3, Alert): a Clear for every stuck-unit work with an open or suppressed episode whose run is readable and not stuck; new test `TestStuckUnitsAlertsAgainAfterADismissal`. |
| 1 | STUCK-PARALLEL-READ | Selecting only the last launch can hide another review that is still running. Existing unit runs support parallel reviews: the last-listed review can finish while an earlier review exceeds 35 minutes. The reader must evaluate active launches before selecting one standing and stop target. Step 1 does not work without this: it silently misses a supported running-unit shape and can clear a valid alert. | accepted | Right: reads run in parallel when the plan declares no shared read outputs (`internal/launch/unit_run.go:432-436`, `internal/launch/read_sequence.go:234-240`), so the last launched step need not be the running one. | Revision 2 (section 3, Reader): every launch of the newest round is read; the standing is the running launch furthest over its limit; new test `TestUnitStandingsNamesTheRunningParallelRead`. |
