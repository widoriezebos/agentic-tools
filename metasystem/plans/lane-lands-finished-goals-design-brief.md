# Design brief: lane-lands-finished-goals (landing redesign 1a)

Working Mode: Design
Goal: lane-lands-finished-goals. Tier 3, top priority (Wido 2026-10-03: lane goals generic, top priority, tier 3). Coordinated by m1e in Wido's word.

Read first, in this order: `plans/designs/machinery-mechanisms.md` (accepted; mechanisms 2, 3, 4, 5, 6, 9, 12 are consumed here), `plans/machinery-open-findings-plan-2026-10-06.md` (findings 1, 2, 3, 12 and the stop section), and the code sites cited below. Every premise in this brief was read in the code on main 9eb87cd15; cite the site you build against, and if a site has moved, say so in the design.

## Intent

The landing lane lands finished goals only, by merging, and classifies every red before it returns anything. Wido's charter (2026-10-05): "Landing is responsible for making sure the goal's test suite is green in the most efficient way, and can fix merge conflicts so it can do that. Landing exists only because the test suite is expensive and it can batch goal tests. No goal slice should ever end up there." And: "You only integrate when you are done with a goal."

## What is true today (read, with sites)

1. `work land` (`cmd/metasystem/intent_delivery.go:1659` runIntentLand, `:1899` landGoalRoute) refuses a hand-in only when a committed unit lacks a clean read (`handLandingSubject :2217`: "unit X has no clean read"). Nothing compares the branch's units with the accepted design's Units table: `launch.DeclaredUnits` (`internal/launch/admit.go:457`) is read only at build time (`cmd/metasystem/intent_work.go:1057`, `:2136`). A goal with three of eight units built hands in three units.
2. Eight sites print `work land` as the next step after a published unit read: `cmd/metasystem/intent_selection.go:281`, `:285`, `:358`, `:627`; `cmd/metasystem/intent_unit_review.go:399`; `cmd/metasystem/intent_manual_submit.go:339`, `:363`; `cmd/metasystem/intent_delivery.go:1894` (deliveredHint); plus the prose at `cmd/metasystem/landing_plain.go:67`. The night of 10-04/05 followed them: nine pushes, twenty units, zero goals.
3. Before every hand-in the seat rebases its branch (`cmd/metasystem/intent_delivery.go:1962` rebaseGoal, `internal/goal/branch/rebase.go:90` rebaseWith, `git rebase --reapply-cherry-picks` in a scratch tree at `:385`), rewriting unit commits; reads are carried by `carryReviewsWith :263` and voided for changed units. `landRebaseSkip` (`cmd/metasystem/landing_rebase.go:9`) skips it only for `--through`, a waiting hand-in, no worktree, or a standing person's word.
4. The lane itself already merges: the landing agent (`skills/landing-agent/SKILL.md`, "The loop" step 3) runs `git merge --no-ff SHA` per waiting line on a detached origin/main, proves once with `landing.prove.command` (`cmd/metasystem/intent_landing_prove.go:25`, `:119`; set per lane installation, not committed), pushes with `landing push` (`cmd/metasystem/intent_landing_push.go:147`, `plain.PushChecked`, force-with-lease), returns with `landing return G --reason TEXT` (`cmd/metasystem/intent_landing_return.go:36`, free text). On a red it may repeat once, then "find the culprit by proving smaller merges" (SKILL case 3), by hand of the agent.
5. Conflicts: `landing resolve` (`cmd/metasystem/intent_landing_resolve.go:23`, `internal/landing/plain/resolve.go:70`) regenerates generated paths and returns any other conflict with a `Conflict{main, paths[{path, class, resolution}]}` record; the seat resolves it in its rebase through the resolve round (`internal/goal/branch/rebase_conflict.go:46`, `resolveRebaseRound intent_work_rebase.go:139`).
6. Return reasons are free text; the week's 45 returns were 29 red proofs (several "main itself is red" and "red on latest main merged with this branch") and 16 conflicts. A return writes no board stage (`board.StageReturned` is never written on a lane return; `cmd/metasystem/landing_plain.go:147` writes `joined` only).
7. The trunk-red register exists (`plans/goals/trunk-red.json`, `internal/goal/trunkred.go:21`; classes trunk-red, pending-flake, known-flake, hang, quality; `RecordTrunkRed :538`, `ClearTrunkRed :744` with no caller, `OwnTrunkRed :841`, `CloseTrunkRed :935`). `incident list/claim/close` (`cmd/metasystem/intent_planning.go:424-446`, `:2168-2263`) read and own it. The lane writes only pending-flake entries (`cmd/metasystem/intent_landing_flake.go:100`). The hand route's red path (`internal/goal/branch/red.go:209`) wants a recorder that cmd never supplies. The lane's wake has a `full-due` reason (`internal/landing/plain/wake.go:19-23`) and SKILL case 8 proves origin/main when nothing waits, but a red there is only asked about, never registered. The week's eight reds on main registered nothing.
8. The claim quota (`internal/goal/verbs.go:1390` claimQuotaRefusal, `internal/goal/validate.go:399-490`) counts a goal as holding the machine's slot unless it has `HandedOver`, `Landing != nil` (the land-ready slot from `work land --queue-only`, `cmd/metasystem/intent_planning.go:1322`), or a fence. A plain lane hand-in writes nothing to the ledger, so a seat whose goal waits in the lane cannot claim its next goal. The parked branch `origin/parked/m04-handin-leaves-quota-av` tried to fix this with ledger writes per hand-in (`RecordHandIn`), which main's landing tests forbid ("a hand-in never moves main").
9. The lane's records: `queue.jsonl` (`internal/landing/plain/queue.go:51` Line, `:76` Entry with states waiting, landed, returned, superseded), `results.jsonl` (`internal/landing/plain/prove.go:48` Result), `pushes.jsonl`, running.json, `design-gate.jsonl`; landed is derived by ancestry (`internal/landing/plain/queue.go:346`).
10. The lane's lease (`internal/landing/lane/agent.go`, keeper `Run :144`, `barrenHold :338`) and the landing agent's hold are the lane's stop today: a hold after `barrenLimit` barren launches, cleared by a person's `landing run`.

## What the design must deliver (the units; the design may re-cut them, not drop their intent)

The mechanisms page gives the shapes; name the mechanism you consume and build its smallest form here.

| Unit | Intent | Lines |
| --- | --- | ---: |
| U1 whole-goal admission | `work land` refuses a goal whose accepted design's Units table has a unit with no commit on the branch or no clean read, naming them; tier-1 (ReadsWaived) needs every unit built. The eight next-step sites name the next unit of the design (the first without a clean read) or `goal done`, never `work land`. One hand-in per goal; `--through` lands no partial goal. | 120 |
| U2 no rebase at hand-in | `work land` never rebases; `landRebaseSkip` becomes the rule; `work rebase` stays a verb a person or driver names; a returned conflict is resolved on the branch by the existing resolve round through `work rebase`, which carries reads. The lane's `--no-ff` merge is the integration (mechanism 9: the lane's lease is held during the merge and released while a model builds a resolution). | 60 |
| U3 the cause of a red | Every red proof and every return carries a cause (mechanism 3: `own`, `main`, `other <goal>`, `flake <test>`, `environment <kind>`, `unclassified`), stored in `Result` and in the queue line's return; `landing return` takes `--cause`; the landing agent's culprit search (SKILL case 3) becomes code: replay the batch's merges one at a time with the cheap gate, full proof only where the cheap gate passes, at most two full proofs, then `unclassified`. Only `own` returns to the seat; `other` returns that goal; `main` holds the batch and registers; `flake` re-proves once; `environment` retries once; `unclassified` holds and asks. A return writes `board.StageReturned`. | 150 |
| U4 trunk red | The `full-due` check (and every batch proof) that finds main red writes a trunk-red entry through `RecordTrunkRed` keyed `(main commit, test)` with the failing test's own output as evidence (mechanism 4); a green full proof on main clears it (`ClearTrunkRed`); `incident list` shows it; `work land` refuses while an entry is open except for the goal that `incident claim`ed it, and a person's `work land --exception` passes once. `proof.trunk-every` (declaration, default 4h) drives `full-due`. | 100 |
| U5 quota through the lane | The claim quota (verbs.go:1390, validate.go:399-490) excludes a goal whose latest lane entry is waiting, and counts it again when returned, by reading the lane's queue through `laneInstallOf` (`cmd/metasystem/landing_plain.go:25`); no ledger write per hand-in; AV's intent met without AV's ledger writes. | 80 |
| U6 the lane's stop and question | The lane's hold is a stop record (mechanism 2: loop `lane-proof`, attempt n of 2, measure "red set shrinks", class the failing test, handoff `hold <incident>` or `ask <question>`), shown by `landing status`; the ask is a `question` whose text is the one command that resolves it (mechanism 12). The barren hold of `barrenHold :338` becomes a stop record of loop `lane-return` with class "lane unchanged". | 80 |
| U7 the proof ladder's keys | `proof.full` (committed-only, mechanism 5) replaces the per-installation `landing.prove.command` for the batch proof and the trunk check; `proof.cheap` runs after every merge in a batch as the merge gate (SKILL step 3 gains the gate); `settings check` validates both; `system adopt` leaves them explicitly missing. | 100 |

Build order: U1, U2, U3, U4, U5, U7, U6. Each unit: one build, at most two corrections, material must fall, a class repeat stops the unit (plan, stop section). Each unit's acceptance has one test through the public verb (`work land`, `landing return`, `landing prove`, `goal claim`, `incident list`), never only through a bed.

## Estimates

Per unit about 70 minutes (a 20-30 minute Codex build, a 10 minute proof, a 20 minute Opus read) when the first read is clean; one correction adds about 50 minutes. Seven units: about 8 to 11 hours of machinery time, one goal in flight. Box proposed: 3d/40/3000m/2/20 (working-hour days).

## Not in this goal

The lane's policies and the helm (`settings` keys `landing.batch`, `landing.on-red`, `landing.trunk-red`, `landing.proof`; `landing drain`; `helm take --all`): design 1b. Areas on the claim: 1b. The records ref (finding 3): its own decision. The resolve round moving into the lane's merge commit: after 1a, when the lane's returns show it is needed.

## Readers of what this goal changes

`work land`: `cmd/metasystem/intent_delivery.go` (runIntentLand, landGoalRoute, handLandingSubject, deliveredHint), `cmd/metasystem/landing_plain.go` (handIn, laneQueueState, writeJoinedCard), `cmd/metasystem/landing_rebase.go`, `cmd/metasystem/intent_work_rebase.go`; the eight next-step sites above; the board (`internal/board/card.go` stages); the lane: `skills/landing-agent/SKILL.md`, `cmd/metasystem/landing_agent.go`, `internal/landing/lane/agent.go`, `internal/landing/plain/{queue,prove,push,resolve,wake,scope}.go`, `intent_landing_{prove,push,return,resolve,flake}.go`; the register: `internal/goal/trunkred.go`, `cmd/metasystem/intent_planning.go` incident verbs, `internal/steward/trunkred.go` (health role), `cmd/metasystem/intent_landing_flake.go:51` (reads the register from main); the quota: `internal/goal/verbs.go`, `internal/goal/validate.go`, `internal/goal/landgate.go`; the tests that pin today's behaviour: `cmd/metasystem/landing_plain_handin_test.go`, `cmd/metasystem/landing_rebase_test.go`, `cmd/metasystem/intent_delivery_owner_test.go` ("a hand-in never moves main"), `cmd/metasystem/landing_plain_cards_test.go`, `cmd/metasystem/landed_notice_test.go`, `cmd/metasystem/intent_records_test.go`, `internal/goal/landing_test.go`, `internal/goal/validate_test.go`.

## Acceptance of the goal

- A goal with an unbuilt or unread unit of its accepted design cannot be handed in; the refusal names the unit; after a published read the next step is the next unit or `goal done`.
- A hand-in rewrites no unit commit; a branch's reads survive integration.
- Every lane return carries a cause; a red that is main's own, or another goal's, never returns the goal; main's red is an incident `incident list` shows, and landing holds except for its fix goal.
- A seat whose goal waits in the lane can claim its next goal; a returned goal holds the slot again.
- The full cmd/metasystem package is green on the integrated tree.

## Questions the design may put to Wido

Only ones whose answer changes what is built. Name the mechanism and the two options.
