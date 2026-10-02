# Blocked agent asks the human

- Kind: design
- Id: 01M3YB2G3QF059VC2HXQ0VAP13
- Status: draft
- Goals: blocked-agent-asks-the-human

Wido, 2026-10-02: a blocked agent is not a failure; it means the human should get involved, easily, and unblock it by a decision or otherwise. That includes the landing agent, and it is what lets the machinery switch on. Facts below were read at `fea4ad1e8` unless marked *(inferred)*; the inventory is `agentic-tools-evidence/escape-hatch-20261002/inventory.md`.

## Threat model and rabbit-hole risks (read before critiquing)

Our own agents and operators make mistakes: an agent asks too often or too vaguely, forgets to wait, or waits in a way that burns turns; an operator misses a message. Nobody attacks. The channel's existing TOTP check on answers stays as it is.

| Rabbit hole | Mitigation |
|---|---|
| Answering from the UI | Deferred (step 2). Step 1 answers only in the channel thread, as today (`ui/decisions/decisions.go:120-128`). |
| Redesigning the question record or the TOTP check | One optional field (`about`) is added; nothing else in the record, the poll or the TOTP check changes. |
| A general alert-routing framework | Only the five named alerts go to the channel, chosen by episode owner in one list. No routing rules, no per-alert config. |
| An agent that asks too often | The rule's threshold: ask only after the one obvious next step was tried and failed. Identical open questions are already de-duplicated (`internal/channel/question.go:219`). |
| Goal-less questions turning into a new store | They are ordinary question records with `about` set; same directory, same poll, same list. |
| Rewriting the Stop hook | Two recognitions only: a pending question wait is work in flight, and an `ASKED` marker without its `ANSWERED` is a pending human word. |

## Step 1 and what it reuses

Exists and is reused unchanged: `metasystem question ask|wait|show|list|withdraw` (`cmd/metasystem/intent_process.go:330-409`); the question record (`internal/channel/question.go:43-61`); Telegram delivery and TOTP-checked answers by long-poll (`internal/channel/poll.go`); the durable wait that needs no model turns (`cmd/metasystem/channel_verbs.go:226-327`); alert episodes that submit once (`internal/steward/alert_episode.go:474`). No new transport, no new store.

## Decision 1: the rule and where it goes

Rule text (one paragraph, identical everywhere it appears):

> **When you are blocked, ask.** If a refusal or failure stops your work and the one obvious next step (the command the refusal names, or one retry) did not clear it, do not loop, guess or stop silently. Run `metasystem question ask GOAL --question "<the decision you need>" --fact "<refusal line 1>" --fact "<what you tried>" --option "<label>: <consequence>"...` (with no goal: `--about lane` or `--about machine` in place of GOAL), then `metasystem question wait channel:Q` in the background and end your turn. The answer is a decision, the person having done the act, or a bypass; act on it. Work on nothing else that the answer could invalidate.

Where: `metasystem/AGENTS.md`, replacing the "Ask first about choices..." sentence's tail and the Next-marker paragraph's opening (the `ASKED` marker now counts as a pending human word, so that paragraph gains `ASKED` to its list); `metasystem/skills/landing-agent/SKILL.md` as Case 8 ("Blocked outside cases 1-7: ask with `--about lane`, then end your turn; the keeper holds you until it is answered"); seats take AGENTS.md, so no seat brief text is added. Delegates get a different line, in `internal/protocol/templates/brief.md` and `design-brief.md`: "When blocked, stop and return `BLOCKED:` with the refusal's first line, what you tried and the decision needed; your seat asks." (Decision 6 says why.)

## Decision 2: how an agent waits

A seat runs `question wait` as a background task and ends its turn. The wait already polls the channel and returns the answer text with no model turns; Claude Code re-invokes the session when the background task exits. Two small changes make the Stop hook let that turn end:

1. `registeredWaits.hasWorkInFlight` (`internal/goal/turnverdict.go:275-282`) also returns true for a human-act wait whose selector names a question (`wait.row.Selector.Question != ""`, set by `channel_verbs.go:311` and read at `turnverdict.go:1041`). Today such a wait is not work in flight, so the idle-backlog branch (`turnverdict.go:1284-1304`) refuses the Stop up to three times. The observation at `turnverdict.go:1036` already drops a wait that is no longer pending.
2. `NextStepNamesAPendingHumanWord` (`internal/goal/humanword.go:8-16`) also matches `ASKED <qid>` in the newest entry unless the same entry holds `ANSWERED <qid>`. `goal.Asked` writes the first (`internal/goal/verbs.go:76`), the answer writes the second (`verbs.go:194`). The steward's seat ladder (`internal/steward/seat_ladder.go:77,111`) and idle continuation (`turnverdict.go:1320,1338`) then leave an asked goal alone.

The lane needs no wait: its Stop is always allowed (`internal/hooks/runtime_hook_stop.go:129-136`). The keeper gains one hold beside the proof and outage holds (`cmd/metasystem/landing_agent.go:184-192`): an open question with `about: lane` on this machine holds the relaunch, so a blocked landing agent is not relaunched into the same block. When it is answered the hold lifts, and the relaunched agent reads the answer with `question show`.

## Decision 3: a question without a goal

`question ask --about lane|machine` (no GOAL) writes a record with `goal: ""` and `about` set; `machine` is already recorded. Justification: the record, delivery, poll and list all serve as they are; only the goal-ledger steps need skipping, because a goal-less question has no goal file to mark. So `AskOrFind` (`question.go:205`) accepts an empty goal when `about` is set and kind is `other` (authority kinds keep needing a goal); `goal.Asked` (`question.go:258`) and `goal.Answer` (`poll.go:357`) are skipped and the answer goes straight to phase `recorded` with the receipt "recorded on question Q". A goal-less `question wait` registers the existing `human` wait kind (`cmd/metasystem/wait_register.go:93-101`), which the Stop hook already counts (`turnverdict.go:964-967`), and polls the record until `answer` is recorded. Rejected: naming a goal anyway (a lane block between batches belongs to no goal, and marking an unrelated goal misleads its seat); a per-machine pseudo-goal (a new ledger object).

## Decision 4: steward signals to the channel

Episodes keep their once-per-episode submit (`alert_episode.go:474-515`). For the owners below, when a channel is configured (`phase.Load`), the episode's transport is a channel post through the provider instead of the local notifier (`internal/steward/notify.go:145-172`); otherwise the local notifier as today. Each message is two lines: what happened in plain English, then the one command.

| Signal | When | Message |
|---|---|---|
| Claimed-goal delivery "burned without delivery" (`internal/steward/delivery.go:140-152`) | its existing health episode opens (150% of the goal's own limit) | notice: "Goal G has run past its budget without delivering." / `metasystem status G` |
| Seat idle (`alert_episode.go:236`) and stuck patterns (`internal/steward/pattern_episode.go:98`) | the existing episode opens (seat: third refusal) | notice naming the seat and goal / `metasystem session status --id S` |
| Lane silent with queued work (new episode owner `lane-silent`) | a hand-in waits 20 minutes with no proof running, no proof result since it was queued and no open lane question; sooner than seats because the lane blocks every seat | notice: "The landing lane has had work waiting for 20 minutes and is not moving." / `metasystem landing status` |
| Spend fence (`alert_episode.go:516-600`) | a new crossing multiple | notice / `metasystem alert clear E` |
| Budget breach-stop (`internal/steward/tick.go:137-175`) | a stop report reaches state stopped | question, kind `stop`, carrying the goal's standing box, options "resume: G resumes under the same box" and "leave it stopped". The recorded answer is the approval that `goal resume G --approved-ref` already accepts (`cmd/metasystem/goalsync_mutations.go:2919`); the steward runs that resume on its next tick. |

Only the breach-stop is a question in step 1, because it is the only one whose answer has a consumer today; the others ask the human to act, which is an answer too. The steward asks under lineage `steward`, as its channel duty already does (`internal/channel/phase/phase.go:185-188`).

## Decision 5: the unblock log

The question records are the log: opened, asked by (machine), goal or about, refusal line (first `--fact`), answer text and time. One read is added: `metasystem question list --answered [--since 7d]`. Default output is the summary: answered questions grouped by refusal line, most frequent first, with counts and the median time to answer; `--verbose` lists each question. It reads all records (`listQuestions`) instead of the open ones (`WalkOpenQuestions`, `question.go:127-153`). No new store.

## Decision 6: delegates

Delegates launched by the engine inherit `METASYSTEM_OWNER_LINEAGE` from the dispatching seat but no launcher sets it for them: the supervisor copies its whole environment (`internal/adapter/supervisor/deps.go:111`, `lifecycle.go:191`). A delegate's question would therefore be filed under its seat's lineage, and a sandboxed delegate often cannot reach the channel or the ledger *(inferred from the Codex sandbox)*. So the smallest fix is a rule, not code: delegates do not ask; they return `BLOCKED:` and the seat, which owns the goal and the turn, asks. No lineage change.

## Moved effects

| Effect | From | To | Code |
|---|---|---|---|
| Letting a turn end while a question waits | idle-backlog refusal, three times | the pending question wait counts as work in flight | `metasystem/internal/goal/turnverdict.go:275-282`, `metasystem/internal/goal/turnverdict.go:1284-1304` |
| Leaving an asked goal alone | idle continuation and seat ladder treat it as claimable | the `ASKED` marker is a pending human word | `metasystem/internal/goal/humanword.go:8`, `metasystem/internal/steward/seat_ladder.go:77` |
| Holding a blocked landing agent | keeper relaunches on queued work | open lane question holds the relaunch | `metasystem/cmd/metasystem/landing_agent.go:184-192` |
| Delivering the five named alerts | local notifier or osascript | channel post when a channel is configured | `metasystem/internal/steward/notify.go:145-172`, `metasystem/internal/steward/alert_episode.go:474` |
| Recording a goal-less answer | goal ledger answer act | the question record alone | `metasystem/internal/channel/poll.go:357` |

## Tests

1. Stop hook: a pending human-act wait with a question allows Stop with claimable backlog; without a question it still refuses (`internal/goal/turnverdict_test.go`).
2. Human word: `ASKED q-1 (other): x` matches; with `; ANSWERED q-1: yes` it does not; `ASKED` of another id still matches.
3. Keeper: an open `about: lane` question holds the start; answered, the start proceeds.
4. Goal-less ask: `--about lane` records no ledger act; the answer reaches `recorded`; `--about` with kind `stop` is refused.
5. Alerts: each named owner posts once per episode to a fake provider; a second tick posts nothing; an unnamed owner still goes to the local notifier; no channel configured falls back.
6. Lane silent: queued 19 minutes no episode; 20 minutes one; a proof running or an open lane question, none.
7. Breach-stop: a stopped report asks one `stop` question; its recorded answer resumes the goal once.
8. `question list --answered --since`: grouping, counts, `--verbose`, open questions excluded.
9. End to end (`cmd/metasystem`, fixture seat): the session asks; `question wait` runs in the background; the Stop hook allows the turn to end; the fake provider delivers a TOTP-valid reply; the wait exits 0 printing the answer; `question list --answered` shows it.

## Deferred (step 2)

| Item | Builds on |
|---|---|
| Answering in the UI | the record's `answer` and the poll's recording phase |
| Free-form human-to-seat messages | the agent inbox (`cmd/metasystem/intent_agent.go:163-215`) |
| Waking an idle seat when its answer arrives | the `human` wait row's question and the seat's session id |
| Options on the steward notices | the record's `options`, once a woken seat consumes the answer |
| Runtimes other than Claude Code resuming on wait exit | the wait's exit and the session start's question read |
