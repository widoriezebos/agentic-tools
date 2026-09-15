# seats-spend-tokens-in-bounded-sessions

Goal 1:1 (plans/goals/seats-spend-tokens-in-bounded-sessions.md), revision 2 of the design, written by a fresh Claude Fable delegate from artifacts/reports/tokens-design-brief-r2.md on 2026-09-15 at trunk a1d23839f. Paths are relative to the metasystem module. Line numbers name that tree; the symbol beside each is the anchor.

Changed from revision 1, after Codex critique round 1 (artifacts/reports/tokens-design-critique-r1.md, 17 material findings, all accepted) and the seat rulings R1 to R7:
- The cap trigger fires at a trigger line below the cap, from the transcript the hook names; an unknown sample never blocks (R1). The handoff ends in-flight waits and records the work; the successor registers fresh (R1).
- The refusal policies stay separate; only the idle-backlog bound changes, with the third-Stop outcome and the wake source stated for each case (R2).
- U1 is the registered-wait design's members A, B1, B2, B3, C, unchanged; every other unit's boundary names the real owners and is split under 300 lines (R3).
- The launcher is a Go verb on the delegate path with a judgement pause; seat rules are split into today and later; every rule has a Go witness (R4).
- Measurement has a window, a fleet sum, a same-scope baseline, a receipt-based landed-unit count, delegate subtypes, a provider comparison and the five-read figure (R5). The cap is configuration; the expected day is recomputed without the 20M double count and without the peer saving (R6). Section 6 records every finding.

DONE, restated: a seat's main session stays under the configured cap (default 200 thousand tokens of context) by construction, because the Stop gate makes it hand off at a trigger line below the cap through the existing `metasystem context handoff`; long work runs in bounded delegates through one launcher per unit; waiting on registered background work costs no stop-hook turn; tokens per seat per day are split by cause, model and kind and divided by landed units in health; and one working day of three seats lands under the thresholds of Q4 against the 2026-09-15 baseline.

## 1. Evidence

From records/misc/token-diagnosis-2026-09-15.md (CEST day to 12:45, three seats):

| fact | number |
|---|---|
| three-seat day; per seat | 871.7M; m1b 317.0M, m1c 165.4M, m1e 389.2M; cache reads 96.7 to 98.6 percent |
| main interactive sessions | 779.5M (89.4 percent); calls at 400K context or more carry 696.6M of that |
| notification turns | 88 turns, 538 calls, 299.7M; 6.1 calls per wake at 555K mean context; background Bash 204.9M, Monitor 56.5M, Agent finished 31.6M |
| stop-hook refusal turns | 103 turns, 439 calls, 284.2M; 4.3 calls per refusal at 646K; 54 percent of m1c's day; 27 refusals for one unchanged task |
| peer message turns | 53 turns, 181 calls, 118.2M; 3.4 calls at 652K; 348 more messages arrived mid-turn (148 notifications, 99 peers) |
| human turns | 19 turns, 54 calls, 34.9M at 644K |
| compaction and usage-limit resumes | 43 calls of summaries plus 34 resumes, 42.6M |
| delegates (subagents); engine jobs | 79.9M (478 calls at 164K; m1c 28.0M, m1e 51.9M; by type: code-critique 48.0M, general-purpose 31.9M); 12.2M (100 calls at 117K) |
| largest session | m1e main 36c93128: 653 calls, 335.8M, peak 966K, 2 compactions |
| context growth | about 4K tokens per call (m1c: 217 calls to 869K with no compaction) |
| fresh-session floor | about 110K (m1e's in-flight coordinator 9292cf37: 16 calls at 122K mean) |
| the fence | m1e seat dayTokens 264.4M at 13:00 CEST (top-level main transcripts only, UTC day) against `spend.ceiling.day.tokens` 250,000,000 (metasystem.conf:27-35 shows the commented default; the active value is the seat's); with m1e's 51.9M of subagent files the same-scope figure is 316.3M |
| the first bounded read | receipt 1789472212 (e700328e1): read_tokens=96634, read_calls=23, against 16 to 24M over 106 to 165 calls before |

What the tree has: `usage.LatestCall` reads the last main-thread call's `input + cache_creation + cache_read` behind a cursor (internal/usage/calls.go:93) and honours an explicit transcript path (internal/usage/calls_claude.go:14-17); `steward.ContextBudgetLine` renders the `context-budget` health role with the constants `ContextBoundTokens = 150000` and `ContextCeilingTokens = 200000` (internal/steward/context.go:22-24; `contextVerdict` :293) and reads non-blocking (:95-99, :121-126); the Stop hook passes session, main id and stop-hook-active to `report turn-verdict` but not the payload's `transcript_path` (scripts/agents/supervision-hook.sh:2336-2339; verb flags cmd/metasystem/goal.go:653-660); the verdict allows a Stop under a live handoff (`TurnVerdictOptions.HandoffRecorded`, internal/goal/turnverdict.go:205, :393) and treats an eligible registered wait as work in flight (`registeredWaits.hasWorkInFlight`, :257, used at :977 and :1439); `metasystem context handoff` refuses a `registering` or `pending` wait with `HANDOFF_WAIT_IN_FLIGHT` (internal/run/waiter_states.go:16-20; internal/steward/handoff_capture.go:374-426), a pending launch with `HANDOFF_LAUNCH_IN_FLIGHT` (:480), and a refusal exits 9 with no handoff recorded (cmd/metasystem/context_verbs.go:335-340). None of it bit on 2026-09-15: the health line said "dead, hand off" at 966K and nothing forced the act; and no wait a seat registered from a background shell ever matched the Stop's session (the fault plans/registered-wait-matches-the-runtime-session-design.md states once at its section 1).

## 2. Decisions

### Q1. Context cap: configured, default 200K, triggered at 150K

The cap is configuration, not a constant: `context.ceiling.tokens` (default 200000) and `context.handoff.margin.tokens` (default 50000) in metasystem.conf, read by `internal/config` (U3a1). The trigger line is the ceiling minus the margin, 150K by default, which is today's bound constant; the ceiling is today's ceiling constant, and `contextreport.go:565-570` keeps proving "no call over the ceiling, p95 under the bound" from the same keys. Growth is about 4K per call and a refusal turn about 4 calls, so two refusals (8 calls) plus the handoff turn (3 calls) fit in 44K of the 50K margin: a seat that obeys within two Stops never crosses the cap. A turn that runs past the cap before its Stop is outside the trigger's reach; the proof's maximum rule measures it, and S2 and S5 bound turn length.

Model: floor F = 110K after a fresh start, growth 4K per call, handoff cost two calls at the trigger plus one boot call at F. Baseline main-session calls per day: m1e 653, m1b 484, m1c 217. The mean context in the table is (F + C)/2, a conservative figure; with the trigger at C minus 50K the real mean is about 25K lower.

| cap C | mean context | wake (6.1 calls) | refusal (4.3) | peer (3.4) | calls per session | m1e handoffs per day at 653 calls | m1e main day |
|---|---|---|---|---|---|---|---|
| 200K | 155K | 0.95M | 0.67M | 0.53M | 22 | 29 at 0.51M | 101M + 15M = 116M |
| 250K | 180K | 1.10M | 0.77M | 0.61M | 35 | 19 at 0.61M | 118M + 12M = 130M |
| 300K | 205K | 1.25M | 0.88M | 0.70M | 47 | 14 at 0.71M | 134M + 10M = 144M |

Chosen: 200K. It is the cheapest day at every cap (116M against 130M and 144M for m1e's baseline calls, from 337.3M), it is the ceiling the landed proof already checks, and under 200K no harness compaction fires, which removes the 42.6M of summaries and resumes. Rejected: 250K, 14M a day dearer per seat; 300K, 28M dearer, and its only gain (14 rather than 29 relaunches) shrinks to 3 against 7 once Q2 and Q3 cut the calls per day to about 150. Under a 300K answer only one configuration line changes (open question 2).

The sample and its path: the hook reads `transcript_path` from the Stop payload the way it reads `session_id` (supervision-hook.sh:1534) and passes `--transcript <path> --runtime <runtime>` to `report turn-verdict`; the verb fills `TurnVerdictOptions.ContextSample` from `usage.LatestCall` with `ReadOptions{Transcript: path, NonBlocking: true}` and the runtime's declared call-sample capability, exactly as `ContextBudgetLine` reads today (context.go:95-126), sharing its cursor. The sample is the last main-thread call's prompt tokens. A sample that is unknown or unreadable (no path in the payload, a per-invocation runtime, the cursor lock busy, a malformed line) never blocks: the verdict prints `CONTEXT: unknown (<reason>)` and decides as today.

Where the trigger fires and what the seat sees: at the first Stop whose sample is at or over the trigger line with no live handoff for the session, the verdict blocks with class `context-over-trigger` and the one sentence `CONTEXT AT <N>K OF <C>K: run metasystem context handoff --root <installation> alone in your next message, then end this session; if it exits 9, follow the printed recovery`. That block takes precedence over idle-backlog and open-work blocks. It is bounded to two per session (a counter on the session state, not a size band); the third Stop is allowed with the same sentence as a standing notice and the human attention item `context-over-trigger`. Cost: at most two refusals of about four calls at 150K, 1.2M, once per session. Under the trigger nothing changes. Rejected: blocking at the cap after the fact (revision 1; the refusal turn itself crosses); a mid-turn message (the harness has no hook for it); the health line alone (it did nothing on 09-15).

The handoff with a wait in flight (R1): the seat cancels the wait and the successor registers fresh. `metasystem context handoff` gains an end-waits step before `waiterInFlight`: for every row owned by this main in state `registering` or `pending`, it signals the waiter's pid (SIGTERM; the waiter's own signal path turns the row `interrupted`, cmd/metasystem/wait_verb.go:159, internal/run/waiter.go:686) and captures the target under `openWork` in the state file: the kind, the target (job id, run id, attempt id, goal event and question, local label and child pid), the deadline that was left, and the fresh registration command the successor runs (`metasystem job watch --job J`, `metasystem wait --local --pid P --label L`, `metasystem wait --goal G --event human-act --verb answer --question Q --after COMMIT`). It never records `wait --resume`: a successor never adopts a predecessor's row (registered-wait design B3.3). A `registering` row is waited for up to 10 s, then treated as pending. The background shell that held the waiter exits 130 and the harness may wake the predecessor once more; that turn reads `handoff recorded` and ends the session. A child of a local wait keeps running; its pid is what the successor attaches to.

Recovery after exit 9: `HANDOFF_WAIT_IN_FLIGHT` after the end-waits step means a waiter re-registered in between; the seat retries once. `HANDOFF_LAUNCH_IN_FLIGHT job=<id>` means a dispatch is mid-write; the seat waits for its return (seconds, bounded by `dispatch.cap-max`) and retries once. Any second refusal: the seat ends the session at its third Stop, which the bound allows, without a handoff; the successor starts cold from the goal ledger (`goal next`), which is the authority wherever a snapshot is missing (the D3 contract), and the attention item names the refusal code. Unchanged in the verb: the state file schema fields and the 32 KB bound (D3-2), the nonce directory and `O_EXCL` write (D3-1), intent staging and digest verification (D3-4, D3-5), the authority rules C1a and C1b, and the tick's launch after the predecessor's death. Changed: the end-waits step and `openWork` (U3b); the binding's `successor` and the grace hold (U3c1, only under open question 1a).

The successor of an interactive seat is open question 1. Recommended: a fresh interactive `claude` in the same tmux pane, because that pane is where Wido's human verbs run. Units that change: pane relaunch builds U3c1, U3c2 and U3d; a headless successor drops all three, and the steward continuation the tick launches today reads `openWork` from the state file (U3b) and registers fresh.

### Q2. The stop gate sees harness-tracked work: the wait registry, with a wake source for every allowance

How the gate learns of live work: the wait registry only, made to match the runtime session by U1. The harness's own files are not a liveness source (`~/.claude/tasks/` on this box holds one directory from August; a delegate's `meta.json` says when it started, not whether it ended). Q4 reads those files after the fact; the verdict never does. Rejected: mtime liveness on transcript files; a Codex seat has none.

| work | how the gate sees it | liveness rule | wake source when the Stop is allowed |
|---|---|---|---|
| engine jobs (Codex builds, dispatched delegates, continuations) | `metasystem job watch` rows, kind job (today; U1 makes them match) | as today: waiter pid alive at its exact birth, heartbeat within 30 s on both clocks, deadline ahead (turnverdict.go:668-707) | the watch runs as the seat's background Bash; its exit is a harness notification |
| local work (engine runs, beds, landing drivers, the unit launcher) | `metasystem wait --local --label L -- <command>` forks the command and registers a kind `local` row whose `Target` carries the child's pid and birth; `--pid P` attaches to a running process (U2a) | eligible while the waiter and the child are both alive by the identity prober at their recorded births, plus the row rules above; the row turns terminal with the child's exit code, or `ended-unobserved` for an attached pid | the same: the wait verb's exit |
| a pending human question | the landed goal wait: `wait --goal G --event human-act --verb answer --question Q --after COMMIT` (wait_verb.go:118-121) | waiter alive, heartbeat, deadline; the answer ends it | the wait verb's exit |
| Claude Agent runs and Monitor | no row; not covered | the Stop is decided as if the run did not exist; the harness's own completion notification still wakes the seat | S3 moves any read longer than one step to a watched delegate job; Monitor is not used |

Allowing a Stop with no `systemMessage` is safe only when the harness will wake the seat when the work ends. The rule: an allowance is granted only for a row whose waiter is a process of this session's background Bash, so every end of the wait, normal or not, is a harness wake. Outcomes: the waiter crashes: its exit is the wake; the row is no longer pending, so the next Stop grants nothing; the seat re-registers (`job watch` for a job, `wait --local --pid` for a live child). The child exits before delivery: the waiter is the child's parent and observes the exit; for an attached pid the waiter polls liveness every 2 s and returns 0 with `ended-unobserved`; either way the wait verb exits and wakes the seat. An Agent run never registered: no allowance; the idle-backlog rule applies with its bound; its completion still wakes the seat. A human question wait suppresses open-work and unwatched blocks and prints its `WAITING` line, but never exempts the idle backlog: the standing order says the backlog never waits for the human (docs/orchestration.md), so the seat takes the next claimable goal, and stops only when nothing else is claimable.

The idle-backlog rule (R2): the bound of three refusals per unchanged backlog (turnverdict.go:1006, `refusal N of 3`) becomes two. The second refusal for the same digest runs `escalateIdleBacklog` exactly as the third does today (turnverdict.go:1037-1146: pick the goal, stage a steward continuation, record the incident, raise the alarm when the seat actor is unavailable) and allows the Stop. Third-Stop outcome for the same digest: allowed, no block, the standing notice `IDLE WITH BACKLOG: continuation staged at refusal 2`. Safety rule: the backlog is never stranded, because the staged continuation carries it and a changed digest restarts the count; a live eligible wait resets the count (:977) as today. Untouched, with today's outcomes: the unreadable-ledger uncertainty refusal (three, :935-963), unwatched work (block once per digest, :1184-1218), open work (once per marker or signature, :1404-1416), goal revisions (once per revision, :1451-1455). Expected: 103 refusals a day fall to at most two per distinct idle condition, about 20 across three seats; most conditions never refuse because the wait is live. Cost at the cap: 20 at 0.67M = 13.3M against 284.2M.

### Q3. Fewer wakes: one launcher per unit on the delegate path, no Monitor, short messages

One launcher per unit (U4): `metasystem unit run --plan <file>`, a Go verb that runs a unit's steps through the repository's own paths: dispatch the build (`metasystem delegate --role <role> --brief <file> --goal <id> --destructive-reach <class>`), wait on the job (the wait verb), prove the round (`metasystem job prove-round`), dispatch the reader as a fresh delegate with the bounded review brief of e700328e1, wait on it, then stop at the judgement pause: it writes artifacts/reports/<unit-id>-run.md with one verdict line per step and each step's report path, and prints exactly one line of at most 200 characters (unit, last step, verdict, report path). It never lands, retries or follows up: a red step ends the run, and the seat adjudicates the read's two-line return, lands with its own receipt, and certifies (docs/orchestration.md, Trust and Certification). The plan file names the unit id, goal, build brief, role, reach, reader brief, the two call budgets and the proof selection. The seat starts the launcher once as a background Bash under `metasystem wait --local`, and is woken once; the landing turn is the seat's own act. Rejected: one background task per step (61 background completions cost 204.9M); the seat as the sequencer; a Bash state machine beside the delegate path (revision 1; committed logic belongs in Go, development/project-rules-local.md:10, and dispatch owns the record, prompt and permissions).

Poll scripts and Monitor as separate background tasks: not allowed. A human answer is the goal wait, not a Monitor on a file. Monitor cost 56.5M today; expected after: 0.

Peer messages: the seat rule S4 first; the mechanism is U7 (an inbox file per seat that `message send` writes, the stop report counts, and the seat reads at its next turn start) under open question 3. Shortening a message does not remove its wake, so no saving is claimed for the rule alone: peers stay at 53 wakes at 0.53M = 27.9M at the cap. With U7 only a question that stops the sender is a `SendMessage`, about 10 a day: 5.3M.

The wakes goal's landed members are taken as given: the wait verb with typed exits, the publication hints, the stop gate honouring a registered wait. Its remaining legs (clause 5 and the two whole-goal legs) measure what U1, U2 and U4 deliver; U5's per-cause ledger gives clause 5 its tokens-while-pending figure. Expected notification cost: 25 wakes (one launcher per unit plus wait returns) at 4 calls at 155K = 15.5M against 299.7M.

### Q4. Measurement and the DONE thresholds

Scope (R5). The window is a half-open interval in a named zone: the baseline is 2026-09-15 00:00 to 12:45 Europe/Amsterdam across the seat roots m1b, m1c and m1e. The three seats are added as a plain sum of their per-call usage; an engine job is counted once, by job id, on the machine holding its record. Delegate transcripts under `<session>/subagents/` count into the seat that launched them, with the subtype from `meta.json` type (`code-critique`, `general-purpose`, other) and, for engine jobs, the role from the job record (`design`, `build`, `code-read`, `critique`, other). Cache reads stay a separate column. Landed units per seat-day: receipt lines in memory/receipts.log with `outcome=shipped` and the new optional field `seat=<machine>` (written by the landing receipt verb from the enrolled machine, like `read_tokens`) whose stamp falls in the window; each receipt is one landing, because a receipt is appended in the same commit as the work (project rule); `release` and `land-ready` are ledger verbs, not landings, and are never counted. The baseline count for 09-15 is produced by the same rule with the seat attributed through the goal's claimant at the receipt's stamp, replacing the diagnosis's estimate of about six for m1e.

What lands: U5a builds attribution into `internal/spend` (turn-starter cause per call by the diagnosis's rules, `queued_command` as a mid-turn arrival, recursive subagent discovery, a cursor cache schema bump), U5b the health line, per-model ceilings and landed units, U5c the verbs `spend window --from <RFC3339> --to <RFC3339> --zone <zone> --roots <a,b,c>` (per seat, cause, model, kind, subtype and landed units, from each root's cursor caches, which hold per-call timestamps), `spend compare --day <UTC day> --provider-claude-tokens <N>` (the Claude usage figure Wido reads from the provider console; deviation printed; pass at ten percent or less; Codex is a non-goal), and `spend reads --last 5` (the last five receipts carrying `read_tokens` and `read_calls`, the measurement code-reads-and-critiques-run-bounded handed to this goal). Health: the `spend-fence` reason gains `causes notification=.. stop_hook=.. peer=.. human=.. delegate=.. engine=..; models ..; units=N per-unit=..M; main-share=..%; ctx-mean=..K`; per-model lines `spend.ceiling.day.tokens.<canonical-model>` are their own crossings. The fence's seat scope widens to subagent files.

Expected day (same 12:45 window, three seats; calls per turn from the diagnosis's table 2d where unchanged; every call at the 155K mean of Q1; handoffs at one per 22 calls):

| class | baseline | cap only (Q1) | all units, peers by rule (no U7) | all units with U7 |
|---|---|---|---|---|
| notification | 299.7M | 88 wakes, 6.1 calls: 83.2M | 25 wakes, 4 calls: 15.5M | 15.5M |
| stop-hook | 284.2M | 103 at 0.67M: 68.6M | 20 at 0.67M: 13.3M | 13.3M |
| peer | 118.2M | 53 at 0.53M: 27.9M | 27.9M | 10 at 0.53M: 5.3M |
| human | 34.9M | 54 calls: 8.4M | 8.4M | 8.4M |
| compaction and usage-limit | 42.6M | 34 resume calls, no compaction: 5.3M | 5.3M | 5.3M |
| handoffs | 0 | 60 at 0.51M: 30.6M | 18 at 0.51M: 9.2M | 9.2M |
| delegates | 79.9M | 79.9M | 59.9M (the reads move to engine jobs; the sibling goals' cuts are not credited) | 59.9M |
| engine jobs | 12.2M | 12.2M | 32.2M | 32.2M |
| total | 871.7M | 316.1M | 171.7M | 149.1M |
| main-session share | 89.4 percent | 71 percent | 79.6M: 46.4 percent | 57.0M: 38.2 percent |

Revision 1's 179.6M kept the 20M of reads in both rows; the critique's corrected figure of 159.6M still carried a 15.8M peer saving no unit causes. Full day: the 09-14 ratio of the full day to its first 14 hours is 1.86 (1,443.0M over 776.7M); applied to a 12.75-hour forecast it understates a little (SSTB-119), so the full day is about 320M for three seats by rule, 280M with U7; the proof day measures the true ratio.

Thresholds, each measured by U5c and cross-checked by the diagnosis script on the same day:

| threshold | value | baseline in the same scope |
|---|---|---|
| three seats, 00:00 to 12:45 CEST | at most 200M | 871.7M |
| three seats, full working day; per seat | at most 400M; at most 150M per seat, subagent files included | 09-14 full day 1,443.0M |
| m1e's own fence figure at 13:00 CEST, UTC day, subagent files included | at most 100M | 316.3M (264.4M plus 51.9M) |
| main-session share of the three-seat window | at most 50 percent by rule; at most 40 percent with U7 | 89.4 percent |
| main-session context | every call at most the configured cap, no compaction, no usage-limit resume above it | mean 555K to 720K, peaks 966K, 3 compactions |
| tokens per landed unit, per seat-day | at most 20M | m1e 389.2M over its 09-15 receipts (about six: about 65M) |
| stop-hook refusals per seat-day | at most 10 | 41, 7 and 55 |
| the next five closing reads | each at most 2M tokens and 40 calls in `read_tokens` and `read_calls` | 16 to 24M over 106 to 165 calls; the first bounded read 96,634 over 23 |
| Claude provider comparison | U5's Claude day within ten percent of the provider figure | not measured |

### Q5. Seat practice from tomorrow

Each rule is checkable from the seat's transcript or a ledger. Rules that need no build apply from tomorrow; the later form of a rule waits for the named unit and is written into docs/orchestration.md by that unit.

| rule | today, no build | later, with the unit |
|---|---|---|
| S1 handoff size | when the `context-budget` health line shows the last call at or over 150K, the seat's next message runs `metasystem context handoff --root <installation>` alone and ends the session; on `HANDOFF_WAIT_IN_FLIGHT` the seat ends its background watch first (kill the waiter) and retries once; the successor is the steward continuation the tick launches, or the pane Wido restarts. Check: no main-thread call after the first sample at or over 150K except one handoff turn of at most three calls; `compact_boundary` records zero | the verdict's block does the telling (U3a2); the handoff ends waits itself (U3b); `context resume` first (U3c2) |
| S2 one background task per step | at most one `run_in_background` Bash per unit at a time, no Monitor, no `sleep` loop in a foreground Bash, every wait through `metasystem job watch` or `metasystem wait`. Check: Monitor uses zero; background launches per unit id at most one per step | one launcher per unit under `wait --local` (U2a, U4) |
| S3 delegates fresh and bounded | every design and revision is a fresh delegate (design-delegates-run-fresh-and-bounded, m1c); every code read is fresh with the bounded review brief (e700328e1); every Agent launch names its call budget and a page ceiling; critique rounds are follow-ups on the one critic chain (skills/design-critique/SKILL.md:64-67) and corrections use `delegate --follow-up`; no other `SendMessage` to a delegate. Check: `resumedAgentId` results zero outside critique chains; every Agent prompt contains the word budget | reads longer than one step run as delegate jobs inside the launcher (U4) |
| S4 messages | a `SendMessage` is at most 400 characters, one subject, sent at a unit boundary or for a question that stops the sender, never an acknowledgement; more goes by path. Check: length and count per unit | everything but a stopping question goes through the inbox (U7, open question 3) |
| S5 reads by path | the main session never reads a tool result over 20,000 characters (grep headings, `sed -n` ranges); a large output is read by a delegate from its persisted file. Check: results over 20,000 characters zero | unchanged |
| S6 first act | the first act of a session reads the `context-budget` health line and `goal next`. Check: the first tool calls | `metasystem context resume` first when a handoff waits (U3c2); the day's receipt names the seat's tokens and units (U5b) |

## 3. Units

Every unit is at most 300 changed lines (additions plus deletions, tests and docs included; docs/design/design-principles.md, Implementation Slicing), buildable by Codex from this page and its brief. Token effects are per day for three seats at the cap. Order is tokens saved per line built with dependencies respected: U6a costs no build; U1 turns the landed exemption on; U2a and U2c before U3 because a seat at the trigger usually holds a wait; the cap group U3a1, U3a2, U3b lands together; U3c and U3d wait for open question 1; U4 after U2a; U5 before the proof.

| id | goal | boundary (files) | ceiling | rules | expected token effect |
|---|---|---|---|---|---|
| U6a | 1:1 seats-spend-tokens-in-bounded-sessions | docs/orchestration.md (one paragraph: S1 to S6, today's forms), internal/goal/orchestration_rules_test.go (new) | 60 | R44 | S1 alone moves the mean context from 555K to 155K wherever the seat obeys; no machine effect claimed |
| U1 | 1:24 registered-wait-matches-the-runtime-session | members A, B1, B2, B3, C exactly as plans/registered-wait-matches-the-runtime-session-design.md states them, in that order, with its files, tests and ordering; this page adds nothing to them | that page's | that page's tests (A.5, B1.3, B2.4, B3.4, C) | refusals while a job wait is pending stop: about half of 103 refusals; 52 at 2.8M today = 145M; at the cap 35M |
| U2a1 | 1:23 stop-gate-sees-harness-tracked-work | internal/run/waiter_local.go (new: fork or attach, heartbeat, child exit result), internal/run/waiter.go (kind `local` in `ValidateWaitSelector` :400, `WaiterTarget` child pid and birth), cmd/metasystem/wait_verb.go (`--local`, `--label`, `--pid`, positional command after `--`; :72-88 refuses positionals today), tests | 280 | R1, R2, R3, R4 | the launcher's cover and the other half of the refusals: 51 at 0.67M = 34M |
| U2a2 | 1:23 | internal/goal/turnverdict.go (`registeredWaitSource` case local :710; child liveness in eligibility :668), internal/goal/turnverdict_test.go | 120 | R5, R6, R7 | same |
| U2b | 1:23 | internal/goal/turnverdict_test.go (human question wait rows), docs/orchestration.md (one sentence) | 80 | R8, R9 | proves the goal's human half on the landed goal wait |
| U2c | 1:23 | internal/goal/turnverdict.go (idle bound :996-1010), internal/goal/turnverdict_idle_test.go | 60 | R10, R11 | one refusal fewer per idle condition: about 20 at 0.67M = 13M |
| U3a1 | 1:10 coordinator-context-stays-under-budget, amendment 8d | internal/config/context.go (new: the two keys), internal/steward/context.go (constants become defaults from config), internal/steward/contextreport.go (reads the keys), scripts/agents/supervision-hook.sh (`transcript_path` to `--transcript`, `--runtime`), cmd/metasystem/goal.go (flags, `ContextSample`), tests | 220 | R12, R13, R14 | enables the trigger |
| U3a2 | 1:10, 8d | internal/goal/turnverdict.go (context block, two per session, precedence), internal/report/stoppresentation.go (the sentence), tests | 200 | R15, R16, R17, R18 | main sessions 779.5M to about 230M at 155K mean (cap-only column: 871.7M to 316.1M) |
| U3b | 1:10, 8d | internal/steward/handoff_capture.go (end-waits step, `openWork` capture), internal/steward/handoff_state.go (`openWork` on state and manifest, strict decoder :139-151, validation :289-365, limits), internal/run/waiter.go (`EndWait`: signal and row state under the waiter lock), tests | 300 | R19, R20, R21, R22 | enables handoffs under Q2; without it every seat at the trigger with a live wait exits 9 |
| U3c1 | new goal seat-relaunches-into-its-handoff, tier 2 (open question 1a only) | internal/steward/handoff.go (`Successor` on `HandoffBinding` :23-35), handoff_state.go (validate), handoff_capture.go (write), cmd/metasystem/context_verbs.go (`--successor`), internal/steward/revive.go (`decideForRevival` holds an interactive intent for `HandoffRelaunchGrace = 15m`), tests | 200 | R23, R24 | enables the cap for interactive seats |
| U3c2 | seat-relaunches-into-its-handoff (1a only) | cmd/metasystem/context_verbs.go (`context resume`, `context pending`), internal/steward/intervene.go (consume by nonce for a dead predecessor), scripts/agents/supervision-hook.sh (SessionStart prints `context pending`), tests | 250 | R25, R26, R27 | each handoff costs 0.51M instead of a session that grows to 966K |
| U3d | seat-relaunches-into-its-handoff (1a only) | cmd/metasystem/context_verbs.go (`context relaunch-plan`: the decision), scripts/agents/seat-relaunch.sh (tmux plumbing only: send `/exit`, wait for the announced pid's death, start `claude` in the pane), tests | 160 | R28 | same |
| U4 | new goal units-run-through-one-launcher, tier 2 | internal/unit/plan.go (new), internal/unit/run.go (new: steps behind a dispatch seam), cmd/metasystem/unit_verbs.go (new), tests with a fake seam | 300 | R29, R30, R31, R32 | notifications 83.2M (cap only) to 15.5M; Monitor 0; reads move to fresh engine jobs |
| U5a1 | 1:5 spend-fence-reports-tokens-per-model-and-cause | internal/spend/cache.go (schema bump; `Cause`, `Kind`, `Subtype`, `TurnStarter` per cached request :28-51), internal/spend/transcript.go (recursive `subagents/` discovery :43-95, turn-starter scan :304, `meta.json` type), tests with a synthetic transcript tree | 280 | R33, R34 | 0; makes every other number measurable daily |
| U5a2 | 1:5 | internal/spend/measure.go (`Row.Cause`, `Kind`, `Subtype`; ledger `Causes`, `Models`, `Kinds`), tests | 200 | R35, R36 | 0 |
| U5b | 1:5 | internal/steward/health.go (reason line), internal/config/spend.go (per-model ceiling keys), internal/spend/measure.go (landed units from receipts), internal/receipt/receipt.go and cmd/metasystem/receipt_verbs.go (`seat` field), tests | 280 | R37, R38, R39, R40 | 0 |
| U5c | 1:5 | internal/spend/window.go (new), cmd/metasystem/spend_verbs.go (new: `window`, `compare`, `reads`), tests | 300 | R41, R42, R43 | 0; the proof's instrument |
| U7 | 1:1 (open question 3b only) | cmd/metasystem/message_verbs.go (new: `message send`, `message read`), internal/goal/turnverdict.go (unread count in the display), tests | 200 | R45 | peers 27.9M to 5.3M |
| proof | 1:1 | records/misc/tokens-day-<date>.md: one working day of three seats through `spend window`, the diagnosis script re-run, `spend compare`, `spend reads --last 5` | record | the Q4 thresholds | the DONE |

## 4. Open questions for Wido (at most three, unanswered at revision 2)

1. The successor of an interactive seat. Recommended (a): a fresh interactive `claude` in the same tmux pane through U3c1, U3c2 and U3d, because the pane is where human verbs and approvals run. Alternative (b): the landed headless steward continuation is the successor and the pane holds a human shell; U3c1, U3c2 and U3d are dropped, U3b's `openWork` is what the continuation reads. Every other unit builds under either answer.
2. The cap. Recommended 200K (Q1's table). Alternative 300K if a relaunch every 22 calls is too much friction for the pane; it costs 28M a day per seat more at today's calls and is one configuration line, no unit.
3. Peer messages. Recommended (a): the seat rule S4 and one measured day under U5c before any mechanism; the thresholds then read "by rule". Alternative (b): build U7 now; the thresholds then read "with U7". Nothing else changes under either answer.

## 5. Verification rows

Each rule names the focused Go witness that fails when that rule alone is removed; a builder runs it with `go test -run` and no fixture bed, no live session and no tmux. Shell legs and live runs are proof of DONE, recorded in the proof record, never a rule's witness. U1's witnesses are the registered-wait page's.

| rule | statement | witness |
|---|---|---|
| R1 | `wait --local --label L -- <command>` forks the command, registers a schema 2 row of kind `local` whose `Target` carries the child's pid and birth, heartbeats every 10 s, returns the child's exit code and turns the row terminal with it | TestLocalWaitRunsAndReturnsTheChildExit (internal/run) |
| R2 | `wait --local --pid P` attaches to a live pid at its recorded birth, polls every 2 s, returns 0 with `ended-unobserved` when it dies; a dead or reused pid is refused at registration | TestLocalWaitAttachesToAPid |
| R3 | positional arguments are accepted only with `--local`; `--local` with another selector or `--resume` is refused | TestWaitLocalParser (cmd/metasystem) |
| R4 | a local deadline returns 124 and never signals the child | TestLocalWaitDeadlineLeavesTheChild |
| R5 | an eligible local wait allows the Stop, resets the idle count and prints `WAITING: registered wait <id> covers local <label> until <deadline>` | TestLocalWaitAllowsTheStop (internal/goal) |
| R6 | a local wait whose child is dead or whose pid was reused (birth differs) is ineligible | TestDeadLocalChildDoesNotAllowTheStop |
| R7 | a local wait whose heartbeat is older than 30 s on either clock is ineligible | TestStaleLocalHeartbeatDoesNotAllowTheStop |
| R8 | a live human question wait suppresses the open-work and unwatched blocks, prints its `WAITING` line, and leaves the idle-backlog decision unchanged | TestHumanQuestionWaitAllowsTheStop |
| R9 | a human question wait whose waiter is dead or whose answer landed grants nothing; the verdict is today's | TestDeadHumanQuestionWaitDoesNotAllowTheStop |
| R10 | the second idle refusal for an unchanged digest escalates as the third does today and allows the Stop; the third prints the standing notice and no block; a changed digest restarts the count | TestIdleBacklogEscalatesAtTwo |
| R11 | the uncertainty, unwatched-work, open-work and goal-revision bounds are unchanged | TestOtherBlockSourcesKeepTheirBounds |
| R12 | `context.ceiling.tokens` and `context.handoff.margin.tokens` default to 200000 and 50000, must be positive with the margin under the ceiling, and feed `ContextBudgetLine` and the context proof | TestContextCeilingConfig (internal/config, internal/steward) |
| R13 | `report turn-verdict --transcript P --runtime R` fills `ContextSample` from that path through `usage.LatestCall`; the hook text passes `transcript_path` to it | TestReportTurnVerdictReadsTheTranscriptFlag; TestHookPassesTheTranscriptPath (reads the script text) |
| R14 | a missing, unreadable or per-invocation sample prints `CONTEXT: unknown (<reason>)` and never blocks | TestContextSampleUnknownNeverBlocks |
| R15 | a sample at or over the trigger line with no live handoff blocks with class `context-over-trigger` and the one sentence, ahead of idle and open-work blocks | TestTurnVerdictBlocksAtTheTriggerLine |
| R16 | the block is counted per session; the third Stop is allowed with the standing notice and raises the `context-over-trigger` attention item | TestContextBlockIsBoundedToTwoPerSession |
| R17 | with a live handoff for the session the allowance `handoff recorded: <nonce>; end this session` wins | TestContextRuleYieldsToARecordedHandoff |
| R18 | under the trigger line the verdict is unchanged | TestContextRuleSilentUnderTheTrigger |
| R19 | the handoff ends every `registering` or `pending` row of this main (signal; row `interrupted`), records each under `openWork` with kind, target, deadline left and a fresh registration command, never `--resume`, and then passes `waiterInFlight` | TestHandoffEndsWaitsAndRecordsOpenWork (internal/steward, injected signal seam) |
| R20 | a `registering` row is waited for up to 10 s on the injected clock, then treated as pending | TestHandoffWaitsForARegisteringRow |
| R21 | a job with status `pending` still refuses `HANDOFF_LAUNCH_IN_FLIGHT job=<id>` | TestHandoffRefusals (existing row, kept) |
| R22 | the strict decoder accepts `openWork`; rows over the inline limit overflow to the manifest; the 32 KB bound holds; a row without a fresh command is invalid | TestHandoffStateCarriesOpenWork |
| R23 | the binding carries `successor` `interactive` or `headless`, validated and written by the verb's `--successor` flag (default from the runtime's pane presence) | TestHandoffBindingSuccessor |
| R24 | `decideForRevival` holds an `interactive` intent for 15 minutes after the predecessor's observed death, then launches; a `headless` intent launches at once as today | TestDecideForRevivalHoldsForTheRelaunchGrace |
| R25 | `context resume` on a live intent for this machine whose predecessor is dead verifies the digest, consumes the intent, prints the state path and the `openWork` commands, exit 0 | TestContextResumeConsumesTheHandoff |
| R26 | `context resume` while the predecessor lives refuses `HANDOFF_PREDECESSOR_ALIVE` and consumes nothing; with no live intent it prints `no handoff`, exit 0 | TestContextResumeRefusesWhileThePredecessorLives |
| R27 | `context pending` prints `handoff <nonce> waits for this seat: run metasystem context resume --root <installation> first` when a live intent names this machine, else nothing; the SessionStart hook text calls it | TestContextPendingPrintsTheFirstAct; TestHookCallsContextPending (reads the script text) |
| R28 | `context relaunch-plan` refuses with `no live handoff for this seat` when no intent names the seat; with one it prints the pane, cwd and command and nothing else | TestContextRelaunchPlan |
| R29 | `unit run` executes the plan's steps in order through the dispatch seam, stops at the first red step, and writes that step's verdict line and report path | TestUnitRunStopsAtTheFirstRed (internal/unit) |
| R30 | the verb's stdout is one line of at most 200 characters naming unit, last step, verdict and report path | TestUnitRunSummaryLine |
| R31 | the run ends at the judgement pause: no landing verb is invoked; the reader is a fresh dispatch carrying the review brief's call budget, never a follow-up | TestUnitRunNeverLandsAndReadsFresh |
| R32 | a plan missing a field, or naming a brief that does not exist, is refused before any dispatch | TestUnitPlanValidation |
| R33 | the cursor cache schema bump invalidates old cursors; `subagents/agent-*.jsonl` files are discovered under each session and their `meta.json` type recorded | TestSpendCursorSchemaAndSubagentDiscovery (internal/spend) |
| R34 | every main-session call is attributed to its turn starter's cause by the diagnosis's rules; a `queued_command` attachment counts as a mid-turn arrival of its cause | TestSpendAttributesCallsToTurnStarters |
| R35 | the ledger's `Causes`, `Models` and `Kinds` rows each sum to the day scope's all-in | TestCauseModelKindRowsSumToTheDay |
| R36 | a delegate's subtype is its `meta.json` type; an engine job's is its role (design, build, code-read, critique, other) | TestDelegateSubtypes |
| R37 | the `spend-fence` reason carries causes, models, units, per-unit, main-share and mean-context in the stated order | TestHealthLineCarriesCausesAndUnits |
| R38 | a per-model ceiling crossing is its own `SpendCrossing` with scope `model-<canonical>` and alerts alone | TestPerModelCeilingCrossesAlone |
| R39 | landed units per seat-day count `outcome=shipped` receipts with `seat=<machine>` in the window, one per receipt; `release` and `land-ready` History rows are never counted; zero prints `n/a` | TestLandedUnitsCountReceiptsOnce |
| R40 | the receipt `seat` field is optional, validated as a machine name, and a receipt without it serialises as before | TestReceiptSeatField (internal/receipt) |
| R41 | `spend window` sums the named roots over `[from, to)` in the named zone, per seat, cause, model, kind and subtype, counting an engine job once by id | TestSpendWindowSumsThreeRoots |
| R42 | `spend compare` prints the deviation of the Claude day from the provider figure and passes at ten percent or less | TestSpendCompareTolerance |
| R43 | `spend reads --last 5` prints the last five receipts carrying `read_tokens` and `read_calls` and flags any over 2M or 40 | TestSpendReadsLastFive |
| R44 | docs/orchestration.md carries S1 to S6 in their today forms | TestOrchestrationCarriesSeatRules (reads the document) |
| R45 | `message send` appends to the receiver's inbox file; the stop display counts unread messages; `message read` marks them | TestInboxSendAndCount |

## 6. Critique record, round 1

| finding | disposition |
|---|---|
| SSTB-101 | folded: U1 is the registered-wait design's members A, B1, B2, B3, C, unchanged and in order; the revision 1 resolver sentence is gone |
| SSTB-102 | folded: Q1 triggers at the ceiling minus the margin, the hook passes `transcript_path`, the verb reads the sample as `ContextBudgetLine` does, an unknown sample never blocks; U3a1 owns the wiring |
| SSTB-103 | folded: Q1's end-waits step, the `openWork` record, the recovery after exit 9, and a per-session bound instead of a size band |
| SSTB-104 | folded: Q2 changes only the idle-backlog bound and states the third-Stop outcome and safety rule; the other four policies are named as untouched (R11) |
| SSTB-105 | folded: the cap is configuration; open question 1 names the units that change per answer; U7 owns the inbox under open question 3 |
| SSTB-106 | folded: U2a1 owns wait_verb.go and the selector; U2b proves the human question half on the landed goal wait |
| SSTB-107 | folded: Q2's wake-source rule and the outcomes for a waiter crash, an early child exit and an unregistered Agent run |
| SSTB-108 | folded: U3b owns handoff_state.go; `openWork` carries fresh registration commands, never `--resume` (B3.3) |
| SSTB-109 | folded: U4 is `unit run`, a Go verb on `metasystem delegate` with a judgement pause and no landing step |
| SSTB-110 | folded: Q5 splits today and later; S3 keeps the critique chain and follow-ups; S6 runs `context resume` first once U3c2 lands |
| SSTB-111 | folded: Q4's window, zone, fleet sum, same-scope baseline (316.3M for m1e) and U5c |
| SSTB-112 | folded: the expected day is 171.7M by rule and 149.1M with U7; the main share and its thresholds are recomputed |
| SSTB-113 | folded: no saving is claimed for the rule alone; U7 is the mechanism and the peer row shows both answers |
| SSTB-114 | folded: landed units are shipped receipts with a `seat` field, one per landing; `release` and `land-ready` never count; the baseline count is produced by the same rule |
| SSTB-115 | folded: delegate subtypes (R36), the provider comparison (R42) and the five-read measurement (R43) |
| SSTB-116 | folded: U3c1 names handoff.go, intervene.go, stage.go's binding path and revive.go; U5a is split with cache.go in U5a1 |
| SSTB-117 | folded: every rule has a Go witness; the hook and driver legs are DONE proof only |
| SSTB-118 | folded: the anchors now read `contextVerdict` :293, `OpenWorkSignature` :112, the refusal text :1006, and metasystem.conf:27-35 as a commented default |
| SSTB-119 | folded in one line under the expected day: the 1.86 factor understates a 12.75-hour window; the proof day measures the ratio |

## 7. Records

Evidence and method: records/misc/token-diagnosis-2026-09-15.md and its script records/misc/token-diagnosis-2026-09-15.py (re-run for the proof day with the same window rules); the first bounded read's receipt in memory/receipts.log (e700328e1). Not read within the call budget: internal/run/waiter.go beyond the `Waiter` struct, the selector validator and the interrupt paths; the diagnosis script's classifier beyond the lines the critique cites; the wakes design's fixture rows beyond section 5's table; cmd/metasystem/delegate.go beyond its entry.
