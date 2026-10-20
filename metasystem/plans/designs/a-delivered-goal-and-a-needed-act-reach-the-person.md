# A delivered goal and a needed act reach the person

- Kind: design
- Id: 01M5DG000000000000000DGA01
- Status: accepted
- Goals: a-delivered-goal-and-a-needed-act-reach-the-person
- Critique: ruled by Wido (steward acting in his name) 2026-10-20, stop at round 3: 4, 3 and 1 material findings in three Astra rounds, all folded

Tier 2, revision 4, Opus 5.5 draft for the steward, accepted at the stop. Revision 2 folded Astra round 1, revision 3 round 2, revision 4 round 3 (see the critique record at the end). Read on 2026-10-20, 18:30-19:40 CEST; the fold was re-read against the same tree on 2026-10-20 evening. Code read in the U1a workspace `artifacts/agents/workspaces/goal-a-persons-channel-carries-only-acts-and-deliveries/u1a/metasystem` at 3abe884dff (U1a committed on top of 649acb4691). Records read in `/Users/wido/LocalStorage/GitHub/agentic-tools-m1e` and in the landing and ui checkouts. Rulings relied on: R-104-m1e (artificial clocks, `memory/rulings.md:163`), R-129-ui (a repeat act is success, `:188`), R-142-m1e and R-143-m1e (`:201-202`).

This goal builds on the accepted page `plans/designs/a-persons-channel-carries-only-acts-and-deliveries.md` ("the channel page" below). It starts after that goal is done (the goal record says so). So U1b, U2 and U3 of the channel page are taken as built. Where this page names a U3 symbol (`personClass`, `needsAPerson`, the machine ask), that symbol does not exist yet. Those names are marked **(U3)**.

## Problem

- **No message says a goal was delivered.** On 2026-10-20 the landing and ui stewards journaled 100 and 103 notices (`analysis/notices-2026-10-20.md`). 54-56 were `steward: idle-backlog-dead`, 34 were `steward: degraded`. Two landings reached main that day and one goal concluded. No notice said so. The only lines that named them were `steward: the shared goal ledger moved to <sha> — the queue reordered ...`.
- **Premise corrected.** The analysis says "delivery-goal exists only in a test". That is true, but it is a fixture goal id in `internal/steward/delivery_test.go:18-26`, not a notice. No production code makes a goal-delivered notice.
- **The landing post does not cover it.** `PostLanded` (`internal/channel/landed.go:62`) posts once per sha. It runs only on the lane's route. The landing checkout's `artifacts/agents/channel/landed.json` holds 38 posted shas and was last written on 2026-10-14 18:46. Today's two landings were by hand, so nothing posted.
- **Who concludes a goal.** Every conclusion is the `goal done` verb. `goal.Done` (`internal/goal/verbs.go:2416`) publishes `doneRequest` (`:2805-2809`, message `goal done <id>`), which moves the file from live to done (`:2905`, `:2915`). Its only caller is `cmd/metasystem/goalsync_mutations.go:1980`. `internal/landing` never concludes a goal (grep for `goal.Done`, `doneRequest` and `"done"` over non-test code). "The lane concludes" means the landing agent runs `goal done` in the lane checkout. "Wido concludes" means he runs it at the enrolled pane. Both write the same ledger commit. Today's: `b2f8570278 goal done a-stop-waits-for-the-unit-boundary`, history line `2026-10-20T13:31:29Z ... done actor=human:Wido`.
- **The ledger is main.** Ledger tips are main commits: `ea022d4baa`, `b2f8570278` and `93938814b5` are all on origin/main. The unit commit `a0f587dc6f` (`Goal-Unit: a-stop-waits-for-the-unit-boundary/U1a.1`) is an ancestor of the done commit `b2f8570278`. The lane checkout's local `main` is at 9367b11f9 (2026-10-02) and does not contain it. Its origin/main does.
- **Every steward sees every move.** Each checkout's steward diffs the shared ledger on its own (`internal/steward/ledgerattention.go:225-299`). Local verb moves come through `recordAcceptedLedgerTransitionWithRepositoryAndWriter` (`:539-551`). Fetched moves come through the stage at `:666`. The diff already compares the Live, Done and Abandoned trees per change (`:257-272`). It records only `MovedGoals` for the digest line, and keeps an event only when claimable, pins or queue changed (`:286`). A first pass sets a baseline and makes no events (`:600-613`). Local mode makes no events at all (`:575-581`).
- **Three stewards on one host.** The landing, ui and m1e checkouts each run a steward. Landing and ui journaled the same six ledger moves today (same tips in both journals); m1e journaled nothing that day, so the three-root fixture (`TestOnlyTheLaneKeeperAnnounces`) is what proves the third steward stays silent. The seat presence lines show the same duplication: `m1e is reachable again` is in both the landing and the ui journals. A notice made from the ledger diff, with no rule about who sends it, would reach the person three times.
- **The funnel contract (U1a, read LAND).** `noticeFunnel{clock, decider}` (`notify.go:140`). `send` takes the decision as a value and reads the clock after the transport (`:167-179`). `deliverNoticeWith` decides once (`:180`). The result is `Submitted`, `Withheld` or `Failed`. `TransportWithheld` and `episodeSettled` are at `alert_episode.go:27-34`. `submitEpisode` decides before it saves an attempt (`:547-592`). The journal line has `withheld` when set (`notify.go:401`). A `Notice` carries `Message`, `Source` and `Ref` only. It has no owner field: for an episode `Source` is `alert` and `Ref` is the episode id (`alert_episode.go:548`). For a queue item the source comes from the nonce prefix (`notify.go:406-415`).
- **A pattern to copy.** The breach-stop notice is one alert episode per goal revision, keyed by digest, posted to the channel, retried from its own record on each pass (`signal_channel.go:104-176`). The lane-silent signal is sent only by "the steward that keeps the lane" (`lane_silent.go:84-91`, `lane.OwnsLane` at `internal/landing/lane/agent.go:581-593`). The tick reaches the lane through an injected reader (`tick.go:465`, `:667`).
- **Where the noise comes from.** `idle-backlog-dead` and `degraded` are queue lines with nonces `verdict-<verdict>` (`tick.go:649`, `:816-829`; `revive.go:195`). Ledger moves are queue lines `ledger-attention-<source>` (`narrate.go:321-326`, queued at `tick.go:544-553`). Narration is `noticing-<key>` (`narrate.go:334-340`). The engine re-arm is `engine-rearm-generation-N` (`runner.go:1201`). Seat transitions use `seat.NotificationNonce` (`seat/transition.go:46`). All of them go through `deliverPendingNotification` (`notify.go:327`). Today's 34 `degraded` lines on landing are one standing condition from 00:04 to 05:36. The steward requeues it every ten minutes after each delivery. The `idle-backlog-dead` lines run every ten minutes from 05:47 to 16:10.
- **Premise corrected (channel page D2).** `alert list` lists alert episodes only (`cmd/metasystem/intent_alert.go:152-165`). Verdict and ledger lines are queue items, not episodes. So `alert list` never showed them, and will not after U3 either. What does list open asks is `channel status`: every open question becomes a `Needs you:` line (`internal/channel/report.go:93-100`). Fenced goals are listed there too (`:128`).
- **Asks today.** A question is posted by the channel package itself (`question.go:325`). It is not posted through the steward funnel. The unit stop asks (`cmd/metasystem/intent_unit_stop.go:336`). The breach stop posts `Goal G was stopped: it spent its budget.` plus `metasystem goal resume G` (`signal_channel.go:104-106`). An unreachable seat is only a queue line with no command: `landing has been unreachable since 00:02 and holds no goals` (`seat/transition.go:78-98`). After U3 it would be withheld, because no row names its owner, and nothing would ask. The pane enrolment is decided only inside `status` (`cmd/metasystem/intent_helm.go:624`, `:777`). The steward has no reader for it.

## Invariant after the goal

Each time a goal enters done on the shared ledger, the person gets one plain delivery line, sent by one steward per host. This holds while a fixed keeper is registered and its episode store is kept: a successful post is then not repeated across ticks or restarts, and the other checkouts stay silent. The stated limitations remain: the crash gap and a provider timeout after success (D3), a lane moved to another checkout (threat model), and several checkouts with no lane registered (Q3). Each state where progress waits for him reaches him as one ask with its literal command. Nothing else the steward says reaches him.

## Decisions

- **D1 The key is the goal entering done.**
  - It is found in the ledger-attention diff, inside the change loop (`ledgerattention.go:249-284`, verified by reading).
  - For each change, every id that is in `projection.Tree.Done` and was not in `previousTree.Done` becomes `LedgerAttentionEvent.Delivered = [{Goal, Revision, DoneAt}]`. `DoneAt` is the `At` of the file's last history line with verb `done` (`goal/file.go:475-479`).
  - The event is kept when `Delivered` is not empty (`:286` gains that clause).
  - Who concluded it does not matter. It covers the pane, the lane, a seat, local and fetched moves alike (`:539-551`, `:666`).
  - A revision change of a goal that is already done is not a new delivery. A reopened goal that is concluded again enters done again with a higher revision, and that is a new delivery.
  - The field is `omitempty`, so the state schema (`:20`) stays at 2. A stage that an old binary saved has no `Delivered`; that is a residual for the one upgrade tick.
- **D2 One announcer per host.**
  - The steward that keeps the registered lane announces. When no lane is registered on the host, the steward announces for its own checkout. That is the one-checkout installation.
  - Mechanism: `LaneSilence` gains `Ownership` with three values, `unknown` (the zero value), `keeper` and `other`, set in `readLaneSilence` right after the ownership check (`lane_silent.go:84-91`) and carried on every later return.
    - `keeper`: the lane is registered and `lane.OwnsLane(self, record)` holds, or no lane is registered.
    - `other`: the lane is registered, its layout reads, and `OwnsLane` is false.
    - `unknown`: `lane.Read` fails (`:85-88`), or `record.Layout()` fails. `OwnsLane` returns false on a layout error (`internal/landing/lane/agent.go:587-590`), so that case must be caught before it and must not read as `other`.
  - **Premise corrected (round 2).** Revision 2 used `Known: false` for "ownership unknown". That is wrong. `Known` is about the lane's silence. A keeper whose running, pending or question read fails also gets `Known: false` (`lane_silent.go:107-108`, `:117-118`, `:124-125`). And the paused, running, empty-queue and open-question returns give `Known: true` with no word on ownership (`:97-98`, `:110-111`, `:120-121`, `:127-128`). So ownership needs its own field, set on every return.
  - The tick reads it through the injected `lane` reader that already exists (`tick.go:465`, `:667`).
  - **Acknowledgement order (DG-01).** Unknown ownership leaves a delivery event pending. A known non-keeper may consume its copy and opens no episode. The keeper marks an event surfaced only after its delivery episode is durably saved. Today the tick marks every queued event (`tick.go:544-551`) and `PersistLedgerAttentionMark` drops it (`ledgerattention.go:719-739`), while promotion has already moved `DiffedTip` past it (`:306-307`). Without this order a keeper that cannot read its lane record for one tick would drop the delivery for good. An event left pending re-queues its own ledger-moved line under the same nonce next tick; that line is withheld (D8), so the cost is a journal line, not a message.
  - Every other steward surfaces the same event and opens no episode. That is why the second and third stewards stay silent: by the ownership rule, not by a shared mark. The alert store and its lock are per checkout (`alert_episode.go:111-123`).
  - If the keeper's steward is down, its own ledger state keeps `DiffedTip`. On return it diffs from there and announces late, with the done time in the text.
- **D3 The durable mark is a `goal-delivered` alert episode in the keeper's store.**
  - Digest: `signalDigest("goal-delivered", goal, revision)` (`signal_channel.go:72-80`).
  - It is opened and saved before the event is marked (D2, acknowledgement order). So a crash between the two re-surfaces the event, finds the digest, and opens nothing new.
  - Each tick first retries every unsettled `goal-delivered` episode, then opens the new ones. That is the breach-stop shape (`signal_channel.go:114-176`).
  - Settled means `episodeSettled` (`alert_episode.go:32`). Then the next tick, a restart, and a repeat `goal done` all send nothing. A repeat `goal done` writes nothing in any case (`AlreadyHolds`, `verbs.go:2822-2824`, R-129-ui).
  - The only duplicate left is the known one: a crash between the send and the saved result (channel page D4, `alert_episode.go:543-546` before U1a), and a provider timeout after a successful post (channel page BO-7).
  - **One visible notification per delivery (DG-02).** Today `send` journals every attempt, a failed one too (`notify.go:167-179`), and each line gets a fresh id (`notify.go:424-431`, the id at `:425`). The browser shows any line `ForAPerson` passes (`internal/ui/notifications/notifications.go:87-94`, `:301-302`) and raises a toast for every new id (`internal/ui/web/_app/src/notifications/notifications.ts:161-168`). So failed post, restart, successful post would raise two toasts for one delivery. Rule: a goal-delivered episode produces one person-visible journal notification, on successful submission. A failed attempt stays a diagnostic record on the episode (`Attempts[].Problem`, `alert_episode.go:585-587`), which `alert list` shows. Mechanism (DG-09): `noticeFunnel` gains `suppressFailureJournal bool`. Its zero value keeps today's journaling. Every existing literal (`tick.go:211`, `:319`; `alert_episode.go:276`, `:297`) leaves it false and keeps its failure lines (`notify.go:172-177`). Only the goal-delivered pass sets it true. With it true, `send` writes no journal line for a `Failed` outcome.
- **D4 What landed is read from the done commit, never from `main`.**
  - New `plain.UnitsLanded(checkout, tip, goal) (units []string, newest string, err)`. It runs `git log <tip> --format=%H%x00%(trailers:key=Goal-Unit,valueonly) --fixed-strings --grep=Goal-Unit: <goal>/`, returns the units in landing order and the newest commit, and keeps the damage check (`units.go:33-39`).
  - **`UnitsOnMain` stays (DG-04).** Revision 1 called it unused; that was a production-only grep that missed real callers. It has two production callers, `cmd/metasystem/intent_landing.go:182` and `intent_landing_push.go:291`, and three test callers: `internal/goal/branch/drop_landing_test.go:88,127`, `cmd/metasystem/intent_drop_rebase_test.go:159`, plus its own `units_test.go:9`. It becomes a thin count wrapper over the new reader, `len(units)`, so the dropped-unit guards keep their meaning. The wrapper and every replaced line are counted in U1.
  - The tip is the event's ledger commit. The ledger fetch put it in the checkout, so a stale local `main` cannot hide a hand landing (evidence above: the lane's `main` is from 2026-10-02).
  - The read runs in the keeper's checkout (`self`).
  - If it fails, the notice still goes and says the units could not be read (D5). A failed read is not a reason to stay silent.
- **D5 The words follow channel page D3.** Plain English. Local time from `DoneAt`, in a location the caller passes. The goal name is a word. A sha appears only inside the command. A delivery has no "If you do nothing" line, because nothing waits.
  - With units on main (today's case, written out):
    `Delivered at 15:31: goal a-stop-waits-for-the-unit-boundary is done; its unit U1a.1 is on main.`
    `To see what changed: git show --stat a0f587dc6f`
  - Several units: `...; its units U1a, U1b and U2 are on main.` The command names the newest commit. In U1 every unit is named. The short form for more than three units, `its units U1a, U1b and 3 more are on main.`, is optional formatting and moves to U2 (round 3).
  - **What the text may claim (DG-03).** A scan that succeeds and finds no matching trailer proves only that no unit landing is recorded. A scan that fails proves nothing. Neither proves that no code reached main (`units.go:15-18` returns the read error; `:25-38` are the trailer parse). So:
    - scan ok, no trailer: `Done at 09:12: goal X is concluded; no unit landing is recorded at this conclusion.` / `To read why: metasystem goal show X`
    - scan failed: `Done at 09:12: goal X is concluded; its landed units could not be read.` / `To read it: metasystem goal show X`
    - Neither text says that no units reached main.
- **D6 The transport is the producer's own, and the funnel decides.**
  - Use `signalTransport(repoRoot, text, deliver)` (`signal_channel.go:65`): the channel when one is configured, else the Mac notifier. The landing checkout posts to the channel today (38 shas in its `landed.json`).
  - Funnel: `noticeFunnel{clock: cfg.now}`, as the breach-stop path at `tick.go:211` (U1a read).
  - The class is `delivery`, given by one row `goal-delivered → delivery` in `personClass` **(U3)**. Nothing withholds a delivery by its words.
- **D7 "Your act is needed" is an ask. This goal does not build a second producer for it.** It depends on U3's decider and U2's words. Why:
  - The channel page already has one decision site (D5) and one identity per decision (D6). A second producer of act notices would bring back the many call sites that page removes.
  - The question record is already the durable bound against asking twice (channel page D6). A new notice type would need its own bound.
  - Channel page D3 already gives an ask's post as one sentence, `To go on: <literal command>`, and `If you do nothing: ...`. That is the act-needed notice.

  The four sites the goal names:
  1. **Fenced goals.** The breach stop posts once per goal revision (`signal_channel.go:104-176`), in U2's words. Fenced goals are also listed in `channel status` (`report.go:128`). Nothing to add.
  2. **Stopped units.** The unit stop is a question (`intent_unit_stop.go:336`). U2 fills its words. Nothing to add.
  3. **The unreachable seat. This goal adds it as row 7 of `needsAPerson` (U3).**
     - It applies when a seat is `unreachable` and holds at least one goal (`seat.MachineStanding.Holds`, `internal/seat/standing.go:70-90`).
     - Identity: `{Kind: "seat-unreachable", Subject: machine, State: sorted holds}`. `Since` stays out, because it is each reader's own first sighting (`standing.go:81-83`).
     - Which steward asks: the reachable machine with the smallest name, among the standings that `seat.Fleet` returns (`standing.go:155`), asks; the others do not. Every steward reads the same refs, so they agree. In a brief window they may not, and then two questions can appear (residual).
     - **Dedup is checkout-local (DG-13).** Seat asks are deduplicated within the elected checkout's question store (`internal/channel/question.go:128-143`; channel page revision 4 on BO-4). If the elected steward changes, for example because it becomes unreachable itself, its successor starts from its own empty store and can ask the same decision again, even when every reader agrees. Continuity across checkouts is deferred.
     - Words: `Seat ui has not reported since 00:02 and holds 2 goals: G1, G2.` / `To go on: start it in its checkout: metasystem system start` / `If you do nothing: its goals stay claimed by it and nobody works on them.`
     - **Recovery (DG-07).** Seat decisions use U3's stable-recovery discipline, extended here explicitly to seat standings. A reachable reading does not close or retire the decision until the seat has been reachable without a break for `healthFlapWindow` (1 h, `health.go:173`), read from the standing's frozen `Since` (`standing.go:81-83`) on the injected clock. Then the question closes with `Closed: ui reports again.` and the decision is retired. Unreachable, briefly reachable, unreachable again with the same held goals asks nothing new.
     - The hook is in `noticeSeatStandings` (`seatpresence.go:237-252`), after the standings are computed. The queue line stays as it is and is withheld by D8.
  4. **Pane enrolment.** Out of scope, see below.
- **D8 Noise is one rule, not eight edits.**
  - `personClass` **(U3)** allows only the owners it lists: questions, breach stops, the spend pause, U3's asks, and `goal-delivered`. Anything else is withheld (channel page D5: an unknown owner is withheld).
  - These have no row, so they are withheld with no call-site change: `verdict-*` (idle-backlog-dead, degraded, stalled), `ledger-attention-*`, `noticing-*`, `engine-rearm-*`, and seat transitions.
  - This goal's whole edit for D8 is the one row in D6 and the tests in U2.
  - Where the withheld lines go:
    - the journal file, with `withheld: <class>` (`notify.go:401`), which the browser skips (channel page D5);
    - the narrator digest, which already holds the ledger move (`narrate.go:64`) and the escalation (`narrate.go:90-94`);
    - `alert list`, for episodes only.
  - A standing condition that needs him becomes a question. It shows as `Needs you:` in `channel status` (`report.go:93-100`).
  - **Degraded (DG-06).** Raw degraded verdicts produce no person-visible notice. Persistent degradation still produces the single act question that the accepted channel page requires (its D6 row 4, `plans/designs/a-persons-channel-carries-only-acts-and-deliveries.md:98-115`; its test `TestDegradedTickAsks`, `:194`). The two outcomes are tested apart (U2). Q1 asks Wido to confirm that reading of the goal record.
- **D9 No person is overruled.** No notice gates a verb. A failed or withheld notice never refuses or delays `goal done` (R-142-m1e). No person overrides a gate here, so no impact statement is due (R-143-m1e).

## Units

Inventory: the sites above, from grep over non-test Go in the workspace at 3abe884dff, 18:40-19:30. Line counts are additions plus deletions, comments included. Each estimate comes from the nearest existing shape: `noticeBreachStops` is 63 lines (`signal_channel.go:114-176`) and `unitsOnMain` is 31 (`units.go:14-44`). Order: U1, then U2. They touch different files, apart from one line of `tick.go`.

- **U1 Goal delivered (D1-D6). 187-231 production lines, stop line 235. Usable alone: each goal concluded from the pane or the lane reaches the person once, in plain words.**
  - `LedgerAttentionEvent.Delivered` and its type (`ledgerattention.go:30-38`): 8.
  - Entry into Done inside the change loop, and the keep clause (`:249-286`): 14-18.
  - `plain/units.go`: the new `UnitsLanded` reader, built from the body of `unitsOnMain` (`:14-44`, 31 lines, rewritten to keep order and the newest commit), plus `UnitsOnMain` as a count wrapper (DG-04): 36-46 (additions plus deletions).
  - `LaneSilence.Ownership`, three values, set after the ownership check and carried on every later return (`lane_silent.go:34-44`, `:84-128`): 10-14.
  - `noticeFunnel.suppressFailureJournal` and its check in `send` (`notify.go:140-179`, DG-09): 4-6.
  - Liveness seam for the lane reader (round 3, R-104-m1e): `readLaneSilence` takes an `alive func(plain.Running) bool` and passes it as `plain.ProveSeams{Alive: alive}` to `plain.ReadRunning` (`lane_silent.go:106`, where it now passes `plain.ProveSeams{}`). A nil callback keeps today's `identity.KernelProber{}` path (`plain/prove.go:305-313`; the field is `ProveSeams.Alive`, `prove.go:250`). `hostLaneSilence` and `readLaneSilenceWithDesignChecks` pass it through (`lane_silent.go:47-58`): 6-8.
  - New `internal/steward/goal_delivered.go`:
    - owner constant and digest: 6;
    - retry-then-open pass on the breach-stop shape: 55-65;
    - the D5 renderer with three endings (units, none recorded, unreadable) and the location passed in, every unit named: 24-30;
    - the done-time read from history: 6-8.
  - Tick wiring with the acknowledgement order of D2 (`tick.go:544-553`): mark only non-keeper copies and keeper events whose episode is saved, leave unknown ownership pending; funnel `{clock: cfg.now, suppressFailureJournal: true}`, the `lane` reader and `signalTransport`: 16-20.
  - `personClass` row (U3's table): 2.
  - Stop line 235, 15 under Wido's ceiling of 250. It moved from 200 (revision 1) and 230 (revision 2) because the folds of DG-01, DG-02, DG-04 and the ownership field add 34-49 lines that the unit cannot do without: each closes a loss, a duplicate or a compile break. Round 3 adds the liveness seam (6-8). To pay for it, the "and N more" formatting (about 6-8) moved to U2, as the page's stop-line rule already allowed; so the stop line stays 235.
  - **What may be cut at the stop line (DG-11).** Only optional unit-list formatting. The "and N more" branch has already moved to U2; nothing else may be cut. The scan-failure ending stays. I chose to keep the two truthful endings rather than one neutral "Goal X is done" for both: the failure ending costs about 5 lines, and it tells Wido something he can act on (look at the goal), where the neutral text would hide a broken read.
  - If U1 is still past 235 after that cut, the builder stops and reports, per the stop rule. The page's next split is then: U1a posts `Done at HH:MM: goal X is concluded.` / `To read it: metasystem goal show X`, with no unit clause, so it claims nothing about landings. U1b adds `UnitsLanded`, the wrapper and the three endings.
- **U2 The unreachable seat asks, and noise stays home (D7.3, D8), plus the short unit list. 65-96 production lines, stop line 100. Usable alone: an unreachable seat that holds goals asks once with its command, and the test journal shows that nothing else reaches the person.**
  - `needsAPerson` row 7 and its decision identity (U3's table): 6-8.
  - Smallest-reachable-name election over `seat.Fleet` standings: 10-14.
  - The words in `Plain{Decision, Act, IfNothing}` (U2 of the channel page): 15-20.
  - The close after stable recovery (DG-07: an hour reachable from the frozen `Since`), with the closure line: 14-22.
  - Hook in `noticeSeatStandings` (`seatpresence.go:237-252`): 8-12.
  - D8: no production lines. Tests only.
  - The "and N more" form of the delivery text, moved from U1 in round 3: 6-8.
  - Stop line: past 100, hand the close to the stable-recovery retire that U3 already has (channel page D6), with the seat standing passed as its state, and drop the separate close code.

## Tests

The tests use the production compositions with injected effects: the ledger-attention run through `attentionPolicyBed` (`internal/steward/ledgerattention_policy_fixture_test.go:19-60`, real parsed goal files), the tick's surfacing step, `completeTickHealthWithDependencies`, `deliverPendingWithDependencies`, and `channel.Poll` with the fake adapter. Each test injects a clock (a fixed instant and a fixed `time.Location`), a counting transport, and a counting Mac `deliver` (R-104-m1e). No sleeps, no wall clock. Each test asserts the posts, the Mac calls, the journal rows through `notifications.Page`, and the durable state. Each mutation fails the named assertion.

Test edits allowed (Scrooge rule, bent on purpose here): new tests, plus these changes to existing tests, which the new reader makes necessary:
- two lines in the existing bed, for the projection clock (fixture rules below);
- `internal/landing/plain/units_test.go` (DG-10). It calls the private `unitsOnMain` with trailer-only output and pins the old git arguments (`units_test.go:23-28`). Its fixture becomes the SHA-bearing output (`<sha>\x00<trailers>` per commit), with the new arguments. Its damaged-trailer cases (`:18-21`) and its read-error case (`:34-37`) are kept. A repeated-unit case runs through the exported `UnitsOnMain` wrapper on a temp git repo. The dropped-unit guards (`drop_landing_test.go:88,127`, `intent_drop_rebase_test.go:159`) are kept as they are.

Test allocation (estimate): U1 has 18 new tests, about 560-680 lines, plus about 20 changed lines in `units_test.go` and 2 in the bed. U2 has 9 new tests, about 280-350 lines.

Fixture rules (DG-05):
- The bed writes every goal under `plans/goals/` (`ledgerattention_policy_fixture_test.go:47-53`). `goal.ParseTreeFiles` puts such a file in `Live`; only `records/goals/<id>.md` (or `plans/goals/done/`) reaches `Done` (`internal/goal/validate.go:110-117`, `:140-145`, `:174-177`). The new fixtures write done records to `records/goals/`.
- The bed's `ProjectAt` falls back to `time.Now()` when no time is given (`:216-219`), and the stage builder calls it without one (`ledgerattention.go:230`, `:253`). The bed's fallback becomes `b.now`, so the stage reads the injected clock.
- Replay tests revisit the exact branch they guard; decision tests vary the prose, or close the first question, so the facts match (`internal/channel/question.go:305-306`) cannot hide a missing `Decision`; duplicate-notice mutations bypass the real person-facing suppression.
- Both ingestion routes are proved, not only two actors: a local accepted move (`ledgerattention.go:539-551`, called at `:627` and `:653`) and a fetched move (`:666`).
- The done records come from the real verb. A wiring test runs `goal.Done` on a temp ledger for each actor and feeds the written files to the bed. It keeps the conclusion-authority checks (`internal/goal/verbs.go:2864-2882`): the pane case carries a proven human authority; the lane case concludes a main-origin goal claimed by the lane's own pair. A lane actor on a human-origin goal is refused, as today, and produces no notice.

- **U1**
  - `TestDoneEntersOneEventPerTransition`. Bed: base → t1 (G moves from `plans/goals/G.md` to `records/goals/G.md`, history `done actor=human:Wido`) → t2 (a priority move only). Expect: one event, at t1, `Delivered=[G rev N]`; t2 has no `Delivered`. The event is kept although claimable, pins and queue did not change. Mutation: drop the `Delivered` clause at `:286` → no event, fails.
  - `TestPaneAndLaneConclusionsEachDeliverOnce`. Records from the real `goal.Done` (fixture rules above): one goal concluded at the pane with a proven human authority, one main-origin goal concluded by the lane's own pair. Two posts, one per goal. The journal shows no other `goal-delivered` line. Mutation: filter on a human actor → the lane's post is missing.
  - `TestLaneCannotConcludeAHumanGoalAndNothingPosts`. The lane actor on a human-origin goal is refused by the existing check; no event, no post. Mutation: none in production; it guards that the fixture keeps the authority path (a fixture that skips `requireHuman` makes it fail).
  - `TestLocalAndFetchedConclusionsDeliverOnce`. One done published from this checkout (local accepted route, `:539-551`) and one fetched from another (`:666`). One post each. Mutation: drop the `Delivered` computation from one of the two callers' stage → that post is missing.
  - `TestGoalDeliveredOnceAcrossTicksAndRestart`. Tick 1: one post. Tick 2: none. Reload the stores from disk, as a restart does, and run the pass: none. Mutation: the retry pass's predicate skips `episodeSettled` (the branch that tick 2 actually runs, as at `signal_channel.go:135-142`) → tick 2 posts.
  - `TestUnreadableLaneKeepsTheEventPending` (DG-01). The injected `lane` reader answers `unknown` on tick 1, then `keeper` on tick 2. Tick 1: zero posts, the event is still in `Pending`, no episode. Tick 2: exactly one post. Mutation: treat unknown as non-keeper (mark the event) → tick 2 posts nothing.
  - `TestOnlyTheLaneKeeperAnnounces`. Three roots, the same event; the `lane` reader answers `keeper` for one root only and `other` for two. One post in total. Mutation: ignore `Ownership` → 3 posts.
  - `TestNoLaneRegisteredAnnouncesHere`. With no lane record there is one post. Mutation: `other` when unregistered → 0.
  - `TestLaneReaderKeepsOwnershipOnEveryReturn` (DG-12). The production `readLaneSilence` over a temp board home, with a registered keeper whose lane is paused, has a proof running, has an empty queue, has an open question, or has an unreadable pending store. Liveness of the running proof is injected through the `alive` callback (R-104-m1e), not read from the test process as the existing fixture does (`lane_silent_test.go:177`). The test asserts the callback was called for the running case. Every case returns `keeper`. A layout that cannot be read returns `unknown`, not `other`. Mutations: clear ownership on the empty-queue return (`:120-121`) → that case fails; the reader passes `plain.ProveSeams{}` instead of the callback → the callback-used assertion fails.
  - `TestCrashBetweenEpisodeAndMarkPostsOnce`. Open the episode, skip the mark, run the tick again so the same event reaches the open step. One post. Mutation: open without the digest lookup → 2 posts.
  - `TestSaveFailureLeavesTheEventPending`. The episode save fails. The event stays in `Pending`; the next tick posts once. Mutation: mark before the save → 0 posts.
  - `TestFailureRestartSuccessShowsOneNotification` (DG-02). The transport fails, the stores are reloaded as on a restart, then the transport succeeds. The episode has 2 attempts and 1 post. `notifications.Page` returns one row for the delivery, and the stream (`notifications.Next` from the start) yields one. Mutations: treat `Failed` as settled → 0 posts; leave `suppressFailureJournal` false for this pass → 2 rows on both surfaces.
  - `TestOrdinaryFailedNoticeStillJournals` (DG-09). A failed breach-stop post through a funnel literal with no new field journals one line with its `error`; a failed goal delivery journals none. Mutation: read the zero value as "suppress" → the ordinary line is missing.
  - `TestReopenedGoalDeliversAgain`. Done at rev 9, reopened, done at rev 12: two posts. Mutation: digest by goal only → 1.
  - `TestDeliveredTextNamesGoalUnitAndCommit`. Temp git repo: a commit with `Goal-Unit: G/U1a.1`, then the done commit, with local `main` left behind. The text equals the two D5 lines byte for byte, in the injected location. Mutation: read `main` instead of the tip → the unit clause is missing.
  - `TestNoTrailerAndUnreadableScanSayOnlyWhatIsKnown` (DG-03). A tip with no trailer gives `no unit landing is recorded at this conclusion`; a failing git read gives `its landed units could not be read`. One post each, bytes pinned. Mutations: return on the read error → 0 posts; render the none-recorded ending for the error → the bytes differ.
  - `TestUnitsOnMainStillCounts` (DG-04, DG-10). The updated `units_test.go` (SHA-bearing fixture, damaged-trailer and read-error cases kept) passes. Its new repeated-unit case, through the exported `UnitsOnMain` on a temp git repo with `goal/a+b`, `goal/a` and `goal/c`, counts 3. The dropped-unit guards pass unchanged. Mutation: the wrapper counts trailers instead of distinct units → the repeated-unit case gives 4 and fails.
- **U2**
  - `TestUnreachableSeatAsksOnceWithItsCommand`. Poll with the fake adapter. Standings: `ui` is unreachable and holds 2 goals; this steward is elected. One question post. Line 2 is `To go on: start it in its checkout: metasystem system start`. A second tick, with a different `Since` in the prose so the facts differ, posts nothing. Mutation: ask without `Decision` → a second post.
  - `TestOneStewardAsksForAnUnreachableSeat`. Three stewards' standings over the same fleet copy: one question in total. Mutation: the election ignores `This` → 3.
  - `TestUnreachableSeatClosesAfterStableRecovery` (DG-07). On the injected clock: reachable for 61 minutes gives the closure line `Closed: ui reports again.` and retires the decision. Mutation: no close → the question stays open.
  - `TestFlappingSeatAsksOnce` (DG-07). Unreachable, reachable for 30 minutes, unreachable again, with the same held goals: one question, no closure line. Mutation: retire after one reachable tick → a second question.
  - `TestSinceDoesNotSplitTheDecision`. Two readers with different `Since` values give one identity. Mutation: put `Since` in `State` → 2 questions.
  - `TestNoiseNeverReachesAPerson`. One tick each, with fixtures: `idle-backlog-dead`, one raw `degraded` verdict, a ledger move, a seat transition, an engine re-arm and a narrator noticing. Channel posts: 0. Mac calls: 0. `notifications.Page` rows: 0. Each journal line carries `withheld`. Mutation: allow the verdict owner in `personClass` → the counting Mac transport becomes nonzero (queued verdicts go to the local transport, `notify.go:284`, `:327-330`) and the person-visible journal-row assertion also fails.
  - `TestPersistentDegradationAsksOnceAndNoRawLine` (DG-06). `degraded` held for `degradedNoticeTicks` ticks (`tick.go:574`) and longer: exactly one act question (U3's `TestDegradedTickAsks` path), and no person-visible `steward: degraded` row. Mutation: withhold the ask's owner → 0 questions.
  - `TestAnAskProducesOneActNeededPost`. A unit-stop ask and a breach stop. One post each. Line 2 of each starts with `To go on: metasystem `. The journal has no other person-visible line for either. Mutation: `ForAPerson` ignores `withheld`, so the stop's withheld verdict line shows → a second visible row.

## Done criteria (from the goal record)

1. `goal done` from the enrolled pane and from the lane each produce exactly one goal-delivered notice that names the goal, the unit and the commit: `TestPaneAndLaneConclusionsEachDeliverOnce`, `TestDeliveredTextNamesGoalUnitAndCommit`, `TestOnlyTheLaneKeeperAnnounces`.
2. An ask produces one act-needed notice with its command: `TestAnAskProducesOneActNeededPost`, `TestUnreachableSeatAsksOnceWithItsCommand`.
3. The test journal shows no other line for either: the journal assertions of the same tests.
4. idle-backlog-dead and degraded never reach the person as raw lines: `TestNoiseNeverReachesAPerson`; a degradation that persists asks once, as the accepted page requires: `TestPersistentDegradationAsksOnceAndNoRawLine`.
5. Live check after landing: the next `goal done` gives one Telegram line from the landing steward, and the ui and m1e journals hold no `goal-delivered` line.

## Threat model

- **A delivery is lost.**
  - The keeper is down: the notice comes late, not never (D2).
  - The keeper cannot read its lane record: the event stays pending until it can (D2, DG-01).
  - The episode save fails: the event stays pending (D2).
  - The lane is moved to another checkout between a done and its announcement: the old keeper may already have surfaced the event, and the new keeper surfaced it earlier without announcing, so the notice can be lost. This is rare and stated.
  - The upgrade tick, as in D1.
  - A local-mode installation makes no ledger events at all (`ledgerattention.go:575-581`), so it gets no delivery notice. See Not in scope.
- **A delivery is doubled.** Only through the funnel's known crash gap and a provider timeout after success (D3). A failed attempt raises no browser notification (D3, DG-02). A ledger rewind gives a fresh event id, but the digest is goal plus revision, so it sends nothing new. A migration that bumps revisions of done goals is not an entry into done.
- **A flood.**
  - The first pass is a baseline and makes no events (`:600-613`).
  - A batch of conclusions gives one line per goal; each is one person act or one lane act.
  - Old done goals never enter done again.
- **A wrong unit or commit.** A hand landing without a `Goal-Unit` trailer gives "no unit landing is recorded"; the text never guesses a commit and never says no code landed. A damaged trailer or a failed read gives "its landed units could not be read" (D5).
- **An id in the text.** Only the goal name, the unit names, and a sha inside `git show`. `TestDeliveredTextNamesGoalUnitAndCommit` pins the bytes.
- **Two seats disagree on who asks.** For one window two questions are possible. The next ticks agree, and each question closes after the seat has reported for an hour.
- **The elected seat changes.** Its successor's question store is its own, so the same decision can be asked again (D7.3, DG-13). Deferred.
- **A flapping seat.** One question per decision; a brief reachable reading neither closes nor retires it (D7.3).
- **A person is blocked.** Never: a notice gates nothing (D9).

## Not in scope

- **Pane enrolment as a pushed ask.** It is decided only in `status` (`intent_helm.go:624`). The refusal at his terminal already names the command (`:777`). An answer over Telegram is his word without the pane, so a missing enrolment never blocks an answer he gives there. A steward reader for it is a later goal if he wants one.
- **Local-mode installations.** They need a delivery source outside ledger attention.
- **A second machine.** The host rule (D2) gives one announcer per host. With two machines each one announces. The fleet is one computer today.
- **Changing the per-sha landing post** (channel page U2). It stays as it is.
- **A free-text message verb.** None is added.

## Questions decided by Wido (2026-10-20)

- **Q1 Degraded: never, or ask once?** The goal record says degraded never reaches you. The accepted channel page (D6, row 4) asks once when a degraded tick stands, as your Word 12 asks. This page now follows the accepted page (DG-06) and asks you only to confirm. Today that would have been one question at about 00:24, and its closure line an hour after 05:36, instead of 34 lines. My advice: keep the one ask. The raw `steward: degraded` line stays withheld, as this page tests.
  **Decided by Wido, 2026-10-20 19:01 (asked with the pros and cons): ask once when the degradation persists, with the remedy; the raw degraded lines stay held back.**
- **Q2 Two lines for one finished goal?** When the lane lands a goal's last unit, the channel page's landing post says `Delivered: G: <what changed>`. When the goal is concluded, this page adds `Delivered at HH:MM: goal G is done`. A hand landing gets only the second line. My advice: keep both. The first says what changed, the second says the goal is finished, and they come from two different acts. If it reads as noise, drop the per-sha post later; this page does not depend on it.
  **Decided by Wido, 2026-10-20 19:01: keep both lines; they come from two different acts, and a hand landing gets only the second.**
- **Q3 No lane registered?** D2 lets each steward announce when the host has no lane. On a host with one checkout that is right. On a host with several checkouts and no lane, it sends one line per checkout. My advice: accept that. A host with several checkouts registers a lane in practice. The other choice, a host-wide mark file under `~/.metasystem/host` beside `landing-lane.json`, costs about 30 more lines and a lock.
  **Decided by Wido, 2026-10-20 19:01: accept one line per checkout on a host with several checkouts and no lane; no host-wide mark file.**

## Critique record

- **Round 1:** Astra, 2026-10-20 18:3x CEST, on revision 1, reading workspace u1a at 3abe884dff. Log: `/Users/wido/agentic-tools-evidence/lane-test-20261009/briefs/a-delivered-goal-and-a-needed-act-reach-the-person-design-critique-1.findings.md`. 4 material, 4 minor. Every cited file:line was re-read before folding; all held, with the two notes under DG-04 and DG-05.

| Finding | Disposition |
|---|---|
| DG-01 (material) | Folded into D2: unknown ownership leaves the event pending, a known non-keeper consumes its copy, the keeper marks only after the episode is saved. Tests `TestUnreadableLaneKeepsTheEventPending`, `TestSaveFailureLeavesTheEventPending`. Cites verified: `tick.go:544-551`, `ledgerattention.go:306-307`; the drop is at `:731-738` inside `PersistLedgerAttentionMark` (`:719-739`). |
| DG-02 (material) | Folded into D3: one visible journal notification per delivery, on success; failed attempts stay on the episode. Mechanism `noticeFunnel.journalFailures`, false for this pass only (renamed and inverted in round 2, DG-09). Test `TestFailureRestartSuccessShowsOneNotification` through `Page` and the stream. Cites verified: `notify.go:167-179`, `:424-431`; `notifications.go:87-94`, `:301-302`; `notifications.ts:161-168`. |
| DG-03 (material) | Folded into D5 with the critic's wording: "no unit landing is recorded at this conclusion" and "its landed units could not be read"; neither says no units reached main. Threat model and tests changed to match. Cites verified: `units.go:15-18`, `:25-38`. |
| DG-04 (material) | Folded into D4 and U1: `UnitsOnMain` stays as a count wrapper over `UnitsLanded`, counted in U1 (36-46 for `units.go`). The critic's callers hold; it also missed two production callers, `cmd/metasystem/intent_landing.go:182` and `intent_landing_push.go:291`, which keep compiling through the wrapper. U1 recounted to 183-223, stop line 230. |
| DG-05 (minor) | Folded into the fixture rules and tests: done records in `records/goals/` (`validate.go:110-117`, `:174-177` verified), the bed's projection clock from `b.now` (`fixture_test.go:216-219`, `ledgerattention.go:230`, `:253` verified), mutations moved to the branch actually run (`signal_channel.go:135-142`), prose varied against the facts match (`question.go:305-306` verified), the duplicate mutation now bypasses `ForAPerson`. |
| DG-06 (minor) | Folded into D8 and U2: raw degraded verdicts make no visible notice; persistent degradation asks once as the accepted page requires (`:98-115`, `:194` verified). Two separate tests. Q1 kept as a confirmation. |
| DG-07 (minor) | Folded into D7.3: seat decisions use U3's stable-recovery discipline, extended to seat standings with `healthFlapWindow` read from the frozen `Since`. Tests `TestUnreachableSeatClosesAfterStableRecovery`, `TestFlappingSeatAsksOnce`. U2 recounted to 59-88, stop line 100. |
| DG-08 (minor) | Folded into the invariant: the once-only guarantee is stated as conditional on a fixed registered keeper and its kept store, with the crash, timeout, migration and unregistered-several-checkouts limits named. D2 says silence comes from the ownership rule, not a shared mark (`alert_episode.go:111-123` verified). |
| Closing note | Folded: both ingestion routes are proved (`TestLocalAndFetchedConclusionsDeliverOnce`), and the lane fixture keeps the conclusion-authority checks (`verbs.go:2864-2882`; `TestLaneCannotConcludeAHumanGoalAndNothingPosts`). |

- **Round 2:** Astra, 2026-10-20 18:47 CEST, on revision 2. Log: `/Users/wido/agentic-tools-evidence/lane-test-20261009/briefs/a-delivered-goal-and-a-needed-act-reach-the-person-design-critique-2.findings.md`. 3 material, 2 minor. Each cite re-read before folding.

| Finding | Disposition |
|---|---|
| DG-09 (material) | Folded into D3 with the critic's sentence: `suppressFailureJournal bool`, zero value keeps today's journaling, set true only by the goal-delivered pass. Every "true by default" wording removed. Test `TestOrdinaryFailedNoticeStillJournals`. Cites verified: `tick.go:211`, `:319`, `notify.go:172-177`, `alert_episode.go:276`, `:297`. |
| DG-10 (material) | Folded into the Tests preamble: `units_test.go` is updated for SHA-bearing output (it calls private `unitsOnMain`, `units_test.go:23-28`, verified), damaged-trailer and read-error cases kept, a repeated-unit case added through the exported wrapper, dropped-unit guards kept. The "only new tests" rule bends here, said so. Test allocation recounted. |
| DG-11 (material) | Folded into U1: at the stop line only the "and N more" formatting may be cut. The scan-failure ending stays; I chose that over a neutral text for both cases, because it costs about 5 lines and does not hide a broken read. A further overrun splits into a U1a with a neutral `Done at HH:MM: goal X is concluded.` that claims nothing about landings. Cites verified: `units.go:15-18`, `:25-38`. |
| DG-12 (minor) | Folded, and it found a defect in revision 2: `Known: false` cannot mean "ownership unknown", since a keeper's read failures at `lane_silent.go:107-125` also give it, and the ordinary returns at `:97-128` carry no ownership. D2 now uses a three-valued `Ownership` set on every return, with a layout error as `unknown` (`lane/agent.go:587-590`). Test `TestLaneReaderKeepsOwnershipOnEveryReturn` with the empty-queue mutation. U1 recounted: this site is 10-14, was 6-8. |
| DG-13 (minor) | Folded into D7.3 and the threat model: seat asks are deduplicated only within the elected checkout's store (`question.go:128-143`, verified); a change of elected steward can ask again; continuity across checkouts deferred. |

- **Round 3:** Astra, 2026-10-20 evening, on revision 3. Log: `/Users/wido/agentic-tools-evidence/lane-test-20261009/briefs/a-delivered-goal-and-a-needed-act-reach-the-person-design-critique-3.findings.md`. 1 material, 2 minor. Each cite re-read before folding.

| Finding | Disposition |
|---|---|
| R-104 seam (material) | Folded into U1 and the ownership test with the critic's replacement: `readLaneSilence` threads the existing `plain.ProveSeams.Alive` callback; the test injects liveness, asserts the callback was used, and fails when the reader falls back to default seams. Verified: `lane_silent.go:106` passes `plain.ProveSeams{}`; the default reaches `identity.KernelProber{}` at `plain/prove.go:313`; `ProveSeams.Alive` is at `prove.go:250`; the existing fixture probes the test process at `lane_silent_test.go:177`. Wiring 6-8 lines counted in U1; the "and N more" formatting moved to U2 to keep the stop line at 235. |
| DG-14 (minor) | Folded: the noise mutation now names the counting Mac transport and the person-visible journal-row assertion. Verified `notify.go:284`, `:327-330`. |
| DG-15 (minor) | Folded into the Problem text: landing and ui journaled the same six moves, m1e journaled nothing that day, so the three-root fixture proves the third steward's silence. Verified against `analysis/notices-2026-10-20.md`. |

Stop at round 3 (2026-10-20, steward in Wido's name): converged 4 → 3 → 1 material; the last was a proof seam folded with its verified replacement; no decision changed since revision 2.
