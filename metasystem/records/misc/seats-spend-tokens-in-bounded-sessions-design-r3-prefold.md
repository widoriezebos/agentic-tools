# seats-spend-tokens-in-bounded-sessions

Goal 1:1 (plans/goals/seats-spend-tokens-in-bounded-sessions.md), revision 3 of the design, written by a fresh Claude Fable delegate from artifacts/reports/tokens-design-brief-r3.md on 2026-09-15 at trunk a1d23839f. Paths are relative to the metasystem module. Line numbers name that tree.

Changed from revision 2, after Codex critique round 2 (artifacts/reports/tokens-design-critique-r2.md, 10 material findings, 11 round-1 findings reopened), the binding facts F1 and F2 and the seat rulings R8 to R14:
- This is now the program page. It decides seven program questions, P1 to P7, and maps units to their owning goals. Boundary file lists, witness names and per-unit ceilings are gone; each owning goal designs its own units from section 3 (R8).
- The successor is automatic: the steward continuation launches while the predecessor lives, the launch is confirmed by a named signal, and no path ends a session without a successor (R9, F1, F2). Revision 2's lines 54, 56, 58, 60, 75 and 130 are replaced.
- The trigger's in-turn overshoot is stated as a number; the harness installs no tool-use hook today (R10). One open question asks whether to install one.
- Measurement has one scope, Claude only, with an engine-window rule, a checked baseline, a landing identity with dedup and a semantic delegate kind (R11). The forecast counts sessions to the trigger with one handoff cost and recomputes every threshold, per seat too (R12).
- Today's seat rules say what a seat does over the trigger with no build: keep working, never stop (R13). Three anchors are fixed (R14).

DONE, restated under F1: a seat's main session stays under the configured cap (default 200 thousand tokens of context) by construction. Over the trigger line it records a handoff, the steward starts a successor while the seat still lives, and the seat keeps working through bounded delegates until the successor is confirmed. A limit is never a stop and never a wait on Wido. Long pipelines and long reads run in bounded delegates through one launcher per unit; waiting on registered background work costs no stop-hook turn; Claude tokens per seat per day are split by cause, model and kind and divided by landed units in health; and one working day of three seats lands under the thresholds of P5 against the 2026-09-15 baseline.

## 1. Evidence

From records/misc/token-diagnosis-2026-09-15.md (Claude tokens only, Europe/Amsterdam day from 00:00 to 12:51, three seats):

| fact | number |
|---|---|
| three-seat day; per seat | 871.7M; m1b 317.0M, m1c 165.4M, m1e 389.2M; cache reads 96.7 to 98.6 percent |
| main interactive sessions | 779.5M (89.4 percent); calls at 400K context or more carry 696.6M of that |
| notification turns | 88 turns, 538 calls, 299.7M; 6.1 calls per wake at 555K mean context; background Bash 211.6M, Monitor 56.4M, Agent finished 31.6M |
| stop-hook refusal turns | 103 turns, 439 calls, 284.2M; 4.3 calls per refusal at 646K |
| peer message turns | 53 turns, 181 calls, 118.2M; 3.4 calls at 652K |
| human turns | 19 turns, 54 calls, 34.9M at 644K |
| compaction and usage-limit resumes | 3 turns of 41.3 calls and 3 turns of 11.3 calls; 158 calls, 42.6M |
| delegates; Claude engine jobs | 79.9M (478 calls at 164K; m1c 28.0M, m1e 51.9M); 12.2M (100 calls at 117K, all m1b) |
| main calls per seat | m1b 484, m1c 217, m1e 669 (40 Fable, 629 Opus); 1,370 in all |
| largest session | m1e main 36c93128: 653 calls, 335.8M, peak 966K, 2 compactions |
| context growth; fresh-session floor | about 4K tokens per call; about 110K after boot (9292cf37: 16 calls at 122K mean) |
| the fence | m1e seat dayTokens 264.4M at 13:00 CEST (top-level main transcripts, UTC day); with m1e's 51.9M of subagent files 316.3M; the fence's 58.1M of engine records on m1e are not Claude (section 4 of the diagnosis lists no m1e engine row) |
| the first bounded read | receipt 1789472212 (e700328e1): read_tokens=96634, read_calls=23, against 16 to 24M over 106 to 165 calls before |

F2, m1c's live evidence: `metasystem context handoff` recorded three handoffs on 2026-09-15 (12:20, 14:30, 15:05, nonce aef24bbc5d8173d2) and no successor appeared; the health line stayed dead at 287K and then 327K; the Stop hook kept refusing on idle-backlog. Cause, in the tree: `decideForHandoff` holds the intent while the predecessor pid lives (internal/steward/handoff.go:93-100) and again while any seat main lives (:101-114), and an interactive seat cannot exit itself. The tree already admits the shape this page needs: a `seatIdle` intent launches a continuation beside a live main, bypassing "only the ordinary live-main suppression" (internal/steward/revive.go:301-337). A launch is `metasystem delegate --revive <nonce>` in its own session (cmd/metasystem/steward_verbs.go:493-509), after `ConsumeIntent` and before `StampLaunch` (revive.go:186-205; intervene.go:219-235). The runner ticks every ten minutes by default (`TickSeconds`, internal/steward/runner.go:96-97). The Stop gate allows a Stop under a live handoff with the display `handoff recorded: <nonce>; end this session` (internal/goal/turnverdict.go:393-411), where live means staged and not yet consumed (`LiveHandoffForSession`, cmd/metasystem/goal.go:710); after consumption the allowance is gone, which is F2's idle-backlog loop. The installed hooks are SessionStart, SessionEnd and Stop only (../.claude/settings.json); no tool-use event reaches scripts/agents/supervision-hook.sh.

## 2. Program decisions

### P1. The trigger and its bound

The cap is configuration: `context.ceiling.tokens` (default 200000) and `context.handoff.margin.tokens` (default 50000), read by `internal/config`; the trigger line is the ceiling minus the margin, 150K by default. The sample is the last main-thread call's prompt tokens, read from the Stop payload's `transcript_path` through `usage.LatestCall` with the cursor `ContextBudgetLine` already shares (internal/steward/context.go:95-126). An unknown or unreadable sample never blocks; the verdict prints `CONTEXT: unknown (<reason>)`.

The bound. The trigger is observed at Stop time, so one turn can carry the sample past the trigger before the gate sees it. No tool-use hook is installed and the supervision hook receives none, so growth inside a turn is bounded only by turn length. The number: growth is 4K per call (diagnosis, m1c's 217 calls to 869K). Turns of the causes that survive the cap average 2.8 to 6.1 calls; the longest surviving turn class is the usage-limit continuation at 11.3 calls; the compaction continuation at 41.3 calls does not exist under the cap, because no compaction fires under 200K. Maximum overshoot at 12 calls: 48K, so a Stop sample of at most 198K. Maximum overshoot at the observed worst turn of 42 calls: 168K, so 318K. What DONE's "under a stated context size" then means: every Stop-time sample is under 200K by construction; every call is under 200K only while turns hold at 12 main-thread calls (rule S2 in P6); the landed proof's maximum rule (internal/steward/contextreport.go:566-569) reports any call over the ceiling and fails the proof. Whether that is enough is open question 4; the recommendation there installs a tool-use hook (U3e) that denies any tool call but the handoff over the trigger, which bounds every call at 150K plus three calls, 162K.

### P2. The successor start and the predecessor's end

The launcher: the steward continuation, launched by the existing `delegate --revive` path, with the two holds in handoff.go changed. Rejected: a pane relaunch, because it needs the predecessor to die first and Wido's pane is not a launcher; a new headless launcher beside the continuation, because the continuation already carries the brief, the role, custody, the reaper and the dry-revival cap.

What changes in handoff.go:93-101. For a `seatHandoff` intent the predecessor's liveness is no longer a hold: `handoffPredecessorLiveness` is read for the record only. The census hold at :101 counts seat mains other than the predecessor's pid (`binding.Predecessor`), so the predecessor's own main does not hold its successor; `CensusComplete`, `Untracked` and `Unprovable` keep holding. The `others > 0` hold at :117 excludes the consumed intent whose `JobId` equals `binding.PredecessorJob`, so a continuation can be succeeded by the next continuation. The dry-revival cap and the provider-outage hold are unchanged. The role's rule 2 (scripts/agents/roles/steward-continuation.md:20-22, yield when a live session is mid-work) gains one sentence: under a handoff nonce the predecessor is expected to be alive and idle, and the successor never yields to it. The staged brief (internal/steward/stage.go:153-164) tells the successor to run `metasystem context resume --root <root> --nonce <nonce>` first.

Numbers. The handoff verb records the state, stages the intent, and then runs the revival itself (`CompleteRevival` with the launch seam of steward_verbs.go:493-509) instead of waiting for a tick; the runner's tick, every 600 seconds by default, is the retry. The launch takes seconds; the successor's boot call is about 110K; the verb waits up to 30 seconds for the confirmation.

The confirmation signal, named and observable: the consumed intent record carries `launchStamped: true` (intervene.go:219-235) and the continuation's job record `<jobId>` is at status running with a pid the identity prober confirms. `steward status` prints `continuation <jobId> running for handoff <nonce>`; the handoff verb prints `successor running: job=<jobId> pid=<pid>` and exits 0. Until the signal exists the seat is not handed off.

Claims and lineage. The goal claim belongs to the machine, so the continuation on the same machine works under the same claim with no ledger verb (`handoffGoalIsStillOwned`, handoff.go:54-66). The intent's `JobId` is the successor's job record; its custody names the handoff nonce and the binding's predecessor session, main id and pid; the successor's receipts carry the same seat. The chain closes at the reap (internal/steward/reap.go:33-57). The authority rules for cancelling a handoff (C1a, C1b of plans/coordinator-context-stays-under-budget-design.md) are unchanged.

The live predecessor after the confirmation. It is handed off: the gate marks the session (a `handedOff` nonce on the session state at the first allowed Stop) and every later Stop of that session is allowed with no display and no `systemMessage`, whatever the backlog. The idle-backlog, open-work, unwatched, uncertainty and goal-revision branches are skipped for a handed-off session; it is never refused on idle-backlog again. The `HandoffRecorded` closure reads live intents and consumed intents whose binding names the session, so the allowance survives consumption and the reap. It does no work. Its background waiters were ended by the handoff (below); their exit wakes it once, and that turn makes one call and stops. It stays in the pane as the seat's human console: a human message costs one turn; the predecessor answers in one line naming the successor's job id and relays the text through the inbox of U7. Whether a headless session receives a `SendMessage` is unverified; the inbox does not depend on it, which is one reason open question 3 recommends U7.

Waits. The handoff ends every `registering` or `pending` row of the session (signal; the row turns `interrupted`) and records each under `openWork`. The rule: `openWork` carries each ended row's complete `WaitSelector` (kind, targetId, goalId, event, after, verb, question, chain, poll; internal/run/waiter.go:55-68) with the deadline left and, for a local wait, the child's pid and birth, and the successor's first act, `context resume`, registers each as a fresh row through the wait verb; it never records or runs `wait --resume`. U3b writes the record; U3c registers it.

No path ends a session with no successor started. The over-trigger block: the first over-trigger Stop with no live handoff blocks once with the sentence `CONTEXT AT <N>K OF <C>K: run metasystem context handoff --root <installation> alone in your next message` (two calls). At the second over-trigger Stop with no handoff the gate stages and launches the handoff itself through the capture seam, as `PrepareIdleContinuation` stages a `seatIdle` intent today, and allows the Stop with `handoff recorded`. A refusal on a wait in flight cannot occur, because the capture ends the waits first; `HANDOFF_LAUNCH_IN_FLIGHT` (a dispatch mid-write) is answered by one more block, bounded by the dispatch's own seconds. While the successor is not confirmed, the gate keeps blocking over the trigger with the same sentence, every 600 seconds the tick retries the revival, and the second block raises the attention item `handoff-successor-missing`; there is no bound on these blocks, because a bound would be a stop (F1), and each block costs two calls at the frozen size. The one path that ends with no successor is an infrastructure failure of the capture itself, which the gate reports as `infrastructureVerdict("handoff-record")` does today (turnverdict.go:396) with the attention item `context-handoff-failed`; it is a fault, not a policy. The revision 2 paths at its lines 54 (a third Stop allowed without a successor), 56 (the last wake ends the session), 58 (a cold successor from the ledger after exit 9), 60 (a pane relaunch), 75 (an idle escalation whose continuation the pid hold never launched) and 130 (S1 ends the session) are gone.

The successor is a headless session under the same hooks, so the same trigger applies to it; when it reaches the trigger it hands off the same way, and it may end its own session once its successor is confirmed. A chain of continuations is the normal shape of a seat's day.

### P3. The stop-gate policy

R2 stands: the refusal policies stay separate; only the idle-backlog bound changes. Changes P2 needs, and no others: the handed-off allowance above; the idle-backlog bound of three (turnverdict.go:991-1010, `refusal N of 3`) becomes two, and the second refusal for an unchanged digest runs `escalateIdleBacklog` (:1037-1146) whose `seatIdle` continuation now launches beside the live seat (revive.go:301-337 already admits it) and marks the session handed off; the third-Stop outcome for that digest is an allowed silent Stop. Safety rule: the backlog is never stranded, because the continuation holds it and the seat is idle by its own act. An allowance for a registered wait is granted only for a row whose waiter is a process of this session's background Bash, so every end of the wait is a harness wake; the outcomes for a crashed waiter, an early child exit and an unregistered Agent run are revision 2's, unchanged. The ten-refusal threshold counts blocks of every class except `context-over-trigger`, which is a handoff, not a refusal.

### P4. The measurement scope

One runtime scope: Claude tokens only. Engine-window rule: an engine job counts in the window that contains its job record's `startedAt`, with all of its tokens, and only when the job record's runtime is a Claude runtime; a Codex job is skipped by that field, and the fence's fleet metric adopts the same filter. Kinds: `main` is the seat's coordinator, which is the interactive main and every steward continuation for the seat; `delegate` is a subagent file; `engine` is any other Claude job. The window is a half-open interval in a named zone. The baseline is 2026-09-15 00:00 to 12:51 Europe/Amsterdam (the diagnosis's generation time, 10:51Z; revision 2's 12:45 was wrong) across the seat roots m1b, m1c and m1e, and its figure in this scope is 871.7M: the diagnosis counted Claude only and included subagents and m1b's Claude engine jobs. The critique's 374.4M for m1e is the any-runtime figure (264.4M main, 58.1M engine records, 51.9M subagents); under the Claude-only scope m1e's engine part on 09-15 is zero, so the same-scope m1e baseline is 316.3M. Both baselines are re-read by the verb over the baseline interval before the proof day, and a deviation over 2 percent is a defect in the spend-fence build, not a new baseline.

Who owns the day and the endpoints: `spend window --from <RFC3339> --to <RFC3339> --zone <zone> --roots <a,b,c>` owns both endpoints and the roots; the diagnosis script (records/misc/token-diagnosis-2026-09-15.py) is a prototype whose figure seeded the baseline and it is not rerun for DONE.

Landing identity with dedup: a landed unit is a landing commit, the commit that appended one or more `outcome=shipped` receipts of a landing kind to memory/receipts.log. The count per seat-day is the number of distinct landing commits attributed to the seat whose commit time falls in the window. Several receipts from one commit count once; a revival receipt is not a landing and does not add; a goal History row (`release`, `land-ready`) is never a landing. The receipt gains a `seat` field; the 09-15 baseline attributes through the goal's claimant at the receipt's stamp. Field-level detail is the spend-fence goal's (section 3).

Delegate kind, a semantic source that does not rely on `meta.json`: the launch declaration. For an engine job it is the job record's role, mapped to design, build read, critique or other. For a native delegate it is the first line of the delegate's prompt, `Kind: design | build-read | critique | other`, which rule S3 requires and the spend reader parses from the subagent transcript's first user message; a missing line is `other` and a count of missing lines is shown in health.

### P5. The forecast and the DONE thresholds

One handoff cost everywhere: two calls at 150K for the handoff turn, one boot call at 110K, one last wake of the predecessor at about 158K; 0.58M. Sessions are counted to the trigger, not the ceiling.

| step | by rule (no U7) | with U7 |
|---|---|---|
| main calls after the units: notification 25 wakes at 4, stop-hook 20 at 4.3, peer 53 or 10 at 3.4, human 54, resumes 34 | 100 + 86 + 180 + 54 + 34 = 454 | 100 + 86 + 34 + 54 + 34 = 308 |
| growth calls per session: (150K minus 110K) over 4K | 10 | 10 |
| sessions, so handoffs | 45.4 | 30.8 |
| mean context of a work call: (110K + 150K) over 2; the in-turn overshoot adds under 5 percent | 130K | 130K |
| work tokens: calls at 0.13M | 59.0M | 40.0M |
| handoff tokens: handoffs at 0.58M | 26.3M | 17.9M |
| main total (interactive main plus continuations) | 85.3M | 57.9M |
| delegates and engine jobs: 79.9M plus 12.2M; the moved reads change rows, not the sum; the sibling goals' cuts are not credited | 92.1M | 92.1M |
| three-seat window, 00:00 to 12:51 | 177.4M | 150.0M |
| main share | 48.1 percent | 38.6 percent |
| full day, at the 09-14 ratio of 1.86 | 330M | 279M |

Per seat, full day, main by each seat's share of main calls (m1e 48.8, m1b 35.3, m1c 15.8 percent) and delegates by each seat's own baseline: m1e 174M by rule and 149M with U7; m1b 79M and 61M; m1c 77M and 69M. The cap: at 250K the main total is 85.1M by rule with half the handoffs; at 300K it is 93.1M; 200K stays (open question 2).

| threshold | value | forecast by rule; with U7 | baseline, same scope |
|---|---|---|---|
| three seats, 00:00 to 12:51 CEST | at most 200M | 177.4M; 150.0M | 871.7M |
| three seats, full working day | at most 400M | 330M; 279M | 09-14 full day 1,443.0M |
| each seat, full working day, subagent files included | at most 150M | m1e 174M; 149M | m1e 389.2M to 12:51 |
| m1e's own fence figure at 13:00 CEST, UTC day, Claude only, subagent files included | at most 100M | about 80M; about 69M | 316.3M |
| main-session share of the three-seat window | at most 50 percent by rule; at most 40 percent with U7 | 48.1; 38.6 percent | 89.4 percent |
| main-session context | every Stop sample under the cap; every call under the cap under S2, or by U3e; no compaction, no usage-limit resume above it | 198K worst Stop sample at 12 calls | peaks 966K, 3 compactions |
| tokens per landed unit, per seat-day | at most 20M | m1e at 8 landings: 21.7M; 18.6M | about 65M |
| stop-hook refusals per seat-day, `context-over-trigger` excluded | at most 10 | two per idle digest, a digest per landing or claim: at most 10 at 4 idle conditions | 41, 7, 55 |
| the next five closing reads | each at most 2M tokens and 40 calls | measured by `spend reads` | 16 to 24M over 106 to 165 calls |
| Claude provider comparison | the verb's Claude day within ten percent of the provider figure | measured by `spend compare` | not measured |

Reading: the fleet lines hold under both answers. The per-seat 150M line and the 20M-per-unit line hold for m1e only with U7 and with no margin; the uncredited cut is the sibling goal's bounded reads (48.0M of critique delegates on 09-15 against about 10M under the 2M bound). That is why open question 3 now recommends building U7.

### P6. Today's seat rules (U6a)

No automatic successor exists today (F2). Each rule is followable with no build, and each check runs on a transcript or a ledger. The later form of each rule waits for the named unit.

| rule | today, no build | check | later |
|---|---|---|---|
| S1 over the trigger | when the `context-budget` health line shows the last call at or over 150K the seat keeps working: every remaining multi-call step runs as a fresh bounded delegate, the main session makes at most three main-thread calls per turn (launch, read the return, act), it records the handoff at a quiet point (`metasystem context handoff --root <installation>` when no wait is registering or pending), and it never stops or waits on Wido: a turn ends only by launching a delegate, registering a wait, or landing | after the first sample at or over 150K: no turn has more than three main-thread calls; a tool result contains `handoff recorded:`; no turn ends with a question to Wido; no allowed Stop while the seat's claim has a next step | the gate's block does the telling and the successor starts (U3a, U3c) |
| S2 turns bounded | at most 12 main-thread calls per turn; at most one background Bash per unit at a time; no Monitor; every wait through `metasystem job watch` or `metasystem wait` | calls between two Stop verdicts at most 12; Monitor uses zero | one launcher per unit (U2a, U4); the tool-use hook (U3e) |
| S3 delegates fresh and bounded | every design and revision is a fresh delegate (design-delegates-run-fresh-and-bounded); every code read is fresh with the bounded review brief (e700328e1); every delegate prompt starts with `Kind: design`, `build-read`, `critique` or `other` and names a call budget and a page ceiling; critique rounds are follow-ups on the one critic chain (skills/design-critique/SKILL.md:64-67) and corrections use `delegate --follow-up`; no other `SendMessage` to a delegate | every Agent prompt's first line is a `Kind:` line and contains the word budget; `resumedAgentId` results zero outside critique chains | reads longer than one step run inside the launcher (U4) |
| S4 messages | a `SendMessage` is at most 400 characters, one subject, sent at a unit boundary or for a question that stops the sender, never an acknowledgement | length and count per unit | the inbox (U7) |
| S5 reads by path | the main session never reads a tool result over 20,000 characters; a large output is read by a delegate from its persisted file | results over 20,000 characters zero | unchanged |
| S6 first act | `metasystem context resume` first when a handoff waits; else the `context-budget` line and `goal next` | the first tool calls | the SessionStart hook prints the pending handoff (U3c) |

### P7. The unit-to-goal map

Every unit is at most 300 changed lines; a unit marked split is sized by its goal into units under that bound. Every unit that adds a verb names cmd/metasystem/main.go (`families()`, :718-744) in its boundary (SSTB-208, stated once here). Order: U6a first, at no build cost; then the cap group U3a, U3e, U3b, U3c, which carries the largest saving (main sessions 779.5M to about 85M); then U1, U2a, U2c, U2b; then U4; then U5a, U5b, U5c; then U7; then the proof.

| id | goal | intent | size | order and dependencies | design questions the goal settles |
|---|---|---|---|---|---|
| U6a | 1:1 seats-spend-tokens-in-bounded-sessions | docs/orchestration.md carries S1 to S6 in their today forms, with a document-reading witness | about 60 | first; none | none; this page settles the rules |
| U3a | 1:10 coordinator-context-stays-under-budget, amendment 8d | the two config keys; the hook passes `transcript_path` and the runtime; the sample in the verdict; the `context-over-trigger` block ahead of idle and open-work blocks; the gate's own staging at the second block | about 400, split | after U6a; none | how the block shares the health cursor; the staging seam's shape beside `PrepareIdleContinuation`; the sentence text |
| U3e | 1:10, 8d (open question 4, answer b only) | a tool-use hook that reads the last call's usage and denies any tool call but the handoff, a revival or a delegate launch over the trigger | about 120 | after U3a | the hook's per-call cost; the allowlist of tool calls over the trigger; the deny text |
| U3b | 1:10, 8d | the handoff ends in-flight waits and records `openWork` with each row's complete selector, deadline left and child identity | about 300 | after U3a | the signal seam; the 32 KB state bound with full selectors and the manifest overflow; a `registering` row's grace |
| U3c | 1:10, 8d | the automatic successor: the two holds and the `others` exclusion in handoff.go, the verb's immediate revival and 30 second confirmation, `steward status` and the verb's confirmation lines, the brief and the role sentence, `context resume` (verify, then fresh registration of every `openWork` row), the handed-off allowance that survives consumption, the SessionStart notice | about 500, split | after U3b | what `LiveHandoffForSession` returns after consumption; the session marker; the successor's hooks in a headless session; the relay line for a human message |
| U1 | 1:24 registered-wait-matches-the-runtime-session | members A, B1, B2, B3, C of plans/registered-wait-matches-the-runtime-session-design.md, unchanged and in order | that page's | after U3c; none | moved SSTB-209: a changed-line allocation at or under 300 per member and a direct focused Go witness for the installed-binary path without the fixture bed's `METASYSTEM_WAIT_BINARY`; moved SSTB-117: focused witnesses instead of bed legs |
| U2a | 1:23 stop-gate-sees-harness-tracked-work | local waits: `wait --local --label L -- <command>` and `--pid P`, a kind `local` row with the child's pid and birth, child liveness in eligibility | about 400, split | after U1 | the child exit result; the attached pid's poll; the deadline that never signals the child |
| U2c | 1:23 | the idle bound of two, escalation at two through the `seatIdle` continuation beside the live seat, the handed-off marker on escalation | about 100 | after U3c | none beyond P3 |
| U2b | 1:23 | the human question wait proved on the landed goal wait | about 80 | after U2a | none |
| U4 | units-run-through-one-launcher, new goal, tier 2 | `metasystem unit run --plan <file>`: build, wait, prove the round, a fresh bounded reader, the judgement pause; one line of output; never lands | about 300 | after U2a | moved SSTB-210: a corrected unit re-enters the launcher through the same critic chain (`unit run --resume <run> --follow-up`) and the plan state that allows it; moved SSTB-208: the plan schema and the router line |
| U5a | 1:5 spend-fence-reports-tokens-per-model-and-cause | attribution in internal/spend: cause per call, kind with continuations as main, delegate kind from the `Kind:` line and the job role, the Claude-only engine filter, recursive subagent discovery | about 480, split | after U4 | moved SSTB-207: the `Kind:` line's grammar, the role map, the missing-line count; moved SSTB-116: the cursor cache schema and invalidation; SSTB-204: the runtime field name and the `startedAt` window |
| U5b | 1:5 | the health reason line, per-model ceilings, landed units by landing commit | about 280 | after U5a | moved SSTB-206 and 114: the commit resolution through git, the receipt field that marks a revival, the `seat` field and the claimant fallback |
| U5c | 1:5 | the verbs `spend window`, `spend compare`, `spend reads --last 5` and the router line | about 300 | after U5b | moved SSTB-205: the endpoints and roots the verb owns and the script's prototype status; moved SSTB-115 and 111: the provider comparison artifact and tolerance, the zone and the fleet sum |
| U7 | 1:1 (open question 3, answer b) | a seat inbox: `message send`, `message read`, the unread count in the Stop display, the handed-off predecessor's relay | about 200 | after U3c | moved SSTB-208: the inbox path, schema, receiver identity, cursor, lock and crash rule; the router line |
| proof | 1:1 | records/misc/tokens-day-<date>.md: one working day of three seats through `spend window`, `spend compare` and `spend reads`, against P5 | record | last | none |

## 3. Open questions for Wido (four)

1. Which automatic launcher. Recommended (a): the steward continuation with the holds changed (P2); the pane session becomes the seat's human console. Alternative (b): a steward-driven pane relaunch, which would replace U3c's allowance with a kill-and-restart of the pane's process and would need the steward to find the pane; it costs the predecessor's death before every successor and is not recommended. Pane relaunch by Wido is not an option.
2. The cap. Recommended 200K: the landed proof's ceiling, the smallest re-read per turn, and the same main total as 250K (85.3M against 85.1M by rule). Alternative 250K, which halves the handoffs (20 against 45 per half day) at equal cost, if 45 continuations a day is too much for the steward; one configuration line, no unit.
3. Peer messages and the inbox. Recommended (b), changed from revision 2: build U7 now, because the per-seat 150M line and the 20M-per-unit line hold for m1e only with it, because a headless successor's receipt of a `SendMessage` is unverified, and because the handed-off pane session needs a relay. Alternative (a): rule S4 and one measured day first; the two per-seat lines then read "with U7" and are not asserted for the first proof day.
4. The in-turn bound. Recommended (b): build U3e, a tool-use hook that bounds every call at 162K, so "under a stated context size" is literal for every call. Alternative (a): no hook; the promise is every Stop sample under 200K and every call under 200K while turns hold at 12 calls (S2), with the proof's maximum rule as the check; that changes what DONE promises from construction to a rule, and any 42-call turn breaks it.

## 4. Critique record

Round 2 and the reopened round-1 findings; every material finding is folded here or moved to the named goal's design questions in P7.

| finding | disposition |
|---|---|
| SSTB-201 | folded in P1: no tool-use hook is installed; the overshoot is 48K at 12 calls and 168K at 42; the DONE meaning is stated; open question 4 with U3e |
| SSTB-202 | folded in P2: `openWork` carries the complete selector with Chain and Poll; U3c registers fresh; the brief and the role change are U3c's |
| SSTB-203 | folded in P5: sessions to the trigger (10 growth calls), one handoff cost 0.58M, every threshold recomputed, per-seat forecasts for the 150M and ten-refusal lines |
| SSTB-204 | folded in P4: Claude only; the engine-window rule by `startedAt` and runtime; 374.4M is the any-runtime figure and 316.3M the same-scope m1e baseline; the fleet metric adopts the filter |
| SSTB-205 | folded in P4: the verb owns the endpoints; the baseline ends 12:51 CEST; the script is a prototype and is not rerun for DONE; moved to U5c for the verb's shape |
| SSTB-206 | folded in P4: a landed unit is a landing commit, counted once; revivals and several rows from one commit do not add; moved to U5b for the fields |
| SSTB-207 | folded in P4 as the launch declaration (`Kind:` line, job role); moved to U5a, spend-fence, for the grammar and the role map |
| SSTB-208 | folded in P7 as one line: every verb-adding unit names main.go; U7's storage contract moved to U7, goal 1:1 |
| SSTB-209 | moved to U1, registered-wait-matches-the-runtime-session: member allocations and the witness that skips the bed |
| SSTB-210 | moved to U4, units-run-through-one-launcher: the rerun and the critic re-entry through one chain |
| SSTB-211 | folded: the idle bound is cited at turnverdict.go:991-1010, the project rule at ../development/project-rules-local.md:10, background Bash at 211.6M |
| SSTB-102 | folded in P1 and P2: the trigger fires below the cap, the overshoot is a number, the block never crosses on its own |
| SSTB-103 | folded in P2: the capture ends waits first; the gate stages the handoff itself at the second block; no path ends without a successor |
| SSTB-105 | folded in P5 and section 3: the cap is configuration, 200K against 250K compared, the launcher and the inbox each have their units |
| SSTB-108 | folded in P2: the complete selector in `openWork`, fresh registration by `context resume`, never `--resume` |
| SSTB-110 | folded in P6: today's rules need no unit; the critique chain and `context resume` first are kept |
| SSTB-111 | folded in P4: one scope, the verb owns the zone and the window, the baseline is the same scope |
| SSTB-112 | folded in P5: recomputed without the double count and without a peer saving by rule |
| SSTB-114 | folded in P4: the landing commit; `release` and `land-ready` never count |
| SSTB-115 | folded in P4 and P5 for the program answer; moved to U5c for the comparison artifact |
| SSTB-116 | moved: the real owners are named in U3c's and U5a's intents; each goal draws its boundaries under 300 lines |
| SSTB-117 | moved to U1 and to every owning goal: focused Go witnesses, bed legs as DONE proof only |

## 5. Records

Evidence and method: records/misc/token-diagnosis-2026-09-15.md and its script (a prototype); the first bounded read's receipt in memory/receipts.log (e700328e1); m1c's three handoff records of 2026-09-15 (F2). Not read within the call budget: the census supplement that counts seat mains (internal/steward/census.go:45-74) beyond its counter; `ConsumedActive` and the consumed store's path; the `delegate --revive` command's own body; the harness's tool-use hook payload beyond its documented `transcript_path`.
