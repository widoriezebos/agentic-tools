# Design for health-is-green-when-the-seat-is-healthy

- Kind: design
- Id: 01M43NHWS54TM32NRZ4643MNFK
- Status: accepted
- Goals: health-is-green-when-the-seat-is-healthy
- Critique: closed at round 3 on 0 material findings (Codex Astra, chain design-critic-8f9197ccb54a1b974a2756b9)

Accepted 2026-10-04 17:37 CEST by m1e in Wido's word (peer message d-a3e8291b20479cfe9903cf886e), after Astra rounds 5/1/0; Wido may overturn.

Step 1: the board follows the lane. Revision 2, 2026-10-04: Astra's five round-1 findings folded, each marked "(revision 2)". Brief `plans/health-is-green-board-follows-the-lane-design-brief.md`; lines at main 1dccd8b2b under `metasystem/`.

## 1. Scope

- Changes: `landing push`, when it moves main, writes the live card of every goal whose hand-in it put on main: out of every progressing stage, carrying the goal's units on main, decided under the seat lock (revision 2).
- Changes: the board carries a card's count forward as it carries its stage spans (revision 2).
- Changes: the lane-route hand-in writes `joined` in the same act as its queue line (D4 taken).
- Changes: a card carries `landed` (a number); the text view and the Doing column say "N units landed".
- Leaves alone: the stage vocabulary and its terminal set (revision 2), the stall rule, the Doing column's order, the lane's records and channel message, `landing return`, the hand route's writes, unit 2, the other standing reds.

## 2. What is true today

Facts 1 to 6 of the brief hold at the cited lines. Additions:

- `landed` is terminal (`internal/board/card.go:63`): `LiveCard` ignores it, release writes nothing over it (`internal/goal/verbs.go:6349`), the bridge expires it (`internal/board/bridge.go:411-417`). Hand-route `landed` cards exist on this host (revision 2).
- `judgement` and `land-ready` are progressing but not process-bound (`card.go:74-87`): they stall on time alone.
- (revision 2) `WriteAt` takes the seat lock and carries only schema, provenance, `Since` and `Stages` (`card.go:282-347`); `LiveCard` reads unlocked (`card.go:433-454`); every stage writer builds a fresh card (`internal/launch/launch.go:845-866`, `internal/proofrun/launcher.go:1127-1148`, `verbs.go:6383-6388`), so a field one writer sets is dropped by the next.
- (revision 2) `plain.Waiting` keeps only `waiting` entries (`internal/landing/plain/queue.go:195-205`); a newer hand-in marks the older line `superseded` (`queue.go:174-179`); the lane may still merge the older sha.
- `landedMessage` calls `plain.ContainedIn` directly (`cmd/metasystem/intent_landing_push.go:20-23`). The lane route discards `handLandingSubject`'s per-hand-in count (`intent_delivery.go:1908`), which double-counts across returns.

## 3. Step 1

**Writer.** `runIntentLandingPush`, when `outcome.Changed`, beside `postLanded`. (revision 2) `landedHandIns` selects over every `plain.Entries` entry not returned (waiting and superseded): head contains the sha, old main did not, through a `contained` owner. Cards take the whole set; the channel message keeps its selection (the waiting ones). One write per goal.

**Which card.** `board.LiveCard(home, goal)` names the seat; the queue line's nickname names no installation. No live card (the seat released after handing in, or two seats hold one): no write, not even the count; one Details line; the lane list shows the landing.

**The write (revision 2).** `board.Update(home, seat, goal, decide func(current Card) (Card, bool))` takes the lock `WriteAt` takes, reads the card (zero when gone), calls `decide` and writes through `carry` when it says so. The lane decides on that card, not `LiveCard`'s: terminal or gone, no write, told; process-bound, the same card with only the count set; otherwise the stage change below. A `build` or `released` written between finding and writing is kept.

**Stage (D3, revision 2).** `landed` stays terminal, the hand route's; cards already landed are untouched. A held card leaves its progressing stage for `claimed-idle`, or `joined` when a newer hand-in of the goal waits outside head; owner, job and proof cleared, round kept; writer `landing-push`. Both wait: never stalled, found by `LiveCard`, ended by `released` (release, park, done, handover), moved on by the next build, history kept.

**Count (D2).** N is the distinct unit names in `Goal-Unit: <goal>/...` trailers reachable from the pushed main, read in the lane checkout by `git log --format=%(trailers:key=Goal-Unit,valueonly) --fixed-strings --grep=<prefix> <head>`, parsed as `branch` does: what the sentence claims, whichever route landed them. An unreadable count is left unwritten and told.

**Field (revision 2).** `Card.Landed int` (`landed,omitempty`), into `GoalView` and `BoardGoal`. `carry` carries `Landed` from the same claim's previous card when the new one brings none, as it carries `Stages`; a new claim starts at zero. Only the lane's update sets it. `progressed` treats a changed count as progress. `SchemaVersion` stays 1.

**Hand-in card (D4, taken).** `handIn` writes `joined` through `board.Update` once `plain.HandIn` succeeds; a process-bound card is left alone; no owner, no batch; the page says "waiting to land".

**Shown (revision 2).** `StageText` says "3 units landed" ("1 unit landed") for a card with a count, alone on `claimed-idle`, after the stage words elsewhere; the text view shows such a `claimed-idle` card instead of counting it idle. The Doing column admits it among waiting cards; running work and job records still win (`panel.ts:777-793`).

**Failure.** An error or a decision not to write adds one Details line, like a failed channel post; push, record, post and deploy are untouched.

**Tests red without it**

| Behaviour | File, test, fixture |
| --- | --- |
| Push writes `claimed-idle` with the count; a process-bound card keeps its stage, gains the count; a newer waiting hand-in gives `joined`; a superseded hand-in in head is written (revision 2); no live card is told | `cmd/metasystem/landing_plain_verbs_test.go` `TestPlainLanePushWritesLandedCards`: lane verb bed, temporary registry home, `push`, `contained`, `unitsOnMain` stubbed, cards read back at a one-second stall |
| A card that moved between finding and writing is not overwritten (revision 2) | `internal/board/card_test.go` `TestUpdateDecidesOnTheCardUnderTheLock`: after `LiveCard` returns `land-ready`, a `build` is written; `Update` keeps `build` and sets the count; a `released` gives no write |
| A `build` write after a landed count keeps the count; a new claim starts at zero (revision 2) | `card_test.go` `TestBuildWriteKeepsTheLandedCount` |
| The words; a `claimed-idle` card with a count is shown, not counted | `internal/board/view_test.go` `TestStageTextSaysUnitsLanded` |
| Hand-in writes `joined` | `cmd/metasystem/landing_plain_handin_test.go` `TestWorkLandHandInWritesJoined` on `plainLaneBed`, a `land-ready` card pre-written |
| Doing column: a held `claimed-idle` card with a count says "3 units landed"; a running job wins | `panel.test.ts`, "the Doing column" |

**Seams (D5).** `laneVerbOwners.push` and `now`; new `contained` (default `plain.ContainedIn`) and `unitsOnMain` owners; the board home through `METASYSTEM_SUPERVISION_REGISTRY_HOME` (`board.HomeWith`).

## 4. Moved effects

(revision 2) The lane's write and the count are new effects, not moves. One owner moves:

| Effect | From | To | Code |
| --- | --- | --- | --- |
| The stage a goal's card shows while its hand-in waits in the lane | `goal ready` (`land-ready`) and the unit run's `judgement` write | `work land` hand-in (`joined`) | `metasystem/internal/goal/verbs.go`, `metasystem/internal/launch/unit_run.go`, `metasystem/cmd/metasystem/landing_plain.go` |

## 5. Deferred

| What | Builds on |
| --- | --- |
| Unit 2, the stuck detector (added 17:00): its own design page | nothing of step 1; it reads unit runs, not cards |
| One row per standing red (stop-hook duration included), the no-remedy escalation rule, per-checkout-kind role sets | later steps |
| Goal word (2): job records win over a stalled card in `doingOf`; "installation X is not the armed checkout Y" for a goal worktree | the Doing column's order; `unknownReason` |
| A `returned` card from `landing return`; `work status` marking a seat running for a waiting card (`intent_status_view.go:211`); retrying an unwritten card | `board.Update` and the `contained` seam; `Card.Landed`; the Details line |

## 6. Open questions for Wido

1. Does step 1 take D4? Recommended: yes; it closes the hours between hand-in and push.
2. With no live card, write nothing (recommended) or the count onto the `released` card, which nothing shows and the bridge expires?

## 7. Size and units

| Unit | Production | Test | Notes |
| --- | --- | --- | --- |
| board-follows-the-lane | ~200 lines: card.go 40, intent_landing_push.go 75, plain 30, the rest 55 | ~260 lines, six tests | plus the regenerated bundle: ~20 diff lines, 1.5 MB |

One unit, under 1,000 changed lines without the bundle.

## Dispositions (critique design-critic-8f9197ccb54a1b974a2756b9)

Written by metasystem design review when critique design-critic-8f9197ccb54a1b974a2756b9 closed: every answered round's decisions, as the author made them.

| Round | Finding id | Finding | Disposition | Reasoning and evidence | Amendment |
| --- | --- | --- | --- | --- | --- |
| 1 | HEALTH-CARD-UPDATE-RACE | The landing writer can overwrite a newer build or release because its card and stage selection happens outside the write lock. Step 1 needs a conditional update against the current card; otherwise it can hide running work or revive a released card. | accepted | Right: `board.LiveCard` reads without the seat lock (`internal/board/card.go:433-454`) and `WriteAt` only carries history (`internal/board/card.go:311-347`), so a seat's `build` or `released` written between the lane's read and write is overwritten. | Make the lane's write a conditional update decided on the card as it stands under the seat lock: a new board primitive (for example `board.Update(home, seat, goal, func(current Card) (Card, bool))`) that reads, decides and writes inside the lock `WriteAt` already takes. The decision runs on the current card: terminal or gone means no write; process-bound means only the count; otherwise the stage change. Add the red test: a card that moved between `LiveCard` and the write is not overwritten. |
| 1 | HEALTH-LANDED-LEGACY-CARDS | Making existing landed cards non-terminal turns already completed work into unclaimed live cards that no longer expire. The design needs to account for releases that happened before this change. | accepted | Right: terminal `landed` cards written by the hand route exist today and were never released (`internal/goal/verbs.go:6343-6352`); redefining the stage turns them into unclaimed live cards that the bridge never expires (`internal/board/bridge.go:411-416`). | Smallest fix, recommended: leave `landed` terminal and its hand-route meaning unchanged, and have the lane move a held card to a stage that already waits without stalling and is not terminal (`claimed-idle`, or `joined` while a newer hand-in waits), carrying the count; drop open question 1. If the author keeps a non-terminal `landed`, the page must say how cards already landed at deployment end (expiry or a one-time sweep) and test it. |
| 1 | HEALTH-LANDED-SUPERSEDED-HANDIN | The waiting-entry selection misses an older hand-in landing while a newer hand-in of the same goal waits. It therefore skips the count update in a case explicitly promised by step 1. | accepted | Right: `plain.Waiting` excludes superseded lines (`internal/landing/plain/queue.go:172-180`, `internal/landing/plain/queue.go:194-203`), and the lane can push a HEAD that merged an older sha after a newer hand-in superseded it. | Select the cards to write by containment over every hand-in line that is not returned (waiting and superseded): head contains the sha, old main does not. Keep the channel message's selection as it is unless the page says why it should change too (then it is a moved effect). Add the superseded case to the push test. |
| 1 | HEALTH-LANDED-COUNT-LIFETIME | The cumulative landing count has no preservation rule across ordinary card writes. The next build, review, proof, or hand-in can erase the count the lane just published. | accepted | Right: every stage writer builds a fresh card (`internal/launch/launch.go:845`, `internal/launch/unit_run.go:587`, `internal/proofrun/launcher.go:1142`) and `carry` keeps only history and times, so the next write drops the count. | The board's write carries `Landed` forward from the previous card of the same claim (as `carry` keeps the stage spans), so no other writer has to know the field; only the lane's update sets it. Add the red test: a `build` write after a landed count keeps the count. |
| 1 | MOVED-EFFECTS-NO-ROWS | The moved-effect inventory fails the required check and has no code anchors for its ownership changes. | accepted | Right: the inventory check needs the header Effect / From / To / Code with existing backticked code paths and a named old owner (`internal/validate/movedeffects.go:27-31`, `internal/validate/movedeffects.go:100-133`); 'nobody' is a weak owner. | Rewrite section 4 in that form. List only effects whose owner moves, each with its code paths; a new effect (the lane writing a card, the count) is not a move and belongs in section 3. Run the moved-effects check on the page before returning it. |
| 2 | MOVED-EFFECTS-NO-ROWS | The moved-effect table now has valid columns, but its three code paths omit the repository's metasystem/ prefix, so the mandatory ownership check still fails. Correcting these references requires no additional runtime mechanism. | accepted | Right: the inventory resolves code paths from the repository root, so the three paths needed the `metasystem/` prefix. | Section 4's row now names `metasystem/internal/goal/verbs.go`, `metasystem/internal/launch/unit_run.go` and `metasystem/cmd/metasystem/landing_plain.go`; `metasystem design review --check-only` reports rows=1 problems=0. No other change. |
