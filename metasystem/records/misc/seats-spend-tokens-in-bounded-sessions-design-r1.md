# seats-spend-tokens-in-bounded-sessions

Goal 1:1 (plans/goals/seats-spend-tokens-in-bounded-sessions.md), revision 1 of the design, written by a fresh Claude Fable delegate from the brief artifacts/reports/tokens-design-brief-r1.md on 2026-09-15. Paths are relative to the metasystem module. Line numbers name the tree read today; the symbol beside each is the anchor.

DONE, restated: a seat's main session stays under 200 thousand tokens of context by construction (it hands off at that size through the existing `metasystem context handoff`), long work runs in bounded delegates and one launcher per unit, waiting on registered background work costs no stop-hook turn, tokens per seat per day are split by cause and by model and divided by landed units in health, and one working day of three seats lands under the thresholds of Q4 against the 2026-09-15 baseline.

## 1. Evidence

From records/misc/token-diagnosis-2026-09-15.md (CEST day to 12:45, three seats):

| fact | number |
|---|---|
| three-seat day; per seat | 871.7M; m1b 317.0M, m1c 165.4M, m1e 389.2M; cache reads 96.7 to 98.6 percent |
| main interactive sessions | 779.5M (89.4 percent); calls at 400K context or more carry 696.6M of that |
| notification turns | 88 turns, 538 calls, 299.7M; 6.1 calls per wake at 555K mean context; background Bash 204.9M, Monitor 56.5M, Agent finished 31.6M |
| stop-hook refusal turns | 103 turns, 439 calls, 284.2M; 4.3 calls per refusal at 646K; 54 percent of m1c's day; 27 refusals for one unchanged task |
| peer message turns | 53 turns, 181 calls, 118.2M; 3.4 calls at 652K; 348 more messages arrived mid-turn (148 notifications, 99 peers), which under table 2c makes peers second at 229.6M |
| human turns | 19 turns, 54 calls, 34.9M at 644K |
| compaction and usage-limit resumes | 43 calls of summaries plus 34 resumes, 42.6M |
| delegates; engine jobs | 79.9M (478 calls at 164K); 12.2M (100 calls at 117K) |
| largest session | m1e main 36c93128: 653 calls, 335.8M, peak 966K, 2 compactions |
| context growth | about 4K tokens per call (m1c: 217 calls to 869K with no compaction; m1e: 653 calls over three fills) |
| fresh-session floor | about 110K (m1e's in-flight coordinator 9292cf37: 16 calls at 122K mean) |
| the fence | m1e seat dayTokens 264.4M at 13:00 CEST (top-level main transcripts only, UTC day) against spend.ceiling.day.tokens 250,000,000 (metasystem.conf:32); day tokens 322.5M with 58.1M of engine jobs; subagents/ files and the other seats are outside it |

What the tree already has: `usage.LatestCall` reads the last main-thread call's `input + cache_creation + cache_read` behind a cursor (internal/usage/calls.go:93); `steward.ContextBudgetLine` renders the `context-budget` health role with `ContextBoundTokens = 150000` and `ContextCeilingTokens = 200000` (internal/steward/context.go:22-24, contextVerdict :292-330: dead with `NoAutomaticRemedy` over the ceiling, remedy `metasystem context handoff`); the Stop hook prints that health line on every attempt; `metasystem context handoff` writes the state file, stages the `seatHandoff` intent and the tick launches a steward continuation after the predecessor's death (plans/coordinator-context-stays-under-budget-design.md 8c D3-1 to D3-6, slice 3 landed C1a 0cca29a4, C1b ffdc623b, C2 25fb3b54); the turn verdict allows a Stop under a live handoff (`HandoffRecorded`, turnverdict.go:205) and treats an eligible registered wait as work in flight (`registeredWaits.hasWorkInFlight`, :257, used at :977 and :1439). None of it bit on 2026-09-15: the health line said "dead, hand off" at 966K and nothing forced the act; and no wait a seat registered from a background shell ever matched the Stop's session (registered-wait-matches-the-runtime-session).

## 2. Decisions

### Q1. Context cap: hand off at 200K

Model: floor F = 110K after a fresh start, growth g = 4K per call, headroom H = C - F, handoff cost = two calls at C (the handoff turn) plus one boot call at F, mean context (F + C)/2 over a session's life. Baseline main-session calls per day: m1e 653, m1b 484, m1c 217.

| cap C | mean context | wake (6.1 calls) | refusal (4.3) | peer (3.4) | calls per session | m1e handoffs per day at 653 calls | m1e main day (653 calls + handoffs) |
|---|---|---|---|---|---|---|---|
| 200K | 155K | 0.95M | 0.67M | 0.53M | 22 | 29 at 0.51M | 101M + 15M = 116M |
| 250K | 180K | 1.10M | 0.77M | 0.61M | 35 | 19 at 0.61M | 118M + 12M = 130M |
| 300K | 205K | 1.25M | 0.88M | 0.70M | 47 | 14 at 0.71M | 134M + 10M = 144M |

Chosen: 200K. It is the cheapest day at every cap (116M against 130M and 144M for m1e's baseline calls, from 337.3M), it is the ceiling constant the sibling goal already landed and proves (no call over 200K, p95 under 150K, contextreport.go:674), and under 200K no harness compaction can fire, which removes the 42.6M of summaries and resumes. Rejected: 250K, 14M a day dearer per seat and a raised constant; 300K, 28M dearer, and its only gain (14 rather than 29 relaunches) shrinks to 3 against 7 once Q2 and Q3 cut the calls per day to about 150. The bound 150K stays the advisory line ("over the bound: run metasystem context handoff") at which a seat hands off at its next unit boundary.

Measurement: the last main-thread call's prompt tokens from the transcript, through `usage.LatestCall` as today; the verdict reads it, not the hook (runtime independence: the same rule fires on `codex stop`). Where the trigger fires and what the seat sees: the turn verdict, at the first Stop whose latest sample is at or over 200K with no live handoff for the session, blocks with class `context-over-ceiling` and the one sentence `CONTEXT OVER CEILING: N thousand tokens this call; run metasystem context handoff --root <installation> alone in this message, then end this session`; that block takes precedence over idle-backlog and open-work blocks so the seat reads one instruction; it is bounded to two per session, after which the Stop is allowed with the same sentence as a standing notice and the human attention item `context-over-ceiling` is raised (the idle rule's escalation shape at turnverdict.go:1041). Cost of the mechanism: at most two refusals of about four calls at 200K, 1.6M, once per session. A seat under the ceiling sees nothing new; the health line stays as it is.

What the handoff needs: no pending job launch (`HANDOFF_LAUNCH_IN_FLIGHT` stays); every required scratch reference verifying (stays); a goal held (stays). It no longer needs an empty wait registry: a pending registered wait is recorded in the state file as an open wait (Q2, unit 3b) because under Q2 a seat at the cap will usually have one. The fresh session reads first: the state file named by `metasystem context resume` (unit 3c), which prints the held goal, the next step, the open waits with their `metasystem wait --resume WAIT-ID` commands and the messages owed; then the goal file; nothing else before the first act. Unchanged in the verb: the state file schema 1 fields and the 32 KB bound (D3-2), the nonce directory and `O_EXCL` write (D3-1), intent staging and digest verification (D3-4, D3-5), the authority rules C1a and C1b, the tick's launch after the predecessor's death for headless seats, `context verify` and `context prune`. Changed: the wait refusal becomes a record (3b); the intent carries `successor: interactive|headless` and the tick holds an interactive one for a relaunch grace window (3c); the state file gains `openWaits` (3b).

The successor of an interactive seat: the seat's pane is where Wido's human verbs run, so the successor is a fresh interactive `claude` in the same pane, not a headless continuation. A pane driver (unit 3d, `scripts/agents/seat-relaunch.sh`) ends the predecessor and starts the successor; the SessionStart hook tells the successor a handoff waits for it; `context resume` consumes the intent so the tick does not also launch a continuation. Open question 1 puts the alternative to Wido.

### Q2. The stop gate sees harness-tracked work: the wait registry, not harness files

How the hook learns of live work: through the wait registry only. The harness's own files are not a liveness source: on this box `~/.claude/tasks/` holds one session directory from August (background output for today's sessions is not there), and a delegate's `subagents/agent-*.meta.json` (about 200 bytes: type, description, model, launching tool use) says when it started and nothing about whether it ended. Q4 reads those files for measurement after the fact; the verdict never does. Rejected: mtime-based liveness on transcript files (a stalled delegate looks dead, a finished one whose file is still being flushed looks alive; and a Codex seat has no such files).

Three kinds of work and their rule:

| work | how the gate sees it | liveness rule |
|---|---|---|
| engine jobs (Codex builds, dispatched delegates, steward continuations) | today: `work.InFlight` `job:` entries and `metasystem wait --job` rows | unchanged: the job record's custodian pid, reaped by the reaper; the job wait's row under `registeredWaitEligible` (turnverdict.go:668-704) |
| background Bash tasks (engine runs, beds, landing drivers, the unit launcher of Q3) | a `local` wait: `metasystem wait --local --label L [--goal G] --timeout D -- <command>` forks the command and registers a schema 2 row of kind `local` whose `Target` carries the child's pid and start identity and whose heartbeat is written every 10 s | eligible while the waiter pid and the child pid are both alive by the identity prober, the heartbeat (`LastObservedAt`, both clocks) is within 30 s, the deadline (at most 24 h) is ahead, and the row's owner coordinates match the holder as for every kind; the row turns terminal with the child's exit code as its result |
| Claude delegates (Agent tool) and Monitor | not covered | a seat does not stop while one runs; under Q3 reads and designs run headless inside the launcher (a local wait), and Monitor is not used; the Agent tool stays for short in-turn forks only (Q5) |

Why the seat's own waits never matched today: a watch registered from a background shell writes `Session` `session-<pid>` because the shell does not know the runtime session, and `registeredWaitEligible` compares `row.Session` and `row.RuntimeSession` with the Stop's `session_id` literally (turnverdict.go:672). Decision for registered-wait-matches-the-runtime-session: the waiter resolves the runtime session at registration from the engine's own records, not from an exported environment variable: when the registering process descends from the pid the current holder announced (`lease.CurrentHolder` gives `MainId`, `SessionId` and the pid; the announcement file artifacts/agents/mains/<session>-<pid>.json), the row records `Session = RuntimeSession = holder.SessionId`; a process outside that ancestry keeps the pid-derived key and stays ineligible, as does any row whose `MainId`, `OwnerLineage`, `OwnerDigest` or `ClaimEpoch` differs (all still checked). Rejected: the adapter exporting the session into the seat's environment, because a Codex or fake seat then needs its own export and a forged variable would be trusted.

The verdict while such work is live: allow the Stop with no `systemMessage` beyond the ordinary report (`waits.hasWorkInFlight()` already resets `IdleBlocks` and returns before the idle block, turnverdict.go:977; the open-work branch at :1439 already prints `WORK IN FLIGHT`), and one wake when the work completes: the harness's own notification for the background task (Claude) or the wait verb's return (any runtime). Nothing new is sent to the model while the wait is pending.

The idle-backlog rule and the other repeated refusals: 103 refusals a day, 27 of them for one unchanged task. The bound of three per unchanged backlog (turnverdict.go:996, `refusal N of 3`) becomes two, and the same two-per-unchanged-digest bound applies to every block source: the session state keeps `(blockSource, digest) -> count`, where the digest is the idle-backlog digest, the open-work signature (`OpenWorkSignature`, turnverdict.go:88) or the context rule's size band; the third Stop for the same pair is allowed with the standing notice and raises the attention item, as the idle rule does at three today. Expected: refusals fall from 103 to at most two per distinct condition, about 20 a day across three seats; with U1 and U2 most conditions never refuse because the wait is live. Cost at the cap: 20 refusals at 0.67M = 13.3M against 284.2M.

### Q3. Fewer wakes: one launcher per unit, no Monitor, short messages

One launcher per unit: `scripts/agents/unit-launcher.sh <unit-id> <plan-file>` runs the unit's steps in order (brief admission, build dispatch and its wait, verification through the engine, the reads as headless `claude -p --max-turns N` runs with a fresh cwd copy and a call budget from the brief, the section runs, the landing script), stops at the first red step, writes artifacts/reports/<unit-id>-launcher.md with one verdict line per step and the step reports by path, and prints exactly one line (at most 200 characters: unit, last step, verdict, report path). The seat starts it once as a background Bash wrapped in `metasystem wait --local`, and is woken once. Every wait inside the launcher is the wait verb (member 1 of the wakes goal), so there is no poll from the model. Rejected: one background task per step (61 background completions cost 204.9M today); the seat as the sequencer (every step's return is a wake at full context).

Poll scripts and Monitor as separate background tasks: not allowed. Polling belongs inside the launcher or inside `metasystem wait`; a human answer is `metasystem wait --goal G --event human-act --after COMMIT` (member 1), not a Monitor on a file. Monitor cost 56.5M today (13 turns); expected after: 0.

Peer messages: a seat rule first (Q5, S4: at most 400 characters, one subject, sent only at a unit boundary or for a question that stops the sender, never as an acknowledgement, anything longer by path). A mechanism (a per-seat inbox file that the stop report counts, read at the next turn start) only if the measured day after the cap still shows peers above 20M; the rule alone is expected to halve the 53 wakes and remove most of the 99 mid-turn arrivals. Expected: 30 wakes at 3.4 calls at 155K = 15.8M against 118.2M.

The wakes goal's landed members are taken as given: the wait verb with typed exits, the publication hints, the stop gate honouring a registered wait. Its remaining legs (clause 5: under 4 wake-ups per pending hour, under 5 percent of the seat's day while pending, under 60 s event-to-resume; and wait-whole-goal-claude and wait-whole-goal-codex) add the measurement of what U1, U2 and U4 deliver; U5's per-cause ledger gives clause 5 its "tokens while pending" figure without a second reader.

Expected notification cost: 25 wakes (one per unit launcher plus wait returns) at 4 calls (read the one line and the report, decide, brief the next step) at 155K = 15.5M against 299.7M.

### Q4. Measurement and the DONE thresholds

Unit 5 (goal 1:5, spend-fence-reports-tokens-per-model-and-cause) builds the diagnosis's attribution into `internal/spend`: every main-session call is attributed to the cause of its turn starter by the script's rules (`origin.kind` task-notification, peer, human, auto-continuation; text prefixes `Stop hook feedback:`, `<task-notification>`, `Another Claude session sent a message`; compaction summaries), a `queued_command` attachment is counted as a mid-turn arrival of its cause; delegate files under `subagents/` are counted into the seat's day as cause `delegate` with the agent type from `meta.json`; engine job sessions keep their row. `Row` gains `Cause` and `Kind` (main, delegate, engine); the ledger gains `Causes` (calls, turns, all-in, cache read, mean context per cause) and `Models` (all-in per canonical model); cache reads stay a separate column (`Tokens.Cached`) and the health reason prints both all-in and cache-read-at-0.1-weight figures. Landed units per seat-day: the goal ledger's History rows with verb `land-ready` or `release` whose actor is this machine and whose stamp is in the day; tokens per landed unit = seat day all-in divided by that count (shown as `n/a` at zero). Health: the `spend-fence` reason gains `causes notification=.. stop_hook=.. peer=.. human=.. delegate=.. engine=..; models ..; units=N per-unit=..M; main-share=..%; ctx-mean=..K`; per-model alert lines `spend.ceiling.day.tokens.<canonical-model>` join the day and goal ceilings as their own crossings. The fence's seat scope widens to subagent files, so m1e's fence figure and the diagnosis's per-seat figure agree.

Baseline for the thresholds: 871.7M for the three seats over the CEST day to 12:45; 264.4M for m1e's own fence figure at 13:00 CEST in today's scope. Thresholds, each measured by unit 5 and cross-checked by the diagnosis script on the same day:

| threshold | value | baseline |
|---|---|---|
| three seats, same window (00:00 to 12:45 CEST) | at most 200M | 871.7M |
| three seats, full working day | at most 400M; per seat at most 150M in the widened fence scope | 09-14 full day 1,443.0M |
| m1e's own fence figure at 13:00 CEST, today's scope | at most 80M | 264.4M |
| main-session share of a seat's day | at most 40 percent | 89.4 percent |
| main-session context | every call at most 200K, no compaction, no usage-limit resume above 200K | mean 555K to 720K, peaks 966K, 3 compactions |
| tokens per landed unit, per seat-day | at most 20M | m1e about 65M (389.2M over about six landed units) |
| stop-hook refusals per seat-day | at most 10 | 41, 7 and 55 |

Expected day after every unit lands (same 12:45 window, three seats, calls per turn from the diagnosis's table 2d where unchanged):

| class | baseline | cap only (Q1) | all units |
|---|---|---|---|
| notification | 299.7M | 88 wakes, 6.1 calls at 155K: 83.2M | 25 wakes, 4 calls: 15.5M |
| stop-hook | 284.2M | 103 at 0.67M: 68.6M | 20 at 0.67M: 13.3M |
| peer | 118.2M | 53 at 0.53M: 27.9M | 30 at 0.53M: 15.8M |
| human | 34.9M | 54 calls at 155K: 8.4M | 8.4M |
| compaction and usage-limit | 42.6M | 34 resume calls at 155K, no compaction: 5.3M | 5.3M |
| handoffs | 0 | 1,354 calls x 4K / 90K = 60 at 0.51M: 30.6M | about 400 calls: 18 at 0.51M: 9.2M |
| delegates | 79.9M | 79.9M (the sibling goals cut this; not credited here) | 79.9M |
| engine jobs | 12.2M | 12.2M | 32.2M (the launcher's headless reads move 20M here) |
| total | 871.7M | 316.1M | 179.6M |

Derivation of the all-units row: turn counts from Q2 and Q3 (25, 20, 30), unchanged human turns, calls per turn 4, 4.3, 3.4 and 2.8, every call at the 155K mean of Q1, handoffs at one per 22 calls. Scaled by the 09-14 ratio of a full day to its first 14 hours (1.86), the full day is about 335M for three seats, 112M per seat: under the 150M per-seat threshold and the 250M fence line, and 4.3 times below 09-14.

### Q5. Seat practice from tomorrow, without a build

Each rule is checkable from the seat's transcript (the diagnosis script's readers) or from the ledger.

- S1 Handoff size. When the Stop hook's health line or `metasystem context status` shows the last call at or over 200 thousand, the seat's next message runs `metasystem context handoff --root <installation>` alone and ends with `end this session`; the driver or Wido restarts the pane. Check: no main-thread call after the first sample at or over 200K except one handoff turn of at most three calls; `compact_boundary` records: zero.
- S2 One background task per unit step, one launcher when U4 lands. Until then: never more than one `run_in_background` Bash per unit at a time, no Monitor, no `sleep` loop in a foreground Bash, waits through `metasystem wait`. Check: Monitor tool uses zero; background Bash launches per unit id at most one per step.
- S3 Delegates fresh with a call budget. Every Agent launch names its call budget (design at most 45, read at most 30, critique at most 40) and a page or report ceiling; a new round is a new delegate; no `SendMessage` to a delegate. Check: `resumedAgentId` results zero; every Agent prompt contains the word budget.
- S4 Messages. A `SendMessage` is at most 400 characters, one subject, sent at a unit boundary or for a question that stops the sender, never an acknowledgement; more goes by path. Check: message length and count per unit.
- S5 Reads by path. The main session never reads a tool result over 20,000 characters (grep headings, `sed -n` ranges, `cut -c`); a large output is read by a delegate from its persisted file. Check: tool results over 20,000 characters zero (the diagnosis's section 7).
- S6 The seat's own accounting. Before the first act of a session, `metasystem context status --root <installation>`; the day's receipt names the seat's all-in tokens and landed units. Check: the receipt row.

## 3. Units

Every unit is at most 300 changed lines (additions plus deletions, tests and docs included; docs/design/design-principles.md, Implementation Slicing), buildable by Codex from this page and its brief. Token effects are per day for three seats at the cap unless said otherwise. Order is tokens saved per line built, with dependencies respected: U6 costs no build; U1 is small and turns the landed stop-gate exemption on; the cap group (U3b, U3c, U3d, U3a) is the largest saving and lands as a group; U2 before U4 because the launcher runs under a local wait; U5 last but before the proof.

| id | goal | boundary (files) | ceiling | rules | expected token effect |
|---|---|---|---|---|---|
| U6 | 1:1 seats-spend-tokens-in-bounded-sessions (this goal keeps measurement and proof) | docs/orchestration.md (one paragraph of S1 to S6), scripts/agents/roles/steward-continuation.md (open waits line), scripts/agents/conformance-fixtures.sh (grep leg) | 80 | R29 | seat practice from tomorrow: S1 alone moves the mean context from 555K to 155K wherever the seat obeys; no machine effect claimed |
| U1 | 1:24 registered-wait-matches-the-runtime-session | internal/run/waiter.go (registration resolves the holder's session), internal/run/waiter_test.go, internal/goal/turnverdict_test.go (child-shell fixture), scripts/agents/supervision-fixtures.sh (leg wait-child-shell) | 200 | R1, R2, R3 | refusals while a job wait is pending stop: about half of 103 refusals; 52 at 2.8M today = 145M; at the cap 35M |
| U3b | 1:10 coordinator-context-stays-under-budget, amendment 8d | internal/steward/handoff_capture.go (pending waits recorded, not refused), handoff tests, state schema note in plans/coordinator-context-stays-under-budget-design.md 8d | 150 | R12, R13 | enables handoffs under Q2; without it every seat at the cap with a live wait is stuck |
| U3c | new goal seat-relaunches-into-its-handoff, tier 2: an interactive seat's fresh session in the same pane picks up the recorded handoff and the tick does not double it | internal/steward/handoff_capture.go (`successor` on the intent binding, `HandoffRelaunchGrace = 15m` in revive.go's hold path), cmd/metasystem/context_verbs.go (`context resume`), scripts/agents/supervision-hook.sh (SessionStart line), tests, supervision-hook-fixtures.sh leg | 280 | R14, R15, R16, R17 | enables the cap for interactive seats; each handoff costs 0.51M instead of a session that grows to 966K |
| U3d | seat-relaunches-into-its-handoff | scripts/agents/seat-relaunch.sh (tmux: send `/exit`, wait for the announced pid's death, start `claude` in the pane with the same cwd, refuse without a live handoff), scripts/agents/seat-relaunch-fixtures.sh | 140 | R18, R19 | same |
| U3a | 1:10 coordinator-context-stays-under-budget, amendment 8d | internal/goal/turnverdict.go (context rule, the two-per-digest bound), cmd/metasystem/goal.go (`TurnVerdictOptions.ContextTokens` from `usage.LatestCall`), internal/report/stoppresentation.go (the sentence), tests | 260 | R8, R9, R10, R11, R20 | main sessions 779.5M to about 230M at 155K mean (the cap-only column: 871.7M to 316.1M) |
| U2 | 1:23 stop-gate-sees-harness-tracked-work | internal/run/waiter_local.go (new: fork, heartbeat, exit result), internal/run/waiter.go (kind `local`, `Target` pid fields), internal/goal/turnverdict.go (`registeredWaitSource` case local, eligibility for kinds carrying a goal), cmd/metasystem/run.go (`wait --local`), tests | 300 | R4, R5, R6, R7 | the other half of the refusals: 51 at 0.67M = 34M; and the launcher's cover |
| U4 | new goal units-run-through-one-launcher, tier 2: a unit's brief, build, reads, verification and landing run as one background launcher that wakes the seat once | scripts/agents/unit-launcher.sh, scripts/agents/unit-launcher-fixtures.sh, one plan-file example under artifacts/reports | 300 | R21, R22, R23 | notifications 83.2M (cap only) to 15.5M; Monitor 0; delegate re-reads move to headless fresh runs |
| U5a | 1:5 spend-fence-reports-tokens-per-model-and-cause | internal/spend/transcript.go (turn-starter attribution, subagent files, meta.json type), internal/spend/measure.go (`Cause`, `Kind`, `Causes`, `Models`), tests with a synthetic transcript | 300 | R24, R25, R26 | 0; makes every other number in this page measurable daily |
| U5b | 1:5 | internal/steward/health.go (reason line), internal/config/spend.go (per-model ceiling keys), internal/spend/measure.go (landed units per day from goal History), tests | 220 | R27, R28, R30 | 0 |
| proof | 1:1 | records/misc/tokens-day-<date>.md: one working day of three seats with U5's ledger and the diagnosis script re-run | (record, no code) | the Q4 thresholds | the DONE |

## 4. Open questions for Wido (at most three)

1. The successor of an interactive seat. Recommended: a fresh interactive `claude` in the same tmux pane through the relaunch driver (U3c, U3d), because the pane is where human verbs and approvals run. Alternative: the landed headless steward continuation is the successor and the pane holds only a human shell; no driver, but every approval then goes through another session. Until he answers, U3c builds the interactive path and the headless path stays as it is.
2. The cap. Recommended 200K (Q1's table). Alternative 300K if a relaunch every 22 calls is too much friction for the pane; it costs 28M a day per seat more at today's calls and raises the landed ceiling constant.
3. Peer messages. Recommended: the seat rule S4 only, and one measured day under U5 before any inbox mechanism. Alternative: build the inbox file and the stop report's unread count now (about 120 lines) if he wants the mid-turn arrivals gone at once.

## 5. Verification rows

Each rule names the witness that fails when that rule alone is removed. Go tests unless said otherwise; bash legs run through the engine on the seat.

| rule | statement | witness |
|---|---|---|
| R1 | a wait registered by a descendant of the holder's announced main pid records `Session` and `RuntimeSession` equal to the holder's announced sessionId | TestChildShellWaitRecordsTheRuntimeSession |
| R2 | a registration from a process outside that ancestry keeps the pid-derived key and the gate ignores it | TestForeignShellWaitStaysIneligible |
| R3 | with an eligible job wait pending the verdict allows the Stop with no block and resets the idle count; the leg registers the watch from a child shell of the seat and drives `claude stop` | TestStopAllowsWhileAJobWaitIsPending; leg wait-child-shell |
| R4 | a local wait with a live child, a fresh heartbeat and a deadline ahead allows the Stop and prints `WAITING: registered wait <id> covers local <label>` | TestLocalWaitAllowsTheStop |
| R5 | a local wait whose child pid is dead or reused (start identity differs) is ineligible | TestDeadLocalChildDoesNotAllowTheStop |
| R6 | a local wait whose heartbeat is older than 30 s on either clock is ineligible | TestStaleLocalHeartbeatDoesNotAllowTheStop |
| R7 | the wait returns the child's exit code, records it as the row's result and turns the row terminal; a deadline returns 124 and never kills the child | TestLocalWaitReturnsTheChildExit |
| R8 | at a latest sample at or over 200K with no live handoff the verdict blocks with class `context-over-ceiling` and the one sentence, and no idle or open-work block is appended | TestTurnVerdictBlocksOverTheContextCeiling |
| R9 | the third Stop at the same size band allows with the standing notice and raises the `context-over-ceiling` attention item | TestContextBlockIsBoundedToTwo |
| R10 | with a live handoff for the session the allowance `handoff recorded: <nonce>; end this session` wins over the context rule | TestContextRuleYieldsToARecordedHandoff |
| R11 | under 200K, and when the reading is unknown, the verdict is unchanged | TestContextRuleSilentUnderTheCeilingAndOnUnknown |
| R12 | a pending registered wait no longer refuses the handoff; the state file lists it under `openWaits` with kind, target, deadline and the resume command | TestHandoffRecordsPendingWaitsInsteadOfRefusing |
| R13 | a job with status `pending` still refuses `HANDOFF_LAUNCH_IN_FLIGHT job=<id>` | TestHandoffRefusals (existing table row, kept) |
| R14 | `context resume` on a live `seatHandoff` intent for this machine whose predecessor is dead verifies the digest, consumes the intent, prints the state path and the open waits, exit 0 | TestContextResumeConsumesTheHandoff |
| R15 | `context resume` while the predecessor lives refuses `HANDOFF_PREDECESSOR_ALIVE` and consumes nothing; with no live intent it prints `no handoff` exit 0 | TestContextResumeRefusesWhileThePredecessorLives |
| R16 | `decideForRevival` holds an intent whose successor is `interactive` for 15 minutes after the predecessor's observed death, then launches the continuation; a `headless` intent launches at once as today | TestDecideForRevivalHoldsForTheRelaunchGrace |
| R17 | SessionStart prints `handoff <nonce> waits for this seat: run metasystem context resume --root <installation> first` when a live intent names this machine | leg session-start-handoff-line (supervision-hook-fixtures.sh) |
| R18 | the driver ends the predecessor, sees the announced pid dead, starts the successor in the same pane and the successor announces within 90 s | seat-run proof recorded in records/misc/seat-relaunch-proof.md |
| R19 | the driver refuses with `no live handoff for this seat` when no intent names the seat, and does nothing to the pane | seat-relaunch-fixtures.sh leg refuse-without-handoff |
| R20 | every block source is bounded to two refusals per unchanged `(source, digest)` per session; the third allows with the standing notice; a changed digest restarts the count | TestRepeatedRefusalsAreBoundedToTwoPerDigest |
| R21 | the launcher runs its plan's steps in order, stops at the first red step and writes that step's verdict line and report path | unit-launcher-fixtures.sh leg stops-at-first-red |
| R22 | the launcher's stdout is one line of at most 200 characters naming unit, last step, verdict and report path | leg one-line-summary |
| R23 | a read step runs `claude -p` with `--max-turns` from the brief's call budget in a fresh cwd copy; the transcript of that run is a new file, never a resumed one | leg read-budget-and-fresh-run |
| R24 | every main-session call is attributed to its turn starter's cause by the diagnosis's rules; a `queued_command` attachment counts as a mid-turn arrival of its cause | TestSpendAttributesCallsToTurnStarters |
| R25 | subagent files count into the seat's day as cause `delegate` with the type from `meta.json`; a session directory with no such file changes nothing | TestSpendCountsSubagentFiles |
| R26 | the ledger's `Causes` and `Models` rows each sum to the day scope's all-in | TestCauseAndModelRowsSumToTheDay |
| R27 | the `spend-fence` reason carries the causes, models, units, per-unit, main-share and mean-context figures in the stated order | TestHealthLineCarriesCausesAndUnits |
| R28 | a per-model ceiling crossing is its own `SpendCrossing` with scope `model-<canonical>` and alerts alone | TestPerModelCeilingCrossesAlone |
| R29 | docs/orchestration.md carries S1 to S6 and steward-continuation.md the open-waits line; the conformance grep leg finds each sentence | conformance-fixtures.sh grep leg tokens-seat-rules |
| R30 | landed units per seat-day count History rows with verb `land-ready` or `release` by this machine's actor in the day; zero prints `n/a` | TestLandedUnitsPerDay |

## 6. Records

Evidence and method: records/misc/token-diagnosis-2026-09-15.md and its script records/misc/token-diagnosis-2026-09-15.py (re-run for the proof day with the same window rules). Not read within the call budget: the wakes design's fixture section 5 and the handoff design's amendments 8c.9 to 8c.15 beyond their headings; the diagnosis's script beyond its method and shape lines.
