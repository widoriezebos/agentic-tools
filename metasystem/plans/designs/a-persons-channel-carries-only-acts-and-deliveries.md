# A person's channel carries only acts and deliveries

- Kind: design
- Id: 01M5BD000000000000000CHN01
- Status: accepted
- Goals: a-persons-channel-carries-only-acts-and-deliveries

Tier 2, revision 3 (final text after one Astra round). Read in `/Users/wido/LocalStorage/GitHub/agentic-tools-m1e` at 7f0f5f3e0e on 2026-10-19, 13:10-14:30 CEST. Rulings relied on: R-104-m1e (artificial clocks) and R-129-ui (a request that already holds is not asked again, `internal/channel/question.go:252-257`).

Wido's binding words, verbatim as given to this page:
- **Word 10 (2026-10-19 13:05):** "I also need much better telegram messages. The current ones are technical blurps that don't mean anything to me. I need plain english ones that I can act on, or I really care about (as in: goal delivered). The rest is rubbish."
- **Word 11 (about 13:40, relayed by the coordinator):** "Mac banners follow the SAME two classes as Telegram (acts and deliveries); the full record stays in `alert list`; nothing else is pushed anywhere a person is addressed."
- **Word 12 (about 13:40, relayed):** "A machine that cannot heal itself does not stay silent: a health role with no remedy left, or a stuck unit nothing advances, becomes a QUESTION to him (an act): one plain sentence naming the subject in his words, what he can do, and what happens if he does nothing; a bound prevents asking twice for the same state."
- **Earlier rules:** 2026-10-02 quiet channel; 2026-09-30 every message human-readable; 2026-09-17 local time for people.

## Problem

No single place asks whether a message is his. Below, every route to a person is traced as producer → decision → transport → reader.

- **The steward's funnel.** `deliverNoticeWith` (`internal/steward/notify.go:140`) has three entry points:
  - episodes, through `submitEpisode` (`alert_episode.go:560`);
  - the pending queue, through `deliverPendingNotification` (`notify.go:289`), which the runner drains at `runner.go:451`;
  - the provider-stale line (`tick.go:310`).

  The funnel never refuses. Its result has two values: `nil` means submitted and an error means failed (`alert_episode.go:566-574`). The queue marks the intent notified before it removes the item (`notify.go:298-301`). A crash between the send and the saved result can deliver twice; the code says so at `alert_episode.go:543-546`. The completion time (`alert_episode.go:562`) and the journal time (`notify.go:380`) both read `time.Now`.
- **The caller picks the transport.**
  - To the channel (Telegram): seat-idle (`alert_episode.go:306`), spend (`:662`, via `signalTransport`, `signal_channel.go:65`), breach stops (`signal_channel.go:153,180`), lane silence (`lane_silent.go:269`).
  - To the Mac notifier (`deliver`, `notify.go:146`): health (`alert_episode.go:532`, called from `tick.go:715`), patterns, every queued line, and narration (`narrate.go:336` puts it in the queue).
  - Patterns include the trunk-red defect (`health.go:990`) and the stuck unit (`stuck_units.go:99-110`).
  - `metasystem.steward.notify-command` (`notify.go:49`) is empty in the m1e, ui and landing checkouts, so the Mac path runs `osascript`.
  - Premise corrected: health lines reach a Mac banner, not Telegram. A Telegram path for them is a hypothesis; I did not find one.
- **The journal is a third route.** Every attempt is written to `notifications.jsonl` (`notify.go:375`). The browser reads it only through `ForAPerson` (`internal/ui/notifications/notifications.go:301`), which drops only lines starting `HEALTH `. That same filter serves the history, the stream, the unread counts (`:87,142,196`) and the Decisions page (`internal/ui/httpd/decisions.go:162`). Each line that arrives on the stream raises a toast (`internal/ui/web/_app/src/notifications/notifications.ts:161`). So a pattern, spend or narrator line ends up as a browser toast.
- **The channel package posts on its own.**
  - Questions: `question.go:325`, `poll.go:100`, `targeted.go:66`.
  - Receipts for his answers: `poll.go:195,203`.
  - Thread notices: `inbox.go:447`.
  - Closure lines `closed: <because>`: `question.go:639`.
  - Split asks: `split.go:41`.
  - Landings: `landed.go:73,114`.
  - `channel status --post` (`channel_verbs.go:146`). Its verb admits both audiences (`cmd/metasystem/intent.go:127`), so an agent can post status without being asked.
  - The lane keeper's decisions are already questions (`internal/landing/plain/policy_question.go:345`).
- **States where progress needs a person, and none is asked.**
  - Dead roles that ran out of remedies: `NoLawfulRemedy` or `AutoHealEnded` (`health.go:938-942`).
  - Health that stays unreadable. Escalation is only assigned to dead roles (`health.go:927-931`); unknown only counts (`ConsecutiveUnknown`).
  - An abnormal revival that has run out of attempts. It cancels the intent and queues `steward: <reason>` (`revive.go:578-585`). Health then reads alive, because no seat step is due (`health.go:1353-1354`).
  - A degraded tick, which queues a verdict line (`tick.go:796-810`; 7 callers).
  - A stuck unit (`stuck_units.go:96`; `launch/unit_stuck.go:66-102`).
  - Remedies that need a person (`remedy.go:56-64`).
  - The steward has one existing ask of this kind, a machine question (`recovery_request.go:155`). It shows a launch id and a UTC time (`:154-156`).
- **Ids reach a person.**
  - Lane questions print `question <id>` (`question.go:492`).
  - Ordinary questions print their facts verbatim (`question.go:496-498`).
  - Remedies name stop identities (`remedy.go:63`).
  - Real lines: `HEALTH unhealthy — steward-runner=alive (runner pid 110 is attempting generation 220 ...`; `SPEND CROSSED day-2026-10-03.tokensx4 ...`; `... metasystem work stop j1:20261014t065911-...`.
- **Landings lose their goal.**
  - The goal route lands through `landpath` (`cmd/metasystem/intent_delivery.go:340-349`). Its callback posts first, with only the delivered sentence and nothing about the goal (`landing_path.go:535-538`, `landpath/land.go:579-582`).
  - `noteLanded` knows `goalID` (`landing_gate.go:190`), but its later post is deduplicated by sha.
  - The push renderer drops hand-ins that have no sentence (`intent_landing_push.go:166`).
- **The records** (alert stores under `metasystem/artifacts/agents/steward/alerts`, 13:0x):

  | Store | Episodes | Breakdown |
  |---|---|---|
  | landing | 84 | health 38, lane-silent 15, pattern 16, breach-stop 8, seat-idle 7 |
  | ui | 209 | seat-idle 121, health 75, spend 6, breach-stop 4, pattern 3 |
  | m1e | 470 | health 264, seat-idle 128, spend 77, pattern 1 |

  None are acknowledged. By class: 51 acts (12 breach stops, 39 questions), 38 deliveries (landed shas), and about 750 other episodes. The digest is the dedup key (`alert_episode.go:484-494`) and it moves with the finding, so the 264 health episodes on m1e repeat one runner.

## Decisions

- **D1 Two classes reach a person.**
  - **(a) `act`, progress waits for him.** It covers questions from any owner, breach stops (`signal_channel.go:102`), and the spend-pause notice (see the Amendment section). It also covers the escalations of D6, which are asked as questions, and the closure line of a machine question that D6 retires.
  - **(b) `delivery`, a goal or change reached main** (`PostLanded`, `landed.go:62`).
  - **In-thread replies to his own act.** The `recorded:` and `not recorded:` receipts (`poll.go:195,203`) and the thread notices (`inbox.go:447`) belong to class (a).
  - **Status.** `channel status --post` posts only when a person runs it at an enrolled terminal (`intent.go:127` gains the person proof that `settings set` uses, `intent_work.go:2791-2795`).
- **D2 Everything else is withheld from every route that addresses a person.** That means Telegram, the Mac banner, and journal readers with their toasts. Withheld: health and pattern lines, seat-idle, spend crossings, lane silence, narration, and the `steward:`/verdict queue lines. They stay in `alert list`, `status` and the journal file. There is no digest line and no setting to widen this; the widening setting from revision 2 is dropped (PC-06).
- **D3 Wording.** Plain English, local time on the injected clock, and no ULID, uuid, sha, pid, generation, digest or job id outside a command. An id may appear inside the one command when that verb needs it (for example `metasystem question answer Q`). Identities stay in the records.
  - **A notice** has three lines: `<subject in his words> <what happened> at <HH:MM>.` / `To go on: <command or answer>.` / `If you do nothing: <consequence that the code makes true>.`
  - **A question** has the same three lines plus one line per option. Its facts, recommendation and budget move to `question show`. The text comes from a new `AskRequest.Plain{Decision, Act, IfNothing}` that every producer fills. If a producer does not fill it: `<subject> needs your answer.` / `To answer: metasystem question answer Q` / `If you do nothing: this question stays open.`
  - **A closure line:** `Closed: <plain reason>.` Every rendering is built once and stored, so a retry posts the same text.
  - **Consequences used in this slice:**
    - breach stop: "it stays stopped; nothing resumes it" (`signal_channel.go:110-111`);
    - stuck unit: "the unit keeps running; it may still finish, and this question closes if it does";
    - a role with no remedy left: "it stays down; this question closes when it has been well for an hour";
    - revival out of attempts: "the steward does not try this restart again" (`revive.go:579`, the intent is cancelled);
    - degraded tick: "the steward retries every tick".
  - **Examples.** Breach stop, landing 2026-10-19 10:25 UTC: `Goal asks-clear-on-their-effect stopped at 12:25: it used its whole budget.` / `To go on: metasystem goal resume asks-clear-on-their-effect` / `If you do nothing: it stays stopped; nothing resumes it.` Delivery, from the landing queue (2026-10-11): `Delivered: shared-state-in-the-ledger: the person's terminal enrollment lives once per machine, shared by every install, so acts from any checkout agree about who is enrolled.`
- **D4 One result contract.** The funnel returns `Submitted`, `Withheld` or `Failed` (`Failed` carries the error).
  - **`Withheld`** is decided before an attempt is appended. The episode gets `TransportResult: TRANSPORT_WITHHELD` and no attempt, so nothing claims a delivery. The journal line carries `withheld: <class>`. A queue item is settled without delivery: it is removed, and `Intent.Notified` stays false. That field is legacy and never gates launch (`intervene.go:60`), so this is MIN-01's disposition.
  - **`Submitted`** is settled. No repeat follows once the result is saved. The existing crash gap stays as the only known source of a duplicate (`alert_episode.go:543-546`); a timeout after a successful remote post is a second one.
  - **`Failed`** retries on the next pass.
  - **One helper.** Every "already sent?" check reads `episodeSettled` (submitted or withheld): `alert_episode.go:299,524,657`, `signal_channel.go:136,177`, `lane_silent.go:266`, `pattern_episode.go:204`.
  - **One clock.** The funnel takes the clock, which replaces `time.Now` at `alert_episode.go:562` and `notify.go:380`.
- **D5 One decision site.**
  - The decision is `personClass(Notice) (class, allowed)`. It is called from the funnel's three entry points and decides by owner. An owner it does not know is withheld.
  - `ForAPerson` (`notifications.go:301`) also returns false for a journal line marked `withheld`. It is the only predicate the interface uses (`:87,142,196`; `decisions.go:162`), so history, stream, counts, Decisions page and toasts agree.
  - `alert list` line 1 adds `(kept here; not sent to a person)` (`intent_alert.go:202`).
- **D6 A machine that needs a person asks once per decision.** States are classified by whether progress needs a person, not by which producer raised them. One table, `needsAPerson`, lists them:
  1. a dead role with `NoLawfulRemedy` (standing) or `AutoHealEnded`;
  2. a role unknown for `healthFailureLimit` (5) ticks with cause `unreadable`;
  3. a revival out of attempts (`stopAbnormalRevival`);
  4. a degraded tick (7 callers);
  5. a stuck unit;
  6. a remedy cause that a person must clear (`remedy.go:56-64`; BO-2 classifies every cause).

  How it works:
  - **Skipped when another owner already asks:** lane policy, unit stops (`intent_unit_stop.go:336`, `intent_unit_questions.go:197`), recovery, process change, breach stop.
  - **Exempt:** holds a person ordered. That covers the helm (`revive.go:265`), his own `work stop`, and his own resume.
  - **The ask.** `channel.AskOrFind` with `About: "machine"` and a structured, machine-only `Decision{Kind, Subject, State}`, separate from the wording. For health the identity is the role plus every `RemedyFact` field (`health.go:132-140`). For trunk red it includes the whole set of unowned incidents (`trunkred.go:45` carries only the first: BO-6). For a stuck unit it is goal, unit, run record, round and stuck step launch (`unit_stuck.go:60-102`). AskOrFind matches on `Decision` the way it matches on `Recovery` (`question.go:298-307`), over open and closed questions that have not been retired. The question record is the durable bound, so a restart between asking and recording cannot ask twice.
  - **Recurrence.**
    - A question stays while it is unresolved, through transient recovery.
    - Unknown states change nothing.
    - An answer or a closure while the state persists asks nothing.
    - Stable recovery means the role is alive with no failure episode inside `healthFlapWindow` (1 h, `health.go:173,858`), or the unit is no longer stuck. Then the steward closes the question with a plain closure line and retires the decision. Only after that can the same decision ask again.
  - **Who asks when the steward cannot tick.** A degraded tick still queues (`tick.go:799`) and the runner drains the queue (`runner.go:451`), so it asks. If the runner and the watcher are both down, nothing on this machine can ask. That is a residual: this slice adds no outside watchdog.
  - **A Mac-only installation** (no channel; `loadSignalChannel` returns not configured): each new open question raises one act banner through the funnel, keyed by question id.

## Amendment required of the stop-boundary design

`plans/designs/a-stop-waits-for-the-unit-boundary.md` changes as follows. Its owner folds this in its next revision (BO-9).
- **D6 there (`:84`).** The `spend-pause` notice is class `act`, in this page's three-line form. Routine spend crossings are withheld.
- **D8 there (`:97`).** "Its crossing still notifies" for exempt seats becomes: the crossing is withheld and the seat reads `exempt: person-driven` in `status`.
- **D9 there (`:99-104`).** A `DEFERRED` breach whose unit is still progressing is status only. The breach-stop notice fires once, at the real stop. This page owns its wording; that design owns its facts ("after finishing its unit", the overrun, the reason).
- **One identity.** While a goal has a breach-stop episode or a `DEFERRED` route open, D6 here raises no stuck-unit ask for it. The stop is the single ask.
- **Cost.** The stop notice shows `elapsedOverrun` (goal clock, `:119`) as meaningful cost. Diagnostic minutes never enter a `Decision` and never enter a notice.

## Units

Inventory: the sites in Problem and Decisions (grep of non-test Go, 13:1x-14:20). 11 test files name the transport result or the fixture log; those that expect a health or pattern send change their expectation. Order: U1, then U2, then U3. Withholding switches on only in U3, once the words and the asks exist (PC-09).

- **U1 Groundwork: result contract and decision identity (D4, D6 identity). About 112 production lines, ~150 test lines. Groundwork only: every class is still allowed, so behaviour is unchanged.**
  - Funnel outcome type and return, ~20.
  - `submitEpisode` decides before the attempt; `TRANSPORT_WITHHELD`, ~20.
  - Queue settle without delivery, ~12. `tick.go:310` caller, ~3.
  - `episodeSettled` and its seven sites, ~12.
  - Clock injected at `alert_episode.go:562` and `notify.go:380`, ~15.
  - `Question.Decision`, AskOrFind matching and persistence, ~30.
- **U2 The words (D3, PC-10). About 205 production lines, ~250 test lines. Usable alone: every act and delivery becomes readable.**
  - `AskRequest.Plain` and the three-line renderer, replacing `question.go:444-519` for posts, ~70.
  - Nine producers fill `Plain` (`policy_question.go:347`, `drift.go:137`, `check.go:143,315`, `setting.go:139`, `recovery_request.go:155`, `intent_unit_stop.go:336`, `intent_unit_questions.go:197`, `channel_verbs.go:244` with new `--decision`/`--if-nothing` flags), ~45.
  - Closure text, ~5. Breach-stop notice, ~8.
  - One landing payload `{sha, goals[{goal, sentence, intent}]}` built before the first post, ~77:
    - `LandRequest.Goal` set on the carried and staged routes (`intent_delivery.go:340`, `intent_land_staged.go:219`), so the first post (`land.go:582`) carries the goal. `noteLanded` posts the same payload and stays deduplicated by sha.
    - The push keeps hand-ins without a sentence, using the goal's intent (`goal.go:51`), or the bare goal name if the goal cannot be read.
    - The cap: at most three lines, one per goal as `<goal>: <what changed>`. With more than three goals: two such lines, then `and N more: <names>; metasystem landing status shows each`.
- **U3 Activation: withhold and ask (D1, D2, D5, D6). About 217 production lines, ~350 test lines. Usable alone: a person receives only acts and deliveries, all plain, and a broken machine asks.**
  - `personClass` table and the funnel gate, ~25.
  - `needsAPerson` table, the ask and the stable-recovery retire, ~100.
  - Hooks: stuck ~15, revival ~10, degraded ~10.
  - Mac-only question banner, ~30.
  - `ForAPerson` and the `withheld` field, ~8.
  - `alert list` line, ~4. Person-only `status --post`, ~8.
  - Spend-pause class, ~2.

## Tests

All tests run through the production compositions with injected external effects, never by stubbing every seam:
- the tick: `completeTickHealthWithDependencies`;
- the queue: `deliverPendingWithDependencies`, which `DeliverPending` uses;
- the poll: `channel.Poll` with the fake adapter (`phase.go:61`, `internal/channel/fake`);
- landing: `landCarriedWithOwners` and the push composition.

Each test injects a clock, a counting Mac `deliver` and a temp repository (R-104-m1e). Each test asserts posts, Mac calls, journal visibility through `notifications.Page`, and durable state. Each mutation fails exactly the named assertion.

- **U1**
  - `TestFunnelOutcomeContract`: submitted, failed and withheld cases; check attempts (1/1/0), episode state, journal field, injected times. Mutation: withheld returns nil, so the state assertion reads SUBMITTED and fails.
  - `TestQueueWithheldSettlesWithoutNotified`. Mutation: call `markIntentNotified` on withheld, so `Notified` is true and fails.
  - `TestNoRepeatAfterDurableSuccess`: two passes, one transport call. Mutation: `episodeSettled` ignores submitted.
  - `TestTransportFailureRetriesOnce`: one failed attempt, a restart, then one success. Mutation: treat failed as settled.
  - `TestDecisionMatchesAcrossProse`: same Decision with different facts gives one question; different Decisions with the same facts give two. Mutation: drop the Decision clause in matching.
- **U2**
  - `TestPostedQuestionTextIsPlain`: Poll over lane-stop, recovery, unit-stop, process and budget questions. At most 3+options lines, no id outside the command, "If you do nothing". Mutation: restore `question.go:492`.
  - `TestQuestionFallbackWording`. Mutation: the fallback emits `facts[0]`.
  - `TestQuestionRetryPostsTheStoredText`: the provider fails, then succeeds; the two texts are equal. Mutation: re-render on retry with the legacy renderer.
  - `TestClosureLineIsPlain`. Mutation: post the raw `because`.
  - `TestBreachStopNoticeIsPlain`: local `HH:MM`. Mutation: format in UTC.
  - `TestCarriedLandingPostsOnceWithGoal`: `landCarriedWithOwners`, then `noteLanded`; one post that names the goal. Mutation: leave `LandRequest.Goal` unset.
  - `TestPushKeepsGoalsWithoutSentence`. Mutation: skip empty sentences.
  - `TestFourGoalsOnePushOnePost`: one post. Mutation: post per goal, so the post count becomes 4.
- **U3**
  - `TestOnlyActsAndDeliveriesReachAPerson`: one tick with health, spend, seat-idle, lane, narrator, verdict and breach fixtures. Channel posts: the breach stop only. Mac calls: 0. Page rows: only the act. Mutation: class spend as act.
  - `TestUnknownOwnerIsWithheld`. Mutation: an unknown owner is allowed.
  - `TestWithheldNeverToasts`: `notifications.Page`/`Next`. Mutation: `ForAPerson` ignores `withheld`.
  - `TestNoRemedyLeftAsksOncePerDecision`: the reason's digits change each tick; one question, one post. Mutation: build the ask without `Decision`.
  - `TestFactsAreStable`, separate from suppression: the rendered Plain is equal across ticks. Mutation: put the minutes in Plain.
  - `TestTwoIncidentsSameProseAskTwice`. Mutation: the identity uses the first incident only.
  - `TestStuckStepsAndRunsAreDistinct`: two stuck steps in one round give two asks; a new run of the same unit gives a third. Mutation: drop the step launch from the identity.
  - `TestRecurrenceSequences`: answered, unanswered, closed, unknown, flapping 30 m alive, stable 61 m, and a restart between ask and record. Mutation: retire after one healthy tick.
  - `TestUnreadableHealthAsks`. Mutation: count unknown as healthy.
  - `TestExhaustedRevivalAsks`. Mutation: remove the hook.
  - `TestDegradedTickAsks`. Mutation: class degraded as steward.
  - `TestPersonHoldAsksNothing`: helm and his own stop. Mutation: hook `holdForHelm`.
  - `TestMacOnlyQuestionBannersOnce`: two ticks, one Mac call. Mutation: drop the question-id key.
  - `TestStatusPostNeedsAPerson`. Mutation: drop the proof.
  - `TestStuckUnitYieldsToTheStop`: a breach stop is open, so there is no stuck ask. Mutation: remove the guard.

## Threat model

- **A real act is lost.** U3 switches on only after U2. Unknown owners are withheld, but the behavioural guard and the `needsAPerson` table test (BO-2) fail on an unclassified cause.
- **A goal-done storm** gives one post per push, within the cap.
- **A flapping machine** asks one question per decision and is retired only after an hour of stable health.
- **The person-driven seat:** his own holds ask nothing.
- **A message that names an id anyway:** the posted-text tests fail. At runtime, wording never withholds an act.
- **Runner and watcher both down:** nothing on the machine asks (residual).

## Not in scope

An outside watchdog. A goal `done` record as the delivery source: I did not read it; the hand-in sentence and the goal intent are used.

## Brief obligations

These are what the code could not settle here.

- **BO-1** Give each of the 7 `degradedTick` callers a constant cause for its `Decision`.
- **BO-2** Classify every `RemedyCause` (`health.go:145+`) as a person's or the machine's; a test fails on an unclassified cause.
- **BO-3** Confirm that the carried route posts at `land.go:582` before `noteLanded`, and set `LandRequest.Goal` on every goal route.
- **BO-4** Choose the scope of the Decision match: `listQuestions` (installation) or `WalkQuestions` (machine).
- **BO-5** Give every `HealthRole` plain words, with a test that each role has them.
- **BO-6** The trunk-red identity carries the whole unowned set.
- **BO-7** Check how the Telegram provider behaves on a timeout after success; keep the duplicate as a stated limitation.
- **BO-8** Verify each consequence sentence in D3 against its producer, for revival and recovery in particular, and test it.
- **BO-9** Fold the amendment into the stop-boundary design.

## Critique record

- **Round 1:** Astra, 2026-10-19 about 14:00 CEST. Log: `/Users/wido/agentic-tools-evidence/lane-test-20261009/analysis/persons-channel-critique-1.log`. 11 material, 1 minor. Verdict: acceptable at the stop if folded without a second round. All findings are folded into revision 3:

| Finding | Disposition |
|---|---|
| PC-01 | Folded. Problem traces producer → decision → transport → reader. D1 classes receipts, notices and closures; D2/D5 cover the journal, `ForAPerson` and toasts; `status --post` is person-only; narration (queue) and keeper (question route) are named. |
| PC-02 | Folded. D6 `needsAPerson` table covers revival, unreadable health, degraded tick, stuck unit and person causes; other owners are skipped; holds are exempt; who asks when the steward cannot tick; Mac-only banner. BO-1, BO-2. |
| PC-03 | Folded. D6 structured `Decision` separate from the prose and carried through AskOrFind. Tests: two incidents, two steps, a new run. BO-6. |
| PC-04 | Folded. D6 recurrence with a 1 h stable window and the question record as the bound. `TestRecurrenceSequences`. |
| PC-05 | Folded. D3 question presentation is in scope (U2); ids only inside a command; the posted text, fallback and retries are tested; consequences are truthful. BO-8. |
| PC-06 | Folded. The widening setting is dropped. |
| PC-07 | Folded. D4 result contract; the crash gap is kept; failure and restart are tested. BO-7. |
| PC-08 | Folded. Amendment section. BO-9. |
| PC-09 | Folded. Recut: U1 groundwork, U2 words, U3 activation; withholding waits for the words and the asks. |
| PC-10 | Folded. U2 single landing payload, goal on the first post, sentence fallback, cap with what changed. BO-3. |
| PC-11 | Folded. Tests run through production compositions with an injected clock; fact stability is tested apart from suppression; the behavioural guard replaces the static Post-site list. |
| MIN-01 | Folded. D4: `Notified` stays false for withheld; it is legacy and never gates launch. |

## Accepted at the stop (steward in Wido's name, 2026-10-19 14:20 CEST)
Astra round 1 on revision 2: 11 material, 1 minor, all folded from the code into revision 3 (persons-channel-critique-1.log); Astra's own verdict: acceptable at the stop without a second round once folded. Wido's words 10 (13:05 Telegram), 11 (13:40 banners follow the same two classes) and 12 (13:40 a machine that cannot heal itself asks) are the binding block. Remaining obligations BO-1..BO-9 go into the unit briefs. The formal `metasystem design review` runs at the next engine start; a material finding there reopens the page. The amendment this page requires of plans/designs/a-stop-waits-for-the-unit-boundary.md is recorded on that page the same day.
