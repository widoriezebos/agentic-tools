# Blocked agent asks the human

- Kind: design
- Id: 01M3YB2G3QF059VC2HXQ0VAP13
- Status: draft
- Goals: blocked-agent-asks-the-human

Wido, 2026-10-02: a blocked agent is not a failure; it means the human should get involved, easily, and unblock it by a decision or otherwise. That includes the landing agent, and it is what lets the machinery switch on. Facts below were read at `135bd6a7a` unless marked *(inferred)*; the inventory is `agentic-tools-evidence/escape-hatch-20261002/inventory.md`.

## Threat model and rabbit-hole risks (read before critiquing)

Our own agents and operators make mistakes: an agent asks too often or too vaguely, forgets to wait, or waits in a way that burns turns; an operator misses a message. Nobody attacks. The channel's existing TOTP check on answers stays as it is.

| Rabbit hole | Mitigation |
|---|---|
| Answering from the UI | Deferred (step 2). Step 1 answers only in the channel thread, as today (`ui/decisions/decisions.go:120-128`). |
| Redesigning the question record or the TOTP check | One optional field (`about`) is added; nothing else in the record, the poll or the TOTP check changes. |
| A general alert-routing framework | Only the named signals go to the channel, each at its own producer. No routing rules, no per-alert config. |
| Turning a channel answer into an automatic authority act | Nothing acts on an answer by itself. Answers reach the agent that asked; steward signals are notices with the one command a person (or the agent the person tells) runs. |
| An agent that asks too often | The rule's threshold: ask only after the one obvious next step was tried and failed. Identical open questions are already de-duplicated (`internal/channel/question.go:219`). |
| Goal-less questions turning into a new store | They are ordinary question records with `about` set; same directory, same poll, same list. |
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

1. **The Stop hook: one predicate.** `questionPending()` on the session's registered waits is true when a wait names a question (`Selector.Question`, set at `channel_verbs.go:311` or by the `human` wait kind) and that question's record is open. It is read in one new case placed before the open-plan case of the Stop switch (`internal/goal/turnverdict.go:1774`), which therefore also covers the claimed-goal revision branch below it (`:1812`, `:1824-1826`): the case prints `WAITING: question Q` and does not block. The idle-backlog branch (`:1284`) reads the same predicate beside `hasWorkInFlight`. `suppressOpenWork` and `hasWorkInFlight` stay unchanged. The goal package cannot import `internal/channel` (channel imports goal), so the Store gets an `openQuestion func(id string) bool` seam, wired by the command to `channel.ReadQuestion`.
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
| Budget breach-stop | the stop pass only reports (`tick.go:247`, `:137-175`) | a report in state stopped opens one episode keyed by its stop id: "Goal G was stopped: it spent its budget." / `metasystem goal resume G` | the stop |

The lane is checked sooner than seats because it blocks every seat. Moved to Deferred: claimed-goal delivery ("burned without delivery") and the stuck patterns. Health episodes carry no owner and share one digest across roles (`alert_episode.go:435-439`), so routing one role needs the health episode split per role, which is more than this step needs.

## Decision 5: the unblock log

The question records are the log: opened, machine, goal or about, refusal line (first `--fact`), state, answer text and time. One read is added: `metasystem question list --answered [--since 7d]`. Its summary groups answered and withdrawn questions by refusal line, most frequent first, with counts and the median time to answer; `--verbose` lists each. It reads all records instead of the open ones (`WalkOpenQuestions`, `question.go:127-153`). No new store.

## Decision 6: delegates

Engine-launched delegates inherit `METASYSTEM_OWNER_LINEAGE` from the dispatching seat; no launcher sets it for them (`internal/adapter/supervisor/deps.go:111`, `lifecycle.go:191`). A delegate's question would be filed under its seat's lineage, and a sandboxed delegate often cannot reach the channel or the ledger *(inferred from the Codex sandbox)*. So the fix is a rule, not code: delegates return `BLOCKED:` and the seat, which owns the goal and the turn, asks.

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

## Tests

1. Stop hook: with an open asked question, Stop is allowed with claimable backlog, with no other claimable goal after the ask changed the revision, and with open plan work; once the question is answered or withdrawn, each branch behaves as today.
2. `AskedOpen`: open makes the goal left alone by idle continuation and the seat ladder; answered or withdrawn releases it.
3. Keeper: an open `about: lane` question holds the start; answered or withdrawn, the start proceeds.
4. Goal-less ask: `--about lane` records no ledger act; the answer reaches `recorded`; `--about` with kind `stop` is refused.
5. Alerts, each through its real producer with a fake provider: the third idle refusal posts once; a spend crossing posts once; a breach-stop posts once per stop id; a second tick posts nothing; no channel configured keeps today's path.
6. Lane silent: 19 minutes without progress no episode, 20 minutes one; a proof that ended 30 minutes ago with work still queued and no push opens one; a proof running, a pause or an open lane question, none.
7. `question list --answered --since`: grouping, counts, `--verbose`, open questions excluded.
8. End to end (`cmd/metasystem`, fixture seat): the session asks; `question wait` runs in the background; the Stop hook allows the turn to end; the fake provider delivers a TOTP-valid reply; the wait exits 0 printing the answer; `question list --answered` shows it.

## Deferred (step 2)

| Item | Builds on |
|---|---|
| Answering in the UI | the record's `answer` and the poll's recording phase |
| Free-form human-to-seat messages | the agent inbox (`cmd/metasystem/intent_agent.go:163-215`) |
| Waking an idle seat when its answer arrives | the `human` wait row's question and the seat's session id |
| Claimed-goal delivery and stuck-pattern notices | health episodes split per role (`alert_episode.go:435-439`); pattern episodes' `deliverTo` (`pattern_episode.go:205`) |
| Runtimes other than Claude Code resuming on wait exit | the wait's exit and the session start's question read |
