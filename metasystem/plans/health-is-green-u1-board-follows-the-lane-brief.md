# Brief: health-is-green-when-the-seat-is-healthy, unit 1 (the board follows the lane)

Goal state: claimed by m1l, tier 2, approved box 1d/10/1200m/1/20. One committed read for this unit (fleet rule of 2026-10-04): build rounds until the check is green, then one committed review; a material finding gets one revise round in the same chain; non-breaking findings become notes.

# Goal

A seat's health verdict is green when the seat works and red only for something someone can act on. This unit makes one fact true: when the landing lane pushes a goal's hand-in to main, that goal's board card stops showing a stage that stalls, and it says how many of the goal's units are on main. A hand-in itself moves the card to a waiting stage. Today a handed-in card keeps `judgement` or `land-ready` and reads "stalled" on the Fleet page while the seat works on something else (live example: m1f's card for one-folder-deployed-and-evolved said `judgement` from 06:58 UTC on 2026-10-04, after its unit landed at 10:13).

# Workspace

Branch goal/health-is-green-when-the-seat-is-healthy. Its workspace does not exist yet; `metasystem work build` prepares it. Leave the change there, uncommitted.

# Inputs

- Design: /Users/wido/LocalStorage/GitHub/agentic-tools-m1l/metasystem/plans/designs/health-is-green-when-the-seat-is-healthy.md (revision 2; Astra critique closed at round 3, rounds 5/1/0; accepted 2026-10-04 17:37 CEST by m1e in Wido's word). Its section 3 "Step 1" and the unit row `board-follows-the-lane` of section 7 are binding for this unit. Section 5 (Deferred) and unit 2 (the stuck detector) are out of scope.
- Code sites (paths from `metasystem/`):
  - `internal/board/card.go`: the stage vocabulary and `Terminal`/`progressing` (lines 43-90), `Card` (lines 142-157), `WriteAt` with the seat lock (lines 282-306), `carry` (lines 308-349), `progressed` (line 351), `LiveCard` (lines 429-455).
  - `internal/board/view.go`: `GoalView` (line 106), the text view's idle counting, `goalText` and `StageText` (lines 251-284).
  - `cmd/metasystem/intent_landing_push.go`: `landedMessage` (lines 15-31) and `runIntentLandingPush` (lines 46-95); `cmd/metasystem/intent_landing.go`: `laneVerbOwners` (line 32 on).
  - `internal/landing/plain/queue.go`: `Entries` (line 151), the superseded marking in `entriesOf` (lines 160-192), `Waiting` (lines 194-205); `internal/landing/plain/push.go`: `ContainedIn` (line 119).
  - `cmd/metasystem/landing_plain.go`: `handIn` (lines 66-88); `cmd/metasystem/intent_delivery.go`: `writeHandLandingCard` (lines 2051-2078), the hand route's own card writes, which stay as they are.
  - `internal/goal/branch/range.go` (lines 97-110): how a `Goal-Unit` trailer names one or more units.
  - The Fleet page: `internal/ui/web/_app/src/fleet/panel.ts` (`STAGES` lines 706-716, `doingOf` from line 768), `internal/ui/web/_app/src/fleet/api.ts` (`BoardGoal` line 399), and the Go payload in `internal/ui/httpd/board.go`.
  - Test beds: `cmd/metasystem/landing_plain_handin_test.go` (`plainLaneBed`, line 17), `cmd/metasystem/landing_plain_verbs_test.go` (`plainVerbBed`; it runs real Git, so the new push test does not use it: stub the push and containment through `laneVerbOwners`), `internal/board/card_test.go`, `internal/board/view_test.go`, `internal/ui/web/_app/src/fleet/panel.test.ts`.

# Units

| Unit | Lines |
| --- | ---: |
| board-follows-the-lane | 460 |

## What this unit builds

1. **`board.Update(home, seat, goal, decide)`**: reads the seat's card for the goal under the same seat lock `WriteAt` takes (a zero card when there is none), calls `decide(current) (Card, bool)`, and when it returns true writes the result through `carry`, all inside the lock.
2. **The count field**: `Card.Landed int` (`json:"landed,omitempty"`), carried into `GoalView` and the interface's board payload. `carry` keeps `Landed` from the same claim's previous card when the new card brings none (as it keeps `Stages`); a new claim starts at zero. `progressed` treats a changed count as progress. `SchemaVersion` stays 1.
3. **The lane's push writes the cards.** In `runIntentLandingPush`, when the push changed main and next to the channel post: select the hand-ins whose sha the new main contains and the old main did not, over every queue entry that is not returned (waiting and superseded), through a new `laneVerbOwners` containment owner (default `plain.ContainedIn`); one write per goal. The channel message keeps its present selection (waiting entries only). For each goal, find the seat with `board.LiveCard`; with none, write nothing and add one Details line. Count the goal's units on the new main through a new owner (default in `plain`: distinct unit names from the `Goal-Unit: <goal>/...` trailers of commits reachable from the new main, parsed the way `internal/goal/branch` parses them); a count that cannot be read is left unwritten and told. Then `board.Update` decides on the card as it stands under the lock: terminal or gone, no write (told); process-bound (`build`, `revise`, `unit-proof`, `review`, `landing`), keep everything and set only the count; otherwise the stage becomes `claimed-idle`, or `joined` when a newer hand-in of the same goal still waits outside the new main, with owner, job and proof cleared, round kept, writer component `landing-push`. A failure of any of this adds one Details line and never fails or undoes the push, its record, the channel post or the deploy.
4. **The hand-in card.** `handIn`, once `plain.HandIn` succeeds, writes `joined` through `board.Update` (process-bound cards are left alone; no owner, no batch).
5. **What people see.** `StageText` says "3 units landed" ("1 unit landed") for a card with a count: alone on `claimed-idle`, after the stage words on other stages. The text view shows a `claimed-idle` card with a count instead of counting it idle. The Fleet page's Doing column admits a held `claimed-idle` card with a count among waiting cards and says "3 units landed"; running work and job records still come first. `landed` keeps its meaning (terminal, written only by the hand route).

## Not in this unit

Unit 2 (the stuck detector), a `returned` card from `landing return`, retrying an unwritten card, the count on other surfaces, any change to the stall rule, the Doing column's order, the lane's records or the hand route's card writes.

# Constraints

The accepted design is the specification. Build in Go, plus the Fleet page's TypeScript; no new dependencies. Behaviour tests stub Git (project rule): no test runs real Git, calls `t.Setenv`, or reads `metasystem.conf.local`; use per-test owners and the board home under a temporary directory (the bed's registry home, as `board.HomeWith` reads it). Any change under `internal/ui/web/_app`, tests included, rebuilds the committed bundle in the same change: from `internal/ui/web/_app`, with `/Users/wido/.npm/_npx/538786c08bcb9442/node_modules/node/bin` first on PATH (Node 24.21.0), run `npm ci --prefer-offline --no-audit --no-fund` if `node_modules` is missing, then `npm run bundle`; otherwise `TestBundleIsCurrent` goes red. Since batch 14 the fast gate refuses dead Go code: every new function and field is reached by production code. Source comments state behaviour and invariants in plain English, never review rounds or history. Never start, stop or restart the standing interface on 127.0.0.1:7878, and never kill processes by pattern.

Maximum reader tool calls: 60

# Expected Return

The change, uncommitted in the goal worktree, with a report: what moved (file by file), the exported names and signatures added, the test list with each test's mutation check (break the rule, see the test fail, restore), and the changed-line count (production, test and bundle separately).

# Acceptance Criteria

- `go build ./...`, `go vet ./internal/board/ ./internal/landing/plain/ ./cmd/metasystem/` and `go run -trimpath ./cmd/devgate static` pass.
- `go test -count=1 ./internal/board/ ./internal/landing/plain/ ./internal/ui/web/ ./internal/ui/httpd/` and `go test -count=1 -run 'PlainLane|WorkLand|HandIn|LandingPush|Board' ./cmd/metasystem/` pass.
- In `internal/ui/web/_app`: `npx tsc --noEmit` and `npx vitest run src/fleet` pass.
- Tests exist and pass, each failing under its named mutation:
  - `cmd/metasystem` `TestPlainLanePushWritesLandedCards` (owners stubbed, no Git): a held `land-ready` card becomes `claimed-idle` with the count; a process-bound `build` card keeps its stage, owner and job and gains the count; a goal with a newer waiting hand-in becomes `joined`; a superseded hand-in that the new main contains is written; a goal with no live card writes nothing and is told in Details; a failing card home leaves the push confirmed. Mutations: select over `plain.Waiting` only; write the stage over a process-bound card.
  - `internal/board` `TestUpdateDecidesOnTheCardUnderTheLock`: after `LiveCard` returned `land-ready`, a `build` card is written; `Update`'s decision sees `build` and keeps it; a `released` card gives no write. Mutation: decide on the card read before the lock.
  - `internal/board` `TestBuildWriteKeepsTheLandedCount`: a `build` write after a count keeps it; a new claim (after `released`) starts at zero. Mutation: drop the carry of `Landed`.
  - `internal/board` `TestStageTextSaysUnitsLanded`: "3 units landed", "1 unit landed", the stage words first on other stages; a `claimed-idle` card with a count is shown, not counted idle.
  - `cmd/metasystem` `TestWorkLandHandInWritesJoined` on `plainLaneBed` with a pre-written `land-ready` card: after the hand-in the card is `joined`; a process-bound card is left alone.
  - `panel.test.ts`, the Doing column: a held `claimed-idle` card with `landed: 3` says "3 units landed"; a running job wins over it.

# Gap Rule

If the design or this brief does not say what to do, choose the smallest change that keeps every existing caller's behaviour unchanged, and say so in the report; stop and report a gap when no such choice exists.
