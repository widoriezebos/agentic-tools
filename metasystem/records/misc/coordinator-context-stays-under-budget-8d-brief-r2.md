# Design brief: coordinator-context-stays-under-budget, amendment 8d, revision 2

Template: scripts/agents/templates/design-brief.md. Draft by a reader for seat m1e, 2026-09-15. Paths are relative to the metasystem module of /Users/wido/LocalStorage/GitHub/agentic-tools-m1e unless absolute.

## Revision

Revision: revision 2 of amendment 8d of plans/coordinator-context-stays-under-budget-design.md (revision 3 plus 8c.15), against the program page plans/seats-spend-tokens-in-bounded-sessions-design.md revision 3 (495ddd0f2).

Reason: the Codex gpt-5.6-sol critique of revision 1 (code at 098a48645) returned rework with 16 material findings, CC8D-101 to 116 (101 to 104 critical); a reader confirmed every cited line on origin/main a24ecff03. Revision 2 answers each accepted finding below by id and keeps every rule of revision 1 that no finding touches, with its witness. Kind: design. This round ends at a written revision 2; the critique round 2 and the builds are later rounds; a NOT LAND or critique verdict never lands.

## Context pack

Read this pack first. Open another file only to check one of the cited lines below; do not widen the read beyond that check.

Diagnosis or prior revision:

The complete revision 1 (its line numbers name trunk 1a3d134b5):

```markdown
# coordinator-context-stays-under-budget, amendment 8d: the trigger, the in-turn hook, the open-work record and the automatic successor

- Written 2026-09-15 by a fresh Claude Fable design delegate for seat m1e from the brief coordinator-context-8d-brief.md (same directory). First draft; goes to a Codex critique the seat runs. Paths are relative to the metasystem module; line numbers name trunk 1a3d134b5.
- Revision base. This amendment extends plans/coordinator-context-stays-under-budget-design.md revision 3 with 8c.15 (the accepted revision for the state file, cancellation and verification) and builds on plans/seats-spend-tokens-in-bounded-sessions-design.md revision 3 (495ddd0f2), the program page, whose P1, P2, P3 and P7 supersede the parent page's section 5 wherever they differ: the successor launches while the predecessor lives (P2, R-114-m1e item 2), not after its observed death (parent section 5, Wido's decision 2 of 2026-09-13). No other page under plans/ or records/ names a later revision of this goal (grep of 2026-09-15).
- Rulings honoured: R-114-m1e items 2, 3, 5, 6; R-115-m1e items 2 and 6; seat rules S1 to S8 (docs/orchestration.md:355-371) stay intact, S7 gets a mechanical check (D4.4). Wido 2026-09-14: every rule names the witness that fails without it (section 4).
- Out of scope: 8c.7 (a held handoff's expiry) is decided by nobody here; section 3.6 names where it plugs in. U7 (inbox), U5 (measurement) and the proof week are other units. The successor may not claim or land until D9's unit is built (P2).

## 1. What the code does today (the facts the decisions change)

1. The cap is two Go constants: `ContextBoundTokens = 150000`, `ContextCeilingTokens = 200000` (internal/steward/context.go:23-24); no `internal/config` key exists (internal/config/spend.go:15-17 shows the key style `spend.ceiling.day.tokens`). The health line reads the last call through `usage.LatestCall` (internal/usage/calls.go:93) with the holder from the announcements; the hook passes no `transcript_path` (no `transcript` in scripts/agents/supervision-hook.sh).
2. The harness installs SessionStart, Stop and SessionEnd only (.claude/settings.json; the hook receives `claude start|stop|end`). No PreToolUse hook; growth inside a turn is unbounded (P1).
3. `metasystem context handoff` (cmd/metasystem/context_verbs.go:139) captures the state and stages a `seatHandoff` intent (handoff_capture.go:926-1029) and launches nothing; its success line is `handoff recorded: <nonce> state=<path> sha256=<digest>` (:208). `decideForHandoff` holds while the predecessor pid lives (handoff.go:93-101), while any seat main lives (:102-114) and while any other intent is open (`others`, an integer from revive.go:280-285; :117). This is F2: an interactive seat cannot die, so no successor ever launched.
4. The capture refuses any in-flight wait row of the main (`HANDOFF_WAIT_IN_FLIGHT`, handoff_capture.go:374-427) and any pending engine job of the held goal (`HANDOFF_LAUNCH_IN_FLIGHT`, :432-494); running engine jobs are recorded as `openJobs`. Harness `Agent` delegates are invisible to the engine; the state (handoff_state.go:125-137) carries no wait row, no delegate, no lessons note, no engine identity.
5. The Stop gate allows a session with a live handoff with the display `handoff recorded: <nonce>; end this session` (internal/goal/turnverdict.go:393-412, wired at cmd/metasystem/goal.go:710-711 through `LiveHandoffForSession`, live intents only: handoff_capture.go:1175). After consumption the allowance vanishes and the idle-backlog refusal (`refusal N of 3`, turnverdict.go:1003-1010) returns. Every allow writes `systemMessage` (internal/adapter/stopoutput.go:91-97); the hook treats an empty display as unreadable (supervision-hook.sh:2368).
6. The steward continuation is a delegate job: `steward revive` runs `<engine> delegate --revive <nonce>` (cmd/metasystem/steward_verbs.go:493-509) which is `dispatch --steward-intent` (delegate.go:236-241; dispatch.sh:1467-1486, `use_worktree=1`); the job's settings install only a SessionStart hook (internal/adapter/claude.go:122-131); the project Stop hook fires in the job's cwd but `lease hook-delegate` (internal/lease/hook_delegate.go:22-23, custody-based) makes the hook skip (supervision-hook.sh:1451-1475, :1581-1586). So a continuation today receives no Stop gate and can record no handoff of its own (SSTB-301).
7. `steward status` prints one JSON object (evidence, liveIntents, pendingNotifications, alertEpisodes; steward_verbs.go:803); no line names a running continuation. `StampLaunch` sets `LaunchStamped` with no time (intervene.go:219-235). The tick runs every 600 seconds (runner.go:104).
8. The context family has status, report, handoff, verify, prune (cmd/metasystem/main.go:431-438); no `resume`. S6 names a verb that does not exist.

## 2. The shape in one paragraph

Two configuration keys give the trigger (D1). The Stop gate samples the last call from the transcript the hook now passes, blocks once over the trigger with one sentence, and at the second block records and launches the handoff itself (D2). A PreToolUse hook denies every tool call but a handoff, a delegate launch, a wait or a landing over the trigger, in under 100 ms, allowing on any doubt (D3). The handoff verb ends the session's waits and records each row's full selector, records every in-flight delegate the seat declares (checked against the transcript on Claude), and refuses without a lessons note touched this session (D4). The verb then runs the revival itself: the predecessor's liveness is a fact for the record, the census and intent holds exclude identities, and the verb waits up to 30 seconds for the running signal (D5). The continuation is a seat main, not a delegate: its Stop hook runs the gate against the installation root, so it records the next handoff and the chain continues (D6). Its first act, `context resume`, verifies, registers every open-work row afresh, and writes the dead time next to the state file (D7). The predecessor's later Stops are allowed with one console line: the successor's status, the goal's next step and its health line (D8). Claims move to the successor by one authenticated leg, and a handed-off session's ledger mutations refuse (D9).

## 3. Decisions

### D1. The trigger is configuration (U3a-1)

- D1.1 Keys in `internal/config`: `context.ceiling.tokens` (default 250000, R-114-m1e item 3) and `context.handoff.margin.tokens` (default 50000); the trigger is ceiling minus margin (200000 by default). `ContextBoundTokens` and `ContextCeilingTokens` become functions of the loaded configuration; every reader (`contextVerdict`, the health line, `context status`, the report's maximum rule at contextreport.go:566-569, D2's block and D3's hook) reads them from one accessor `config.ContextBudget(root) (ceiling, margin, trigger int64)`. An invalid value (zero, negative, margin at or above ceiling) refuses at load with `CONTEXT_CONFIG_INVALID key=<k>`; a missing key takes the default.
- D1.2 The sample is the last main-thread call's prompt tokens from the Stop payload's `transcript_path` when the hook passes it (D2.1), else from the registered session as today. An unreadable sample never blocks: the verdict prints `CONTEXT: unknown (<reason>)` (P1).
- D1.3 Section 7's proof thresholds read the same accessor: the 95th-percentile line against the trigger, the maximum line against the ceiling; the goal's DONE text keeps its numbers until Wido answers open question 1. Nothing here changes what the proof measures or excludes (R-115-m1e item 6).

### D2. The over-trigger block and the gate's own handoff (U3a-2)

- D2.1 The hook passes `transcript_path` and the runtime to `goal turn-verdict` (`--transcript`, `--runtime`); the payload already carries `session_id` (supervision-hook.sh:1534). A missing field is not an error (D1.2).
- D2.2 A new branch `context-over-trigger` in `TurnVerdict` runs after the handoff allowance (turnverdict.go:393) and before the open-work, unwatched, uncertainty and idle-backlog branches: when the sample is at or over the trigger and `HandoffRecorded` reports no live-or-confirmed handoff for the session, the first such Stop blocks with exactly `CONTEXT AT <N>K OF <C>K: run metasystem context handoff --root <installation> alone in your next message` (`BlockSource` = `context-over-trigger`; it is not counted in `IdleBlocks` and not in the ten-refusal threshold, P3).
- D2.3 At the second over-trigger Stop of the session with no handoff (a per-session counter `ContextBlocks` on `sessionState`), the gate records and launches the handoff itself through a seam `options.StageHandoff(session) (HandoffLaunch, error)` wired in cmd/metasystem/goal.go beside `HandoffRecorded`; the seam calls the same `steward.HandoffAndLaunch` D5 gives the verb, with the caller classified from the hook's parent pid as the verb does (context_verbs.go:213-247), the lessons note from the session's last `--note` (D4.4; absent: the seat gets the D4.4 refusal text in the block and keeps working under S1), and `--no-delegates` never assumed: with undeclared delegates the seam refuses `HANDOFF_DELEGATES_UNDECLARED` and the block sentence carries it. The Stop keeps blocking until the running signal of D5.5 is seen; no Stop is allowed and no session is marked handed off before that signal.
- D2.4 While the successor is not confirmed, every later over-trigger Stop blocks with the same sentence, raises the attention item `handoff-successor-missing` once per nonce, and the tick retries the revival every 600 seconds; there is no bound on these blocks (a bound is a stop, R-114-m1e item 6). A capture fault (record unwritable or unreadable) does not block: `infrastructureVerdict("handoff-record")` as today (turnverdict.go:396) with the attention item `context-handoff-capture-fault`; the seat records again at its next quiet point.
- D2.5 The idle-backlog bound stays at three here; P3's change to two is U2c's (stop-gate-sees-harness-tracked-work) and is not built by this amendment.

### D3. The in-turn bound: a PreToolUse hook (U3e; R-114-m1e item 5; Claude-only accelerator under the goal's runtime rule)

- D3.1 .claude/settings.json installs `PreToolUse` (matcher `.*`) running `supervision-hook.sh claude tool`, timeout 5; the hook calls `<engine> adapter claude-tool-gate --root <installation>` with the payload on stdin (`session_id`, `transcript_path`, `tool_name`, `tool_input`). Job settings (adapter/claude.go:122-131) do not install it: delegates are bounded by their own caps.
- D3.2 The gate reads the session's usage cursor (internal/usage cursor cache) and at most the last 256 KB of the transcript after the cursor; the sample is the last assistant record's prompt tokens. Under the trigger, or with no sample, or on any read error, or when the reader's injected deadline of 100 ms has passed: allow, with the reason `tool-gate-allow: <cause>` counted in `artifacts/agents/context/tool-gate.jsonl` (one row per deny or non-trivial allow: session, at, tokens, tool, decision, cause, elapsed). The 100 ms is a deadline the reader receives, not a measurement a test waits for (Wido 2026-09-12: artificial clocks, never load-fragile tests).
- D3.3 At or over the trigger the gate allows exactly: a `Bash` command whose first word sequence, after an optional path prefix, is `metasystem context handoff`, `metasystem context resume`, `metasystem delegate`, `metasystem steward revive`, `metasystem wait`, `metasystem job watch`, `metasystem land` or `metasystem goal land`; the `Agent` tool; `Write` and `Edit` whose path is under the seat's memory directory (the S7 note). Every other call is denied with the PreToolUse output `{"hookSpecificOutput":{"hookEventName":"PreToolUse","permissionDecision":"deny","permissionDecisionReason":"CONTEXT AT <N>K OF <C>K: this call is denied; run metasystem context handoff --root <installation> alone, or launch a delegate"}}`. An allowlisted landing or wait is never denied whatever the sample (R-114-m1e item 5).
- D3.4 The allowlist is one table in internal/adapter/toolgate.go, read by the gate and by its test; it names the ceiling, not the trigger, as the point past which even the allowlist denies everything but `context handoff` and `context resume` (the margin is for the handoff, not for work).
- D3.5 What DONE's "under a stated context size" means after U3e: every call after the trigger is one of the allowlisted calls; at the observed growth of 4K a call, three such calls reach 212K under the default keys; the ceiling denies the rest. The proof's maximum rule reports any call over the ceiling (D1.3).

### D4. The open-work record (U3b; R-115-m1e item 2; S7)

- D4.1 Waits. The handoff ends every `registering` or `pending` row whose `MainId` is the caller's (handoff_capture.go:374-427 no longer refuses): it signals the row through the waiter's own interrupt path (the row turns `interrupted`, `interruptedBy: handoff <nonce>`), then records the row under `openWork` in the state with the complete `WaitSelector` (kind, targetId, goalId, event, after, verb, question, chain, poll; internal/run/waiter.go:57-68), the deadline left in seconds, and for a `local` row the child's pid and birth. A `registering` row younger than 5 seconds is waited for up to 5 seconds before it is ended (the grace P7 asks about); the clock is injected. A row that cannot be ended refuses `HANDOFF_WAIT_UNENDABLE row=<id>` and nothing is recorded. The successor never runs `wait --resume`; it registers each row afresh (D7.2).
- D4.2 Engine jobs stay as today: running jobs of the held goal under `openJobs`, a pending one refuses `HANDOFF_LAUNCH_IN_FLIGHT`.
- D4.3 Harness delegates. The verb takes either `--no-delegates` or one or more `--delegate id=<agent id>,asked=<brief path or one line>,output=<path>`; neither given refuses `HANDOFF_DELEGATES_UNDECLARED`. Each declaration is recorded under `delegates` in the state (id, asked, output, declaredAt). On a runtime with a registered transcript (Claude), the verb lists the transcript's `Agent` tool_use ids whose result record is absent and refuses `HANDOFF_DELEGATE_UNDECLARED id=<id>` for any id not declared (the accelerator); a runtime with no transcript takes the declaration as the record. So the quiet-point rule of S1 reads: no delegate running, or every running delegate declared.
- D4.4 S7 mechanically. `--note <path>` is required (else `HANDOFF_NOTE_MISSING`); the verb stats the note before it writes anything and refuses `HANDOFF_NOTE_STALE note=<path> modified=<t> session=<t0>` when the note's modification time is not after the caller's announced process start (`Announcement.PidStartedAt`, the session's start on every runtime, context_verbs.go:243-245); the state records `lessonsNote {path, modifiedAt, sha256}`, and `VerifyHandoffState` checks `lessonsNote.modifiedAt < writtenAt` (the check S7 states). A headless continuation names its own note the same way.
- D4.5 Engine identity. The state records `engine {path, sha256, installGen}` from `os.Executable()`; D5.7 and D7.4 read it. The 32 KB bound holds as today: `openWork`, `delegates` and `lessonsNote` are state lists that `splitHandoffStateLists` (handoff_capture.go:736) moves to the manifest on overflow, and the manifest overflow is already tested (`TestHandoffManifestPreservesOverflow`).

### D5. The successor starts from the verb, within seconds, by identity (U3c-1; SSTB-310; R-115-m1e item 2)

- D5.1 `decideForHandoff` for a `seatHandoff` intent: `handoffPredecessorLiveness` is read into the decision's reason only (no hold at handoff.go:93-101). The census hold (:102-114) becomes identity-based: `Workers` gains `SeatMainRefs []identity.Ref` and `HandedOffMains []identity.Ref` (from `workersFromVerdict`, which today drops identities after counting), and the hold counts a live seat main only when it is neither `binding.Predecessor` nor a main whose session has a confirmed handoff (D8.1's marker); `CensusComplete`, `Untracked` and `Unprovable` hold as today. The `others` hold (:117): revive.go:280-285 passes the intents themselves, and a consumed intent whose `JobId` equals `binding.PredecessorJob` is excluded, so a continuation is succeeded by the next continuation. The dry-revival cap and the provider-outage hold are unchanged.
- D5.2 `steward.HandoffAndLaunch(stateRoot, caller, args, now, launch LaunchSeam, wait ConfirmSeam)`: `Handoff` as today (record, supersede, stage, prepare), then `CompleteRevival` with the verb's launch seam of steward_verbs.go:493-509 (`<engine> delegate --revive <nonce>`, `Setsid`), then the confirmation wait. The verb `context handoff` calls it; the tick's retry every 600 seconds is the fallback, not the path.
- D5.3 The bound: the successor is running within 30 seconds of the record (`HandoffConfirmWaitSeconds = 30`, a constant with an injected clock for tests; the wait polls the signal every 500 ms). The record time is `state.writtenAt`; the launch time is `LaunchStampedAt` (D5.5).
- D5.4 The predecessor's exact identity and the successor's job id travel as today (`HandoffBinding`, the intent's `JobId`); nothing new is invented for exclusion beyond D5.1.
- D5.5 The running signal, observable and named: the consumed intent has `launchStamped: true` and the new `launchStampedAt` (StampLaunch, intervene.go:219-235), and the continuation's job record `<jobId>` is at status `running` with a pid the identity prober confirms. `steward status` gains the line `continuation <jobId> running for handoff <nonce> since <t>` (or `held`, `missing`); `context handoff` prints `handoff recorded: <nonce> state=<path> sha256=<digest>` then `successor running: job=<jobId> pid=<pid> after <n>s` and exits 0. Without the signal inside 30 seconds it prints `successor not confirmed: <reason>` and exits 1; the intent stays live or consumed-unstamped as today, and D2.4 takes over. The seat is not handed off before the signal.
- D5.6 The staged brief (stage.go:153-164) tells the successor: read the state in bounded views, run `metasystem context resume --root <installation> --nonce <nonce>` first, use the engine at `<engine.path>`; the role's rule 2 (scripts/agents/roles/steward-continuation.md:20-22) gains one sentence: under a handoff nonce the predecessor is expected to be alive and idle, and the successor never yields to it.
- D5.7 Stale engine. Before staging, the verb runs `<engine> context resume --help` with a 2-second injected deadline; a non-zero exit refuses `HANDOFF_ENGINE_LACKS_RESUME engine=<path>` (the observed case: a binary built before the verb existed). The seat rebuilds the engine and records again.

### D6. The continuation is a seat main with a Stop gate (U3c-2; SSTB-301)

- D6.1 `lease.HookDelegate` returns `continuation: true, installation: <root>` when the custody's job role is `steward-continuation`; the hook then does not skip (supervision-hook.sh:1451-1475, :1581-1586): at `claude start` it announces the process as a main (`lease announce`, the seat path), at `claude stop` it runs the seat gate with `--root <installation>` (never the worktree), at `claude end` the seat end. The gate, the health line and `context handoff` therefore see the continuation as the seat's main, and its usage is registered for the sample.
- D6.2 A headless session's allowed Stop ends its process; the reaper closes the chain (reap.go:33-57) and `others` falls. So "it may end its own session once its successor is confirmed" is the D8 allowance applied to a headless predecessor; nothing else ends it. The SessionStart notice for the successor's pane-less session is a receipt line `continuation <jobId> started for handoff <nonce>`.
- D6.3 The continuation runs in the worktree dispatch gives it (dispatch.sh:1482) but every ledger verb and the gate use `--root <installation>` from the custody; landings follow the role contract's covenant as today. Whether a `SendMessage` reaches a headless session stays unverified; U7's inbox is the relay (R-114-m1e item 4).

### D7. `context resume`: verify, register afresh, record dead time (U3c-3; S6; SSTB-308)

- D7.1 `metasystem context resume --root <installation> --nonce <nonce>` (main.go `families()`, a router line): refuses unless the caller is the main of the job the consumed intent for `<nonce>` names (`HANDOFF_RESUME_NOT_SUCCESSOR`); runs `VerifyHandoffState`; prints the held goal, its next step's first line, the `openWork`, `delegates` and `openJobs` counts, and the `context-budget` line.
- D7.2 It registers every `openWork` row as a fresh wait through the wait verb with every selector field (kind, targetId, goalId, event, after, verb, question, chain, poll), the deadline left, and for a local row the pid and birth under U1's authenticated-session registration (registered-wait-matches-the-runtime-session, members A to C; SSTB-308: U1 lands first). A row that cannot be registered is printed as `open-work row <id> not registered: <reason>` and the verb still exits 0; the row stays in the state for the seat to act on.
- D7.3 Dead time. The verb writes once, exclusively, `artifacts/agents/context/handoffs/<nonce>/resume.json` (schema 1): `recordedAt` (= state.writtenAt), `launchStampedAt`, `resumedAt` (now), `successorJob`, `successorSession`, `successorMainId`, `launchSeconds`, `deadTimeSeconds` (resumedAt minus recordedAt), `openWorkRegistered`, `openWorkFailed`. `state.json` is untouched, so the nonce directory stays immutable for `VerifyHandoffState`. `context status` prints `last handoff <nonce>: dead time <n>s`; `context report` lists every handoff of the week with its dead time; U5a reads the same file (R-115-m1e item 3). The expectation: dead time under 120 seconds; a longer one is a proof-week finding, never a stop.
- D7.4 The verb compares its own executable's sha256 with `state.engine.sha256` and prints `engine differs: recorded=<sha> running=<sha>` as a line, not a refusal (a newer engine is the normal case after a landing).
- D7.5 It is S6's first act; the second act is `goal next` on the held goal as the role says.

### D8. The handed-off predecessor: one console line per Stop, no turn (U3c-4; SSTB-305; R-115-m1e item 2)

- D8.1 `LiveHandoffForSession` returns live and consumed intents whose binding names the session (the allowance survives consumption and the reap). The gate marks the session `handedOff: <nonce>` on the first Stop that sees the D5.5 signal, never before; the marker lives on `sessionState`.
- D8.2 Every later Stop of a handed-off session is allowed whatever the backlog: the open-work, unwatched, uncertainty, goal-revision and idle-backlog branches are skipped, and the ten-refusal threshold does not count it. The output is not empty (SSTB-305): `systemMessage` carries one console line, `handed off <nonce>: successor <jobId> <running|held|reaped|missing> since <t>, dead time <n>s; goal <id>: <next step, first 120 characters>; <the successor's context-budget line>` — the worker's status Wido asked for. An allowed Stop with a `systemMessage` costs no model turn; the hook's empty-display rule (supervision-hook.sh:2368) is untouched because the display is never empty.
- D8.3 The line's sources: the consumed intent and the job record (status, pid), `resume.json` (D7.3), the goal ledger (next step), and `ContextBudgetLine` with the explicit holder = the successor's session and runtime (the job's SessionStart signal records its session id, adapter/claude.go:268). A source that cannot be read prints `unknown` in its slot; it never blocks.
- D8.4 If the successor is reaped without having recorded its own handoff and without progress, the next Stop clears the marker, prints `successor <jobId> ended without a handoff; this session resumes under S1`, and the gate is the ordinary gate again (never stop for a limit). The background waiters were ended by D4.1; their exit wakes the session once (one call, then an allowed Stop).
- D8.5 A human message to the pane costs one turn; the predecessor answers in one line naming the successor's job id and relays through U7 when built; until then it answers that the relay is not built. Nothing in this amendment depends on U7.

### D9. Claims and lineage move by one authenticated leg (U3c-5; SSTB-303)

- D9.1 A claim keys on machine plus lineage (internal/goal/verbs.go:502-506, :968-969). New leg: `metasystem goal claim --id <goal> --continue-handoff <nonce>`. It admits only when the caller is the main of the job named by the consumed intent for `<nonce>`, the state's `heldGoal.claimant` equals the ledger's current claimant, and the intent's goal is `<goal>`; it rewrites the claimant's lineage and main id to the caller's, keeps the claim's epoch and revision, and appends a history row `claim continued from <predecessor lineage> under handoff <nonce>`. Any other caller refuses `CLAIM_CONTINUE_NOT_SUCCESSOR`.
- D9.2 Until D9.1 is built the successor may not claim or land (P2); D7.1 prints `claim continuation not built: read-only until U3c-5` in that case so the successor does not try.
- D9.3 Two live sessions at the ledger: every claim, land, goal edit and handoff verb whose caller's session is marked handed off with a running successor refuses `SESSION_HANDED_OFF nonce=<n> successor=<jobId>`; the same `LiveHandoffForSession` closure decides. After D8.4 clears the marker the refusal ends.

### D10. Where 8c.7 plugs in (not decided here)

An expiry, if Wido wants one, takes effect in `handoffExpiryRule` (handoff.go:21, :50-52, consulted at :89-91) exactly as 8c.15.7 says; under this amendment a held handoff is rarer (the holds of D5.1 fall only on census uncertainty, an open other continuation, the dry-revival cap or a provider outage), and D2.4's `handoff-successor-missing` item is the standing notice while it is held. The notice owner for a cancelled-by-expiry record stays the open point 8c.15.7 names.

## 4. Failure modes

| mode | what happens | where it is checked |
|---|---|---|
| Two live sessions (predecessor pane and successor) | By design during the console phase. The predecessor is marked at the signal (D8.1); its Stops cost no turn (D8.2); its ledger mutations refuse (D9.3); the census hold excludes it by identity (D5.1). | `TestHandedOffSessionMutationsRefuse`, `TestHandoffCensusExcludesPredecessorAndHandedOffMains` |
| Handoff recorded while a wait is in flight | Ended and recorded with the full selector (D4.1); registered afresh by the successor (D7.2); a row that cannot be ended refuses. | `TestHandoffEndsAndRecordsInFlightWaits`, `TestResumeRegistersOpenWorkAfresh` |
| Handoff recorded while a harness delegate is in flight | Declared and recorded, or refused `HANDOFF_DELEGATE_UNDECLARED` on Claude, `HANDOFF_DELEGATES_UNDECLARED` when nothing was said (D4.3). | `TestHandoffRefusals/delegates-undeclared`, `TestHandoffDelegateDeclarationsMatchTranscript` |
| Handoff recorded while an engine job is pending | `HANDOFF_LAUNCH_IN_FLIGHT` as today (D4.2). | existing `TestHandoffRefusals` |
| Successor launch fails or is not confirmed in 30 seconds | Verb exits 1 with `successor not confirmed`; the seat is not handed off; the gate keeps blocking over the trigger, raises `handoff-successor-missing`, the tick retries every 600 seconds (D2.4, D5.5). | `TestHandoffAndLaunchReportsUnconfirmedSuccessor`, `TestOverTriggerBlockRaisesSuccessorMissing` |
| Successor dies before its own handoff | The reaper closes the job; the predecessor's next Stop clears its marker and resumes under S1 (D8.4). | `TestHandedOffSessionResumesWhenSuccessorEnds` |
| Stale engine binary | Refused at the record (`HANDOFF_ENGINE_LACKS_RESUME`, D5.7); a differing but capable engine is printed by resume (D7.4). | `TestHandoffRefusals/engine-lacks-resume`, `TestResumeReportsEngineDifference` |
| Second over-trigger Stop with a capture fault | Not a refusal loop: `infrastructureVerdict("handoff-record")` plus `context-handoff-capture-fault` (D2.4). | `TestOverTriggerCaptureFaultDoesNotBlock` |
| Tool gate slow or unreadable | Allow with a counted cause (D3.2). | `TestToolGateAllowsPastItsDeadline` |
| Configuration invalid | Refused at load with the key named (D1.1). | `TestContextBudgetConfigRefusesInvalidValues` |

## 5. Rule-to-witness table (every rule names the test that fails without it)

| rule | unit | witness |
|---|---|---|
| D1.1 keys, defaults, one accessor, invalid values refuse | U3a-1 | `TestContextBudgetConfigDefaultsAndAccessor`, `TestContextBudgetConfigRefusesInvalidValues`; `TestContextVerdictReadsConfiguredBounds` (mutation: a reader with a literal 200000 fails) |
| D1.2 transcript sample, unknown never blocks | U3a-1 | `TestContextSampleFromTranscriptPath`, `TestContextUnknownSampleNeverBlocks` |
| D1.3 proof thresholds from the accessor | U3a-1 | `TestContextReportThresholdsFollowConfiguration` |
| D2.1 hook passes transcript and runtime | U3a-2 | a leg in the supervision-hook fixture bed (section 7 of the parent page: the Stop hook prints the context line; the builder names the script from testing.json) |
| D2.2 first block sentence, source, not counted as idle | U3a-2 | `TestOverTriggerBlocksOnceWithTheSentence`, `TestOverTriggerIsNotAnIdleRefusal` |
| D2.3 second block stages and launches through the seam, no allow before the signal | U3a-2 | `TestSecondOverTriggerStopStagesTheHandoff`, `TestNoAllowBeforeTheRunningSignal` |
| D2.4 unbounded blocks, attention item once, capture fault does not block | U3a-2 | `TestOverTriggerBlockRaisesSuccessorMissing`, `TestOverTriggerCaptureFaultDoesNotBlock` |
| D3.1 PreToolUse installed, not in job settings | U3e | `TestClaudeSettingsInstallNoToolGateForJobs`; a settings.json reading test `TestProjectSettingsInstallToolGate` |
| D3.2 cursor plus 256 KB tail, deadline allow, counted | U3e | `TestToolGateReadsBoundedTail`, `TestToolGateAllowsPastItsDeadline`, `TestToolGateCountsDecisions` |
| D3.3 allowlist exact, deny text exact, landing and wait never denied | U3e | `TestToolGateAllowlist` (table-driven over the D3.3 table; mutation of any row fails) |
| D3.4 ceiling denies all but handoff and resume | U3e | `TestToolGateAboveCeiling` |
| D4.1 waits ended, full selector, deadline, child, grace, unendable refuses | U3b-1 | `TestHandoffEndsAndRecordsInFlightWaits` (asserts all nine selector fields), `TestHandoffRegisteringGraceUsesInjectedClock`, `TestHandoffRefusals/wait-unendable` |
| D4.3 delegates declared or refused; transcript cross-check | U3b-2 | `TestHandoffRefusals/delegates-undeclared`, `TestHandoffDelegateDeclarationsMatchTranscript`, `TestHandoffRecordsDeclaredDelegates` |
| D4.4 note required, stale refuses, verify checks order | U3b-2 | `TestHandoffRefusals/note-missing`, `/note-stale`, `TestHandoffStateVerifierChecksLessonsNoteOrder` |
| D4.5 engine recorded; overflow to manifest | U3b-2 | `TestHandoffRecordsEngineIdentity`, existing `TestHandoffManifestPreservesOverflow` extended with the three lists |
| D5.1 liveness no hold; census and others by identity | U3c-1 | `TestHandoffLaunchesWhileThePredecessorLives` (replaces `TestDecideForRevivalHoldsUntilThePredecessorIsDead`), `TestHandoffCensusExcludesPredecessorAndHandedOffMains`, `TestHandoffOthersExcludesThePredecessorJob`, `TestHandoffStillHoldsOnCensusUncertainty` |
| D5.2, D5.3 verb launches and waits 30 s with an injected clock | U3c-1 | `TestHandoffAndLaunchConfirmsWithinTheBound`, `TestHandoffAndLaunchReportsUnconfirmedSuccessor` |
| D5.5 signal fields, status line, verb lines, exit codes | U3c-1 | `TestStampLaunchRecordsTime`, `TestStewardStatusNamesTheContinuation`, `TestContextHandoffPrintsSuccessorLines` |
| D5.6 brief and role sentence | U3c-2 | `TestStagedBriefNamesResumeAndEngine`, `TestContinuationRoleNeverYieldsToItsPredecessor` (a document-reading witness like `TestSeatRulesPresentInOrchestrationDoc`) |
| D5.7 engine preflight | U3c-1 | `TestHandoffRefusals/engine-lacks-resume` |
| D6.1 hook-delegate marks a continuation; hook runs the seat path against the installation | U3c-2 | `TestHookDelegateMarksStewardContinuation`; a hook fixture leg: a continuation job's Stop is gated, not skipped |
| D6.2 start receipt line | U3c-2 | `TestContinuationStartReceipt` |
| D7.1 resume admission and output | U3c-3 | `TestResumeRefusesANonSuccessor`, `TestResumePrintsTheHeldGoalAndCounts` |
| D7.2 fresh registration with every field | U3c-3 | `TestResumeRegistersOpenWorkAfresh` (asserts the nine fields and never calls `wait --resume`) |
| D7.3 resume.json once, fields, state untouched, status and report lines | U3c-3 | `TestResumeWritesDeadTimeOnce`, `TestVerifyHandoffStateIgnoresResumeRecord`, `TestContextStatusPrintsDeadTime`, `TestContextReportListsHandoffDeadTime` |
| D7.4 engine difference printed | U3c-3 | `TestResumeReportsEngineDifference` |
| D8.1 allowance survives consumption; marker at the signal only | U3c-4 | `TestLiveHandoffForSessionSeesConsumedIntents`, `TestHandedOffMarkerWaitsForTheSignal` |
| D8.2 skipped branches, not counted, console line, never empty | U3c-4 | `TestHandedOffSessionStopIsAllowedWithTheConsoleLine` (replaces the display of `TestTurnVerdictAllowsTheStopUnderARecordedHandoff`), `TestHandedOffStopDoesNotCountTowardRefusals` |
| D8.3 sources and `unknown` slots | U3c-4 | `TestConsoleLineToleratesMissingSources` |
| D8.4 marker cleared when the successor ends | U3c-4 | `TestHandedOffSessionResumesWhenSuccessorEnds` |
| D9.1 continuation leg admits the successor only | U3c-5 | `TestClaimContinueHandoffAdmitsTheSuccessor`, `TestClaimContinueHandoffRefusesOthers` |
| D9.2 read-only line until built | U3c-3 | `TestResumePrintsReadOnlyUntilClaimContinuation` (removed by U3c-5) |
| D9.3 handed-off mutations refuse | U3c-5 | `TestHandedOffSessionMutationsRefuse` |
| R-115-m1e item 6 (no floor lowered) | every unit | each read checks: no threshold, witness or gate removed; D1.3 keeps the proof's measures |

Existing tests that change: `TestDecideForRevivalHoldsUntilThePredecessorIsDead` (revive_test.go:426) and `TestCompleteRevivalHoldsThenLaunchesAHandoff` (:674) assert the old hold and are replaced by the D5.1 witnesses; `TestHandoffRefusals` (handoff_capture_test.go:335) loses the `HANDOFF_WAIT_IN_FLIGHT` case and gains the D4 cases; `TestHandoffWaiterStates` (:311) asserts the ended rows; `TestTurnVerdictAllowsTheStopUnderARecordedHandoff` (turnverdict_test.go:683) asserts the console line; the parent page's section 5 test list ("`decideForRevival` refuses a `seatHandoff` intent while the predecessor pid lives") is superseded by P2 and this table.

## 6. Build units (one Codex build and one read each; every unit names cmd/metasystem/main.go when it adds a verb or flag)

| unit | files | size | DONE | witness | order |
|---|---|---|---|---|---|
| U3a-1 keys and sample | internal/config/context.go (new), internal/steward/context.go, internal/steward/contextreport.go, cmd/metasystem/context_verbs.go, cmd/metasystem/goal.go (`--transcript`, `--runtime`), cmd/metasystem/main.go | about 250 | the two keys exist with R-114-m1e's defaults; every reader uses the accessor; the sample comes from the passed transcript; unknown never blocks | D1 rows of section 5 | first |
| U3a-2 the over-trigger block | internal/goal/turnverdict.go, cmd/metasystem/goal.go (`StageHandoff` seam), internal/steward (the seam's exported entry), scripts/agents/supervision-hook.sh, the hook fixture bed | about 280 | D2.2 to D2.4 hold; the second block stages and launches; no allow before the signal | D2 rows | after U3a-1 |
| U3e the tool gate | .claude/settings.json, scripts/agents/supervision-hook.sh (`claude tool`), cmd/metasystem/adapter_runtime_verbs.go, cmd/metasystem/main.go, internal/adapter/toolgate.go (new), internal/adapter/claude.go (job settings unchanged, asserted) | about 200 | D3.1 to D3.5 hold; deadline allow; the allowlist table is the only source | D3 rows | after U3a-1 |
| U3b-1 waits into open work | internal/steward/handoff_capture.go, internal/steward/handoff_state.go (`openWork`), internal/run (the interrupt entry the verb calls), internal/steward/handoff_state verifier | about 250 | D4.1 holds; `HANDOFF_WAIT_IN_FLIGHT` is gone; the state carries every selector field | D4.1 rows | after U3a-1 |
| U3b-2 delegates, note, engine | cmd/metasystem/context_verbs.go (`--delegate`, `--no-delegates`, `--note`), internal/steward/handoff_capture.go, handoff_state.go (`delegates`, `lessonsNote`, `engine`), internal/usage (Agent id listing), cmd/metasystem/main.go usage line | about 250 | D4.3 to D4.5 hold; S7 is checked mechanically | D4.3 to D4.5 rows | after U3b-1 |
| U3c-1 the launch from the verb | internal/steward/handoff.go, verdict.go (`SeatMainRefs`), census.go, revive.go (intents passed, not counted), intervene.go (`LaunchStampedAt`), a new internal/steward/handoff_launch.go (`HandoffAndLaunch`, the confirm seam), cmd/metasystem/context_verbs.go, cmd/metasystem/steward_verbs.go (status line) | about 300 | D5.1 to D5.5 and D5.7 hold; a live predecessor no longer holds; the verb confirms within 30 s or exits 1 | D5 rows | after U3b-2 |
| U3c-2 the continuation is a main | internal/lease/hook_delegate.go, cmd/metasystem/lease.go, scripts/agents/supervision-hook.sh, internal/steward/stage.go (brief), scripts/agents/roles/steward-continuation.md, the hook fixture bed | about 220 | D5.6, D6.1 to D6.3 hold; a continuation's Stop is gated against the installation | D5.6, D6 rows | after U3c-1 |
| U3c-3 `context resume` | cmd/metasystem/context_verbs.go, cmd/metasystem/main.go (router line), internal/steward/handoff_resume.go (new), internal/run (fresh registration under U1), contextreport.go, context status | about 280 | D7.1 to D7.5 hold; resume.json carries the dead time; open work is registered afresh | D7 rows | after U3c-2 and U1 (SSTB-308) |
| U3c-4 the console line | internal/steward/handoff_capture.go (`LiveHandoffForSession` reads consumed), internal/goal/turnverdict.go (marker, skipped branches, console line), internal/report/stoppresentation.go if the line needs a slot, cmd/metasystem/goal.go | about 280 | D8.1 to D8.5 hold; the pane shows the worker's status at every Stop, at no turn | D8 rows | after U3c-3 |
| U3c-5 claims move | internal/goal/verbs.go (`--continue-handoff`), internal/goal (the `SESSION_HANDED_OFF` refusal), cmd/metasystem/goal.go, cmd/metasystem/main.go usage line | about 250 | D9.1 and D9.3 hold; the successor claims and lands under its own lineage; a handed-off session cannot mutate | D9 rows | after U3c-4 |

Order across the program: U3a-1, U3a-2, U3e, U3b-1, U3b-2 in this page's order; U1 (m1b) before U3c-3; U2c (idle bound of two) after U3c-4; U7 after U3c-5. Each unit's read checks R-115-m1e item 6 and that no rule in section 5 lost its witness. Every unit's builder brief carries "a NOT LAND verdict never lands; after the fold limit, stop and report" (m1e lesson of 2026-09-15).

## 7. Open questions for Wido (each with a recommendation)

1. The DONE's proof numbers against the 250K ceiling. The goal's DONE (approved at revision 71) promises a week where the 95th percentile of per-call prompt tokens is under 150K and no call exceeds 200K. R-114-m1e item 3 sets the ceiling at 250K, so the trigger is 200K: a session grows from about 110K at boot to 200K before it hands off, and the 95th percentile of its calls lands near 200K, not under 150K, by construction; even the program page's own defaults (200K ceiling, 150K trigger) put the 95th percentile near 150K. Recommended (a): Wido restates the two numbers as the configured trigger and ceiling (200K and 250K under R-114-m1e; D1.3 reads them from configuration), keeping the third line (no compaction or reset) and every measurement as written; this is his change to make, since R-115-m1e item 6 forbids a unit from lowering a proof floor. Alternative (b): keep the DONE's numbers and set the defaults to ceiling 200K, margin 50K, with R-114-m1e's 250K as a later one-line change; the 95th-percentile line then still needs his reading. The build takes R-114-m1e's defaults until he answers.
2. The tool gate's allowlist over the trigger (D3.3). Recommended: the list as written (handoff, resume, delegate, revive, wait, job watch, land, the Agent tool, the memory note). Alternative: add `goal edit` for a next-step note; the design leaves it out because a next step over the trigger belongs in the handoff's state.
3. The dead-time expectation of 120 seconds (D7.3) is recorded, never enforced. Recommended: keep it a proof-week line (with R-115-m1e item 3's warm-up share), raising the ceiling by one configuration line if warm-up plus dead time exceed the stated share of the day's saving, as that ruling says. Alternative: an attention item when one handoff's dead time passes 300 seconds; one branch in `context resume`.
4. Should the predecessor's console Stop line (D8.2) also print the successor's last receipt? Recommended: no; the health line and the next step are what R-115-m1e item 2 asks, and a receipt line is one more reader per Stop. Alternative: one receipt line, added in U3c-4 at no new source.

## 8. What was not checked (budget)

The exact script name of the supervision-hook fixture bed and its testing.json section (the parent page's section 7 promises such a bed); the Claude transcript's record shape for a background `Agent` launch and its completion (D4.3 relies on the harness recording a tool_use with an agent id and a later result; S3's check already reads `resumedAgentId` from transcripts, so the shape exists); whether `claude -p` honours a PreToolUse deny in every version the seats run (D3 allows on any doubt, so a missing hook degrades to P1's Stop-time bound).

## 9. Critique record

None yet; the seat runs the Codex critique on this draft.
```

Facts on trunk since revision 1 (origin/main a24ecff03):

- U1: member A landed (c4e7d0f31): `RuntimeSession` and `PreviousRuntimeSession` on the lease announcement (excerpt 5), `lease associate-session`, the association in `metasystem up` (up.go:682-688; advisor branch now :691-705, unchanged). B1 (hook passes `--runtime-session`) is in build; B2, B3, C open. No other cited file changed since 098a48645.
- An active continuation job already records a handoff through the verb as the delegate class (excerpts 2, 3; `activeDelegateCaller`, handoff_capture.go:282-309). Revision 1's 1.6 holds for the Stop hook only.

Critique findings being answered (all cited lines confirmed on a24ecff03; reader notes in brackets):

1. CC8D-101 (critical). The successor cannot become the live main while the predecessor lives: another lineage's new main is an advisor (up.go:691-705), a live different holder keeps the lease except the mission-runner edge (excerpt 1), a main-class handoff needs `MainId == HolderMainId` (excerpt 2). Asked: a fenced succession with exactly one session holding mutation authority through launch and acknowledgement. [The delegate-class path is an alternative; a second live-holder exception is an authority change no ruling covers: pose it to Wido.]
2. CC8D-102 (critical). U3a-2 calls `HandoffAndLaunch`, added by U3c-1; U3c-1 launches before U3c-2 gates the continuation, U3c-3 adds `context resume` (D5.7 runs it) and U3c-5 moves the claim; P7 puts U1 before U3c (program page :130). Asked: an acyclic order with U1, resume, the gate, claim succession and the fence in place before any launch; name the owner of every called function.
3. CC8D-103 (critical). D1.3 and question 1 build the proof at 200K/250K until Wido answers, weaker than the approved DONE ("under 150 thousand, no call exceeds 200 thousand", goal revision 89). Asked: keep 150K p95 and 200K maximum as DONE and witness under either answer. [R-114-m1e item 3 fixes only the ceiling; the margin is the design's.]
4. CC8D-104 (critical). The `Agent` result arrives at once, so "result absent" never means running. Shape (transcript of 2026-09-05, placeholders): assistant `tool_use` `Agent` id `toolu_<x>`; next, user `tool_result` for `toolu_<x>` with text `agentId: <a>` and `output_file: <path>`; later, a user record `<task-notification>` with `<task-id><a>`, `<tool-use-id>toolu_<x>`, `<output-file><path>`, `<status>completed`. Asked: a state machine over these; `--no-delegates` fails on any launch without a terminal notification; fixtures: running, completed, failed, missing notification, mismatched declaration.
5. CC8D-105. The tick does not retry: the intent is consumed before launch, a failed launch escalates (excerpt 6); resume reads live intents only (revive.go:343-351, runner.go:171-185); a consumed unstamped intent is left while its job record is open, else reaped as outcome unknown, never relaunched (reap.go:183-205). Asked: a durable launch state and retry, launch failure distinct from a failed stamp; a failure-injection witness, no duplicate successor.
6. CC8D-106. When the successor dies, clearing the marker returns neither lease nor claim (moved by D9.1), and an idle predecessor may never Stop. The reaper only notifies and closes the chain (reap.go:42-58); ownership is machine plus lineage (excerpt 4). Asked: an event-driven recovery owner, a fenced exactly-once return; witnesses with the predecessor active and idle.
7. CC8D-107. D3.3 allowlists `metasystem land` and `goal land`, which do not exist (main.go:132 family `landing`, :534 `goal land-ready`; landings run `bash scripts/agents/land.sh`); D3.4 denies landings and waits above the ceiling, against R-114-m1e item 5. `wait` (main.go:722) and `job watch` (:221) exist. Asked: the real grammar with path prefixes and the wrapper; landing, registered waits, job watch allowed at both thresholds; a deny witness per spelling.
8. CC8D-108. U3e edits `.claude/settings.json`, but adoption and `hooks check` read `scripts/enforcement/claude-code-hooks.json` (excerpt 9; SessionStart, Stop, SessionEnd only; merge hostsetup/setup.go:153-185, check hooks/hooks.go:40-75). Asked: U3e owns the shipped file; witnesses: PreToolUse installed, preserved, detected when removed.
9. CC8D-109. A capture fault allows the Stop (D2.4) and an allowed headless Stop ends the process (D6.2), against P2: "No path ends a session with no successor started." Asked: fail open interactively, fail closed for a headless seat main until a successor is confirmed; a witness. [Bound the blocks' cost without a stop, R-114-m1e item 6.]
10. CC8D-110. D9.3's fence names internal/goal, but handlers are in cmd/metasystem/goalsync_mutations.go:2260-2289 and context_verbs.go:139; `goal.VerbRequest` (goal/verbs.go:197-220) has no runtime session. Asked: the identity token, its propagation, every entry point; refusal scoped to the handed-off session only. [Candidate token: `RuntimeSession`, excerpt 5.]
11. CC8D-111. `openWork` drops `WaiterTarget` (excerpt 10) and a fresh wait resolves current work (cmd/metasystem/wait_verb.go:180-223), so a reused id can bind a later lifecycle. Asked: persist and verify the full target; register it exactly or refuse a changed lifecycle; a reused-id witness.
12. CC8D-112. The 100 ms bound covers a proposed reader, not the hook: no byte or deadline option (excerpt 7), 32 MB lines (excerpt 8), the reader reads to EOF then writes the cursor (usage/cursor.go:135-215), the hook runs `lease hook-delegate` and engine resolution first (supervision-hook.sh:1464-1476, :1581-1589). Asked: early path, bounded-read API, total deadline, what timeout skips; an end-to-end witness (256 KB tail, 32 MB partial line, cold process, slow engine) keeping the allow paths. [Injected clocks.]
13. CC8D-113. Any path modified after a whole-second `PidStartedAt` (excerpt 5) passes. Asked: constrain location, validate content against delegate records, tie to this write. [Narrowed: S7's check is binding (excerpt 12); keep it, allow only the seat memory directory, require every declared delegate's output path in the note, resolve a same-second prior note by digest or strict order; no machine judgement of lessons text.]
14. CC8D-114. U3a-1 omits `internal/config/validate.go` (numeric knobs registered at :496-512) and `metasystem.conf` (spend keys only, :27-35). Asked: validation, defaults, docs for both keys; positive ceiling, margin below it; witnesses for unknown, malformed, non-positive, inverted.
15. CC8D-115. `lessonsNote` is scalar in D4.4, a list in D4.5; `splitHandoffStateLists` moves openJobs, scratch, messagesOwed only (handoff_capture.go:736-749); the state is schema 1 (excerpt 11), strict-decoded (handoff_state.go:139-151), other versions refused (:304-307). Asked: scalar or list, which collections overflow, a compatible or migrating schema; round-trip and old-schema witnesses.
16. CC8D-116. "About N" sizes, tests missing from file lists, bed legs as sole witnesses (D2.1, D6.1). Asked: exact `Changed-line allocation` at or under 300 including tests and docs (excerpt 13), a focused witness per rule (P7, program page :130: "Every owning goal's design gives each rule a focused witness a builder runs without a fixture bed or a live session"); bed legs stay integration evidence.

Not answered: CC8D-117 (non-material). Revision 1's questions 1 and 2 are posed wrong (CC8D-103, 107): rewrite or drop; 3 and 4 stand.

Cited code excerpts (copied byte-exact from origin/main a24ecff03; paths relative to the metasystem module):

1. `internal/lease/claim.go:107-122` (CC8D-101)

```text
		if leaseLineage(current) == announcementLineage(ann) &&
			announcementLineage(ann) != "" &&
			holderIsMissionRunner(c.root, current) &&
			descendsFrom(ann.Pid, current.Pid) {
			// Kernel ancestry is the whole fence: the mission's
			// host turn AND the runner's own detached loop are both real
			// children of the runner-side holder; a twin runner or an
			// impostor session, whatever its tags claim, is not.
			return c.succeed(current, ann)
		}
		// A live holder that is a genuinely different process never loses the
		// lease, whatever the lineage.
		c.emitter.Emit(c.root, "lease-refused",
			fmt.Sprintf("live holder %s keeps the lease", current.HolderMainId),
			map[string]string{"holder": current.HolderMainId})
		return nil
```

2. `internal/steward/handoff_capture.go:322-340` (CC8D-101)

```text
func admitHandoffCaller(root string, caller HandoffCaller) (HandoffCaller, error) {
	switch caller.Class {
	case handoffClassMain:
		if err := requireObservableHandoffRuntime(caller.Runtime); err != nil {
			return HandoffCaller{}, err
		}
		if caller.MainId == "" || caller.MainId != caller.HolderMainId || caller.Machine == "" || caller.Runtime == "" || caller.Session == "" ||
			caller.Ref.Pid < 1 || caller.Ref.Mode() == identity.CompareInvalid || caller.Tag == "" {
			return HandoffCaller{}, refusal("HANDOFF_NOT_HOLDER", "")
		}
	case handoffClassDelegate:
		var err error
		caller, err = activeDelegateCaller(root, caller)
		if err != nil {
			return HandoffCaller{}, err
		}
		if caller.Machine == "" {
			return HandoffCaller{}, refusal("HANDOFF_NOT_HOLDER", "active continuation has no machine identity")
		}
```

3. `cmd/metasystem/context_verbs.go:223-233` (CC8D-101, the existing delegate-class caller)

```text
	if classified.Class == lease.ClassDelegate {
		jobID, active, err := steward.ConsumedActiveJob(stateRoot)
		if err != nil || !active {
			return caller, err
		}
		delegate, err := hookContextHandoffDelegate(stateRoot, stateRoot, jobID, int64(os.Getppid()))
		if err == nil && delegate.Delegate && delegate.JobID == jobID {
			caller.JobId = jobID
		}
		return caller, err
	}
```

4. `internal/goal/verbs.go:502-507` (CC8D-106, CC8D-110)

```text
// ownPair reports whether a claim names the actor's machine AND
// lineage — the pair is the ownership key, never the
// machine alone: a second lineage on the machine is a stranger.
func ownPair(c *ClaimRecord, a Actor) bool {
	return c != nil && c.Machine == a.Machine && c.Lineage == a.Lineage
}
```

5. `internal/lease/classify.go:21-28` (CC8D-110, CC8D-113; member A's fields)

```text
type Announcement struct {
	SessionId              string `json:"sessionId"`
	RuntimeSession         string `json:"runtimeSession,omitempty"`
	PreviousRuntimeSession string `json:"previousRuntimeSession,omitempty"`
	SessionAssociatedAt    string `json:"sessionAssociatedAt,omitempty"`
	MainId                 string `json:"mainId,omitempty"`
	Pid                    int64  `json:"pid"`
	PidStartedAt           int64  `json:"pidStartedAt"`
```

6. `internal/steward/revive.go:189-206` (CC8D-105)

```text
	consumed, err := ConsumeIntent(repoRoot, it.Nonce)
	if err != nil {
		return ReviveOutcome{}, err
	}
	// The attempt counts against the dry cap the moment it is
	// irreversible — before dispatch, so no crash window between
	// launch and bookkeeping can spend attempts the cap never saw.
	ev = RecordRevival(ev)
	if err := SaveEvidence(repoRoot, EvidencePath(repoRoot), ev); err != nil {
		return ReviveOutcome{}, err
	}
	if err := launch(consumed); err != nil {
		return ReviveOutcome{Escalate: true, Reason: "dispatch failed after consumption; next tick reconciles: " + err.Error()}, nil
	}
	if err := StampLaunch(repoRoot, consumed.Nonce); err != nil {
		return ReviveOutcome{}, err
	}
	return ReviveOutcome{Launched: true, Reason: "continuation dispatched for " + consumed.Goal}, nil
```

7. `internal/usage/calls.go:64-72` (CC8D-112)

```text
type ReadOptions struct {
	Capability   Capability
	Transcript   string
	Home         string
	Toplevel     string
	Installation string
	Now          time.Time
	NonBlocking  bool
}
```

8. `internal/usage/cursor.go:19-19` (CC8D-112)

```text
const maxCallLineBytes = 32 * 1024 * 1024
```

9. `internal/runtimes/registration.go:69-72` (CC8D-108)

```text
		{ID: "enforcement-config", Operation: OpJSONStripKey,
			Requiredness: Requiredness{TemplateSource: "required", AdoptedDestination: "required"},
			Destination:  ".claude/settings.json", Policy: PolicyTransformedBytes,
			InstructionBearing: true, Source: "scripts/enforcement/claude-code-hooks.json", Key: "_comment"},
```

10. `internal/run/waiter.go:45-53` (CC8D-111)

```text
// WaiterTarget pins the waiter to one lifecycle.
type WaiterTarget struct {
	StartedAt   string `json:"startedAt,omitempty"`           // jobs
	Round       int64  `json:"round,omitempty"`               // jobs
	OperationID string `json:"operationId,omitempty"`         // jobs
	Generation  int    `json:"generation,omitempty"`          // runs
	LaunchNonce string `json:"launchNonce,omitempty"`         // runs
	ProofDigest string `json:"proofIdentityDigest,omitempty"` // attempts
}
```

11. `internal/steward/handoff_state.go:22-22` (CC8D-115)

```text
const HandoffSchemaVersion = 1
```

12. `docs/orchestration.md:370-370` (CC8D-113, seat rule S7)

```text
| S7 handoff keeps lessons | A handoff is recorded only after the seat's memory note holds this session's lessons, its reasoning in flight, and every in-flight delegate's output path. | The note's modification time precedes the handoff record. |
```

13. `docs/design/design-principles.md:120-125` (CC8D-116)

```text
When a design divides implementation into units, allocate at most 300 changed
lines to each unit. Count additions plus deletions across the whole candidate,
including production code, tests, scripts, and documentation. State the
allocation explicitly in the design's implementation map and in every unit
brief as `Changed-line allocation: <number>`. This planning maximum leaves
room below a separately declared enforcement ceiling; it neither sets nor
```

Example page:

Revision 1 above is the shape, plus: section 9 maps each finding id to its decision; section 6 rows like `| U3a-1 keys | internal/config/context.go, context_test.go, validate.go, metasystem.conf, docs | Changed-line allocation: 240 | DONE | TestContextBudgetConfigRefusesInvalidValues | after U6a |`; each open question names the ruling it touches.

## Seat rulings

Seat m1e (coordinator) rules on the dispositions in ctx8d-crit-r1-summary.md, 2026-09-15. These rulings are binding for revision 2; do not re-argue them.

- SR1. Dispositions. All sixteen material findings CC8D-101 to 116 are accepted and folded. CC8D-113 is accepted in the narrowed form: S7's check stays the modification-time order (docs/orchestration.md seat rule S7); r2 adds where the note lives (the seat's memory directory), that the note names every declared in-flight delegate's output path, and strict same-second handling; r2 does not machine-check the content of lessons or reasoning. CC8D-117 is rejected (the header name is right in the scratchpad).
- SR2. Authority while two sessions live (101, 106, 110). r2 adds no second exception to the rule that a live holder never yields (lease/claim.go:117-122). The successor works through paths that exist: claim ownership is machine plus lineage (goal/verbs.go:502-506), and an active continuation already records a handoff in the delegate class (context_verbs.go:223-233). The lease and main-ness move only when the predecessor has ended, and the predecessor ends only after its successor is confirmed started (program page P2). The handed-off session's goal mutations are refused by a fence keyed on member A's RuntimeSession token (lease/classify.go:23-25), which member B1 of registered-wait-matches-the-runtime-session makes the hook pass; name B1 as a dependency. If r2 finds a live-predecessor lease handover unavoidable, it poses that as an open question for Wido with a recommendation and puts the unit that needs it behind his word; every other unit still builds.
- SR3. Unit graph (102, 116, 114, 108). No unit calls a surface before the unit that adds it. Every U3c unit follows all of registered-wait-matches-the-runtime-session (members A landed c4e7d0f31, B1 in build, B2, B3, C open). Every unit states `Changed-line allocation: <number>` (docs/design-principles.md:120-130), its test files, and a focused witness that needs no fixture bed and no live session (program page P7). Aim the total near P7's about 1,320 lines for U3a, U3e, U3b and U3c; if more is needed, say per unit why, and never narrow a DONE to fit. U3a-1 names validate.go and metasystem.conf; U3e edits the shipped hooks file scripts/enforcement/claude-code-hooks.json and what `hooks check` reads, not .claude/settings.json.
- SR4. Proof floor (103). The goal's DONE numbers stand: p95 under 150K and no call over 200K. R-114-m1e item 3 fixes only the ceiling (250K in configuration, trigger at ceiling minus margin); r2 chooses the margin and gate so the DONE numbers hold, with the arithmetic from the diagnosis. No interim build at weaker numbers. Open question 1 is withdrawn; ask Wido only if r2 shows with evidence that the numbers cannot be reached.
- SR5. In-flight delegates (104). Observed by m1e in this Claude Code session on 2026-09-15: an Agent launch returns at once with a tool_result carrying the agent id and output file; completion arrives later as a user-turn <task-notification> with <task-id> equal to the agent id, <tool-use-id>, <output-file> and <status> (completed, and other terminal states); the same task id may notify more than once if the agent is resumed. A background Bash command follows the same shape (task id, status, exit code). A delegate or background command is in flight from its launch result until a terminal notification for its task id. Background Bash tasks count as in-flight work the same way as Agent delegates.
- SR6. The hook (107, 112). The gate never blocks a landing or an in-flight wait (R-114-m1e item 5): at least `metasystem landing`, `goal land-ready`, `metasystem wait`, `job watch` and the landing driver stay allowed at every size; no absent command is named. Open question 2 is dropped unless r2 re-poses it on the corrected grammar. The 100 ms bound covers the whole hook invocation including `lease hook-delegate`, with a byte cap and a deadline in the reader's options; timing witnesses use an injectable clock, never wall time (Wido, 2026-09-12: artificial clocks, never load-fragile tests).
- SR7. Launch and failure lifecycle (105, 109). No path ends a session with no successor started (program page P2). r2 does not rely on "the tick retries": a failed launch leaves a live, restageable intent or an explicit escalation that a named actor acts on, matching revive.go, runner.go and reap.go as cited.
- SR8. Records (111, 115). openWork carries every WaiterTarget field a fresh registration needs (waiter.go:45-53, :133-176). The handoff state gets a schema version bump with a migration or a dual reader (handoff_state.go:22, :139-151).
- SR9. Open questions 3 and 4 stay as posed. 8c.7 (held handoff expiry) stays out of scope and waits on Wido.
- SR10. Work style for the designer: batch independent reads into one request; open no file outside the context pack except to check a cited line.

## Tool-call budget

Maximum delegate tool calls: 25

Stop when this number is reached. List anything the budget did not allow you to check.

## Page-size ceiling

Maximum page size: 450 lines

Cut a draft that exceeds this ceiling. If cutting would make the page incomplete, stop and propose a split instead.

## Fresh session

This revision runs in a new delegate session. Use the prior page and critique only through the context pack above. Never resume a delegate from an earlier revision.

## Rulings to honour, verbatim

- R-115-m1e item 2: "Design questions on coordinator-context-stays-under-budget: U3b's open-work record lists every in-flight Agent delegate (id, what it was asked, output path) or the quiet-point rule adds no delegate running; U3c's successor starts within a stated number of seconds, not a tick, dead time from the handoff record to the successor's first act is recorded, and the console pane shows the worker's status (goal next and the health line)."
- R-115-m1e item 6: "Program rule: no unit of the efficiency program lowers a proof floor, removes a witness or a gate, or narrows a DONE to save tokens, and each unit's read checks it."
- R-114-m1e item 2: "the automatic launcher: the steward continuation, a headless session the steward starts. No machinery may assume or rely on tmux." Item 3: "a context ceiling of 250K in configuration, with the trigger at the ceiling minus the margin." Item 5: "build U3e, a separate tool-use hook under about 100 ms that never blocks a landing or an in-flight wait, so the DONE's stated size holds by construction." Item 6: "Never stop work because of a limit: a cap or budget triggers a handoff or a delegate that continues the work, never a stop and never a wait on Wido."
- Wido 2026-09-14 (fix upstream, not at the check): every rule the design adds names the check or test that fails if the rule is removed, so the builder proves each rule by mutation.
- Seat rule S6 (docs/orchestration.md:369): "Run `metasystem context resume` first when a handoff waits; otherwise read the `context-budget` line and run `goal next`." S7 is excerpt 24.

## Page artifact and return shape

Write the page to: /private/tmp/claude-501/-Users-wido-LocalStorage-GitHub-agentic-tools-m1e/9b7e334d-919d-47e9-9555-a9f2ed557072/scratchpad/coordinator-context-8d-design-r2.md

Return only these two lines:

```text
/private/tmp/claude-501/-Users-wido-LocalStorage-GitHub-agentic-tools-m1e/9b7e334d-919d-47e9-9555-a9f2ed557072/scratchpad/coordinator-context-8d-design-r2.md
DESIGN: ready (<lines> lines)
```

or:

```text
/private/tmp/claude-501/-Users-wido-LocalStorage-GitHub-agentic-tools-m1e/9b7e334d-919d-47e9-9555-a9f2ed557072/scratchpad/coordinator-context-8d-design-r2.md
DESIGN: blocked (<reason>)
```
