# Blocked agent asks the human

- Kind: design
- Id: 01M3YB2G3QF059VC2HXQ0VAP13
- Status: accepted
- Goals: blocked-agent-asks-the-human

Critique: Codex Astra, chain design-critic-7c1b3f32b7c2d075001609eb, closed at round 3 on 0 material findings (material per round 6, 2, 0; the round-2 divergence signal was handled by the seat as the guided agent, by subtraction per the rabbit-hole list). Accepted under Wido's 2026-10-02 plan: this goal is what lets the machinery switch on.

Wido, 2026-10-02: a blocked agent is not a failure; it means the human should get involved, easily, and unblock it by a decision or otherwise. That includes the landing agent, and it is what lets the machinery switch on. Facts below were read at `135bd6a7a` unless marked *(inferred)*; the inventory is `agentic-tools-evidence/escape-hatch-20261002/inventory.md`.

## Threat model and rabbit-hole risks (read before critiquing)

Our own agents and operators make mistakes: an agent asks too often or too vaguely, forgets to wait, or waits in a way that burns turns; an operator misses a message. Nobody attacks. The channel's existing TOTP check on answers stays as it is.

| Rabbit hole | Mitigation |
|---|---|
| Answering from the UI | Deferred (step 2). Step 1 answers only in the channel thread, as today (`ui/decisions/decisions.go:120-128`). |
| Redesigning the question record or the TOTP check | Two optional fields are added: `about`, and `lineage` (the asker's, which `AskRequest` already carries); nothing else in the record, the poll or the TOTP check changes. |
| A general alert-routing framework | Only the named signals go to the channel, each at its own producer. No routing rules, no per-alert config. |
| Turning a channel answer into an automatic authority act | Nothing acts on an answer by itself. Answers reach the agent that asked; steward signals are notices with the one command a person (or the agent the person tells) runs. |
| An agent that asks too often | The rule's threshold: ask only after the one obvious next step was tried and failed. Identical open questions are already de-duplicated (`internal/channel/question.go:219`). |
| Goal-less questions turning into a new store | They are ordinary question records with `about` set; same directory, same poll, same list. |
| Rebuilding all of the accepted gateway design (about 2,000 lines) for two installations on one computer | Decision 8 builds only receive, commit, confirm and local matching; every other section is listed under Deferred. |
| A gateway, a lease or a leader | Ruled out by Wido (FCG-PRINCIPLE-01): every installation polls, the ledger push race decides, and a 409 is the ordinary sound of two pollers. |
| Per-installation bots | One bot token for the fleet (Wido's standing ruling); no per-installation token, chat or config step. |
| Rewriting the Stop hook | One predicate, "a question this session asked is open", read by the branches that would otherwise refuse. |

## Step 1 and what it reuses

Reused unchanged: `metasystem question ask|wait|show|list|withdraw` (`cmd/metasystem/intent_process.go:330-409`); the question record and its `state` (`internal/channel/question.go:43-61`); Telegram delivery and TOTP-checked answers by long-poll (`internal/channel/poll.go`); the durable wait that needs no model turns (`cmd/metasystem/channel_verbs.go:226-327`); alert episodes that submit once (`internal/steward/alert_episode.go:474`). No new transport, no new store.

## Decision 1: the rule and where it goes

Rule text (one paragraph, identical everywhere it appears):

> **When you are blocked, ask.** If a refusal or failure stops your work and the one obvious next step (the command the refusal names, or one retry) did not clear it, do not loop, guess or stop silently. Run `metasystem question ask GOAL --question "<the decision you need>" --fact "<refusal line 1>" --fact "<what you tried>" --option "<label>: <consequence>"...` (with no goal: `--about lane` or `--about machine` in place of GOAL), then `metasystem question wait channel:Q` in the background and end your turn. The answer is a decision, the person having done the act, or a bypass; act on it. If the block clears meanwhile, `question withdraw Q`. Work on nothing the answer could invalidate.

Where: `metasystem/AGENTS.md`, beside the "Ask first about choices..." sentence; `metasystem/skills/landing-agent/SKILL.md` as Case 8 ("Blocked outside cases 1-7: ask with `--about lane`, then end your turn; the keeper holds you until it is answered"); seats take AGENTS.md, so no seat brief text is added. Delegates get a different line, in `internal/protocol/templates/brief.md` and `design-brief.md`: "When blocked, stop and return `BLOCKED:` with the refusal's first line, what you tried and the decision needed; your seat asks." (Decision 6 says why.)

## Decision 2: how an agent waits

A seat runs `question wait` as a background task and ends its turn. The wait polls the channel and returns the answer text with no model turns; Claude Code re-invokes the session when the background task exits.

"Pending" is read from the question record, never from ledger markers: a question is pending while its record's `state` is `open`. Answered, closed and withdrawn are all terminal (`question.go:446`, `poll.go:242,415`, `targeted.go:81`), so a withdrawn question stops holding anything with no new marker. Two readers use it:

1. **The Stop hook: one predicate.** `questionPending()` is true when an open question record names this session's machine and owner lineage. It reads the question records, not the registered waits, because a wait on a budget-stopped goal is dropped before any branch sees it (`internal/goal/project.go:741-748`, `turnverdict.go:705-710`, `:1013-1015`); the machine-and-lineage match keeps it to the asking session. It is read in one new case placed before the open-plan case of the Stop switch (`internal/goal/turnverdict.go:1774`), which therefore also covers the claimed-goal revision branch below it (`:1812`, `:1824-1826`): the case prints `WAITING: question Q` and does not block. The idle-backlog branch (`:1284`) reads the same predicate beside `hasWorkInFlight`. `suppressOpenWork` and `hasWorkInFlight` stay unchanged. The goal package cannot import `internal/channel` (channel imports goal), so the Store gets an `openQuestions func() []OpenQuestion` seam (goal, machine, lineage), wired by the command to `channel.WalkOpenQuestions`.
2. **Who may take the goal.** `GoalFacts` gains `AskedOpen` (an open question names this goal), filled where the facts are built (`internal/goal/project.go:440,485,490`) from the same seam. Idle continuation (`turnverdict.go:1320,1338`) and the seat ladder (`internal/steward/seat_ladder.go:77,111`) treat `AskedOpen` like a pending human word. `humanword.go` is unchanged, and the `ASKED` marker stays a human-readable note only.

The lane needs no wait: its Stop is always allowed (`internal/hooks/runtime_hook_stop.go:129-136`). The keeper gains one hold beside the proof and outage holds (`cmd/metasystem/landing_agent.go:184-192`): an open question with `about: lane` on this machine holds the relaunch. When it is answered or withdrawn the hold lifts, and the relaunched agent reads the answer with `question show`.

## Decision 3: a question without a goal

`question ask --about lane|machine` (no GOAL) writes a record with `goal: ""` and `about` set; `machine` is already recorded. The record, delivery, poll and list serve as they are; only the goal-ledger steps are skipped, since there is no goal file to mark. `AskOrFind` (`question.go:205`) accepts an empty goal when `about` is set and kind is `other`; `goal.Asked` (`question.go:258`) and `goal.Answer` (`poll.go:357`) are skipped and the answer goes straight to phase `recorded`. A goal-less `question wait` registers the existing `human` wait kind (`cmd/metasystem/wait_register.go:93-101`) and polls the record until `answer` is recorded. Rejected: naming an unrelated goal (misleads its seat), or a per-machine pseudo-goal (a new ledger object).

## Decision 4: steward signals to the channel

Each signal is a two-line notice (what happened in plain English, then the one command), posted once per episode through the configured provider (`phase.Load`, `Provider.Post`) at the signal's own producer; with no channel configured, the producer's current local path is unchanged.

| Signal | Producer today | Step 1 | When |
|---|---|---|---|
| Seat idle with ready work | `RecordSeatIdleIncident` saves the episode without submitting (`internal/steward/alert_episode.go:236-277`); the separate human alarm is queued only when no continuation could be prepared (`cmd/metasystem/goal.go:784-790`) | the new episode is submitted with the channel post as transport; the queued alarm stays for its own case | third idle refusal (existing) |
| Lane silent | none | a new `lane-silent` episode in the steward tick beside the spend episodes (`internal/steward/tick.go:546-551`) | queued work exists, no proof runs, the lane is not paused, and no progress for 20 minutes; progress is the newest of a proof start (`running.json`), a proof end (`results.jsonl`), a push (`pushes.jsonl`), a return (`queue.jsonl`) or a lane question opened |
| Spend fence | `updateSpendEpisodesWith` submits through the local notifier (`tick.go:549`) | the same call gets the channel post as its deliver function | a new crossing multiple (existing) |
| Budget breach-stop | the stop pass only reports (`tick.go:247`, `:137-175`) | a `COMPLETE` report opens one episode keyed by goal and revision, which every report carries (the first has no stop id); the stop producer is unchanged: "Goal G was stopped: it spent its budget." / `metasystem goal resume G` | the stop |

The lane is checked sooner than seats because it blocks every seat. Moved to Deferred: claimed-goal delivery ("burned without delivery") and the stuck patterns. Health episodes carry no owner and share one digest across roles (`alert_episode.go:435-439`), so routing one role needs the health episode split per role, which is more than this step needs.

## Decision 5: the unblock log

The question records are the log: opened, machine, goal or about, refusal line (first `--fact`), state, answer text and time. One read is added: `metasystem question list --answered [--since 7d]`. Its summary groups answered and withdrawn questions by refusal line, most frequent first, with counts and the median time to answer; `--verbose` lists each. It reads all records instead of the open ones (`WalkOpenQuestions`, `question.go:127-153`). No new store.

## Decision 6: delegates

Engine-launched delegates inherit `METASYSTEM_OWNER_LINEAGE` from the dispatching seat; no launcher sets it for them (`internal/adapter/supervisor/deps.go:111`, `lifecycle.go:191`). A delegate's question would be filed under its seat's lineage, and a sandboxed delegate often cannot reach the channel or the ledger *(inferred from the Codex sandbox)*. So the fix is a rule, not code: delegates return `BLOCKED:` and the seat, which owns the goal and the turn, asks.

## Decision 7: a quiet channel (Wido 2026-10-02)

Wido: "I want to have the IM (telegram/slack/...) channel as silent as possible i.e. if there is no news, there should be no message. I only want a message when there is a landing on main, or when there is something I need to respond to"; "everything else; I have the UI for".

1. The periodic status report (`internal/channel/phase/phase.go:197-213`, `ShouldPost` `internal/channel/report.go:378`) no longer posts to the channel; needs, backlog and delivered live in the UI. `channel.status.interval-minutes` becomes unused and is removed from the local config.
2. A landing on main posts one line per landing: the goal and the short SHA ("landed: G at SHA"). Producers: the lane's `landing push` after a successful push, and a seat that lands its own work. Once per SHA.
3. Every other channel message asks for a response: a question, or a notice whose second line is the act the human takes (Decision 4's signals). A message that only informs does not go to the channel. That includes the answer receipt ("recorded as your word on ...", `internal/channel/poll.go:394-415`): it is no longer posted, and the answer still moves to phase closed and the question to state closed locally, as today.

## Decision 8: one bot, the ledger inbox, first commit wins (Wido 2026-10-02)

Wido: "We will use ONLY ONE BOT TOKEN and we will keep track of the last message read and place the message in the ledger (or central place)." This is step 1 of the accepted design `plans/fleet-channel-gateway-design.md` (revision 4) for today's case: one computer, two installations (the seat `agentic-tools-m1e` and the lane `agentic-tools-landing`), one bot, one ledger. Its binding words: "one bot, one git inbox, FIRST COME FIRST SERVED - no leases"; "receive -> commit to the shared git inbox -> confirm". Today each installation polls with its own cursor (`internal/channel/poll.go:140-148`), Telegram confirms the offset for the whole token, and the first poller files the other installation's reply in its own `unmatched.jsonl` (`poll.go:169-177`), where it is lost.

1. **One token, every installation polls.** Both installations carry the same token and chat (local config only). Each keeps its steward's channel duty and `question wait` poll as they are. There is no lock, lease or leader, and a 409 is retried on the next poll (FCG-PRINCIPLE-01). Receive sends no offset, so Telegram returns every unconfirmed update (FCG-RECEIVE-03).
2. **Commit, then confirm (FCG-RECEIVE-03, FCG-INBOX-02).** For each update, in update order, the receiving installation builds one inbox record, `plans/channel/inbox/<destination>/telegram-<message id>.json`, and publishes it through `goal.Publish`. The record's path makes the write idempotent by message id. A record already on the tip under another opid is the ordinary lost-to-winner (`goal.LostToCompetitor`, `internal/goal/txn.go:561`; `TrailerPresent`, `:476`). After a confirmed or lost publish the installation calls `Provider.Confirm` with the update's `Ack`, and never before the commit is durable. A poller that dies before committing has confirmed nothing, so the update comes again.
3. **The committing installation checks sender and code (FCG-COMMIT-05).** The checks run in order: user id, code present, not stale, TOTP at `sentAt`. The code is removed from the text, and the outcome is written in the record. The replay check moves into the publish: a step already on another message at the tip makes this one `replayed`. Step 1 writes `question: "unmatched"`, because question records are not on the ledger yet. A reply that fails a check (wrong user, no code, bad code, stale, replayed) is answered by the installation that committed it, whichever installation owns the question: only the publish winner posts, once per inbox record. The notice goes into the original question's thread: as a reply to the message the human replied to (the record's `replyTo`), or unthreaded when the human's reply was unthreaded. Its text is "not recorded: <reason>. Reply to the question above with your answer and your code". The notice's message id is not saved anywhere: the corrected reply threads to the question itself, or carries its token, and the owner already matches both. This is a response-needed message under Decision 7. The question stays open, because only a `verified` record is matched.
4. **Each installation matches its own questions from the ledger inbox (FCG-MATCH-06).** Every poll then reads the inbox records at the tip that it has not handled. It matches a `verified` record against its own open questions: threaded first (the `replyTo` is the question's thread), then by token for an unthreaded message, and never a stray. A match records the answer exactly as today, and `poll.go:357` and the local record are unchanged. A record that names none of its questions is left for the other installation. So lane questions work with the lane's own steward and no per-installation bot.
5. **What survives and is reused.** `internal/goal/channel.go` (742 lines) keeps the inbox, question and listener schemas (`ChannelInbound`, `:82`) and `ValidateChannelTree`, which every ledger validation already runs (`internal/goal/validate.go:682`, `validatedread.go:51`). The provider contract already has `Ack`, `UpdateID` and `Confirm` (`internal/channel/channel.go:24-25,45`, Telegram `telegram.go:262`). The fake bot (`internal/channel/fake/fake.go`) serves named listeners, a shared confirmed offset (`:541-575`) and scripted 409s (`:229-240`). The deleted receive library (`3f52b2ff7`, recoverable from `57c310a1e`) is not restored: step 1 needs only the publish Mutate above.

## Moved effects

| Effect | From | To | Code |
|---|---|---|---|
| Letting a turn end while a question waits | open-plan, claimed-goal and idle-backlog refusals | the open-question predicate | `metasystem/internal/goal/turnverdict.go:1774-1786`, `metasystem/internal/goal/turnverdict.go:1812-1826`, `metasystem/internal/goal/turnverdict.go:1284-1304` |
| Leaving an asked goal alone | idle continuation and seat ladder treat it as claimable | `AskedOpen` from the question record | `metasystem/internal/goal/project.go:440`, `metasystem/internal/steward/seat_ladder.go:77` |
| Holding a blocked landing agent | keeper relaunches on queued work | open lane question holds the relaunch | `metasystem/cmd/metasystem/landing_agent.go:184-192` |
| Telling the human a seat is idle | saved episode, local alarm only when no continuation | channel post at episode creation | `metasystem/internal/steward/alert_episode.go:236-277`, `metasystem/cmd/metasystem/goal.go:784-790` |
| Telling the human the spend fence crossed | local notifier | channel post | `metasystem/internal/steward/tick.go:549` |
| Telling the human a goal was breach-stopped | tick report only | channel notice episode | `metasystem/internal/steward/tick.go:247` |
| Telling the human the lane is silent | no signal today | `lane-silent` episode | `metasystem/internal/steward/tick.go:546-551`, `metasystem/internal/landing/plain/queue.go:42-45` |
| Recording a goal-less answer | goal ledger answer act | the question record alone | `metasystem/internal/channel/poll.go:357` |
| Recording an inbound reply | the poller's local files: the answer on its own question record, anything else in `unmatched.jsonl` | one ledger inbox record per message, first commit wins | `metasystem/internal/channel/poll.go:151-253`, `metasystem/internal/channel/poll.go:169-177` |
| Confirming the Telegram offset | the next `getUpdates` sent the saved per-installation cursor | `Confirm` after the inbox commit is durable; `cursor.json` is no longer read | `metasystem/internal/channel/poll.go:140-148`, `metasystem/internal/channel/poll.go:258` |
| Checking sender and code, and refusing a replayed code | the poller, against its local `totp-consumed` register | the committing installation, against the inbox at the ledger tip | `metasystem/internal/channel/poll.go:184-195` |
| Telling the human a reply was not recorded | the poller that owns the question, under its three-post ceiling | the committing installation's receive step, once per inbox record, in the question's thread; the rejection post's reference is no longer saved on the question or used for matching (removed) | `metasystem/internal/channel/poll.go:199-221`, `metasystem/internal/channel/poll.go:132-135` |
| Posting the periodic status report | the channel phase after every poll | removed (Decision 7) | `metasystem/internal/channel/phase/phase.go:197-213` |
| Posting the brain seat's status at Stop | the Stop hook through `channel status --post` | removed (Decision 7) | `metasystem/internal/hooks/runtime_hook_stop.go:860-864`, `metasystem/cmd/metasystem/channel_verbs.go:121` |
| Posting the answer receipt | the poll after recording an answer | removed; the close to state closed is kept (Decision 7) | `metasystem/internal/channel/poll.go:394-415` |
| Posting "landed: G at SHA" | no post today | the lane's `landing push` and the seat's own landing path, once per SHA (Decision 7) | `metasystem/internal/landing/plain/push.go:50`, `metasystem/internal/landing/landpath/commit.go` |
| Matching a reply to its question | the poller, against its own threads only | each installation, against its own open questions, from the ledger inbox | `metasystem/internal/channel/poll.go:122-138`, `metasystem/internal/channel/telegram/telegram.go:205-222` |

## Tests

1. Stop hook: with an open asked question, Stop is allowed with claimable backlog, with no other claimable goal after the ask changed the revision, with open plan work, and for a budget-stopped goal with another ready goal present (`TestQuestionWaitAllowsStopForFencedGoal`); an open question from another lineage does not count; once the question is answered or withdrawn, each branch behaves as today.
2. `AskedOpen`: open makes the goal left alone by idle continuation and the seat ladder; answered or withdrawn releases it.
3. Keeper: an open `about: lane` question holds the start; answered or withdrawn, the start proceeds.
4. Goal-less ask: `--about lane` records no ledger act; the answer reaches `recorded`; `--about` with kind `stop` is refused.
5. Alerts, each through its real producer with a fake provider: the third idle refusal posts once; a spend crossing posts once; a breach-stop posts once per stopped goal revision, the first report included, and the next tick posts no duplicate (`TestBreachStopNoticeOnNewStop`); a second tick posts nothing; no channel configured keeps today's path.
6. Lane silent: 19 minutes without progress no episode, 20 minutes one; a proof that ended 30 minutes ago with work still queued and no push opens one; a proof running, a pause or an open lane question, none.
7. `question list --answered --since`: grouping, counts, `--verbose`, open questions excluded.
8. One bot, two installations (fixture seat and fixture lane on one ledger, one fake Telegram bot, `internal/channel/fake`): each asks a question; both stewards poll at once, the fake scripting a 409 for one of them; the reply to the lane's question is recorded in the lane and the reply to the seat's in the seat; each reply has exactly one inbox record on the ledger, the loser of the commit race confirms without a second record, and no reply is lost; a poller killed after its commit and before Confirm leaves the update to be received again and lost to its own record; a code reused on a second message is committed `replayed`; a reply with a bad code to the seat's question, first received by the lane, gets exactly one "not recorded" post from the lane in the question's thread, and the seat's question stays open; the human then replies to the original question with a good code, and the seat matches and records it; an answered question closes locally and posts no receipt.
9. End to end (`cmd/metasystem`, fixture seat): the session asks; `question wait` runs in the background; the Stop hook allows the turn to end; the fake provider delivers a TOTP-valid reply; the wait exits 0 printing the answer; `question list --answered` shows it.

## Deferred (step 2)

| Item | Builds on |
|---|---|
| Answering in the UI | the record's `answer` and the poll's recording phase |
| Free-form human-to-seat messages | the agent inbox (`cmd/metasystem/intent_agent.go:163-215`) |
| Waking an idle seat when its answer arrives | the `human` wait row's question and the seat's session id |
| Claimed-goal delivery and stuck-pattern notices | health episodes split per role (`alert_episode.go:435-439`); pattern episodes' `deliverTo` (`pattern_episode.go:205`) |
| The rest of the accepted gateway design (`plans/fleet-channel-gateway-design.md`) | step 1's inbox records: question records on the ledger (FCG-INBOX-02 question table) and commit-time matching against them (FCG-MATCH-06 on the committing machine), the intent-post-ref posting protocol (FCG-POST-08), the resident long-poll listener and its jitter (FCG-POLL-04), listener status and heartbeats (FCG-STATUS-09), migration of local question records (FCG-MIGRATE-10), the two-step budget approval (FCG-ANSWER-11), the poison-update `channel skip` verb (FCG-RECEIVE-03), a second computer |
| Runtimes other than Claude Code resuming on wait exit | the wait's exit and the session start's question read |
