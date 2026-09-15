# Design brief: seats-spend-tokens-in-bounded-sessions (1:1), revision 1

Designer: Claude Fable, one fresh delegate. Worktree root: /Users/wido/LocalStorage/GitHub/agentic-tools-m1e/.claude/worktrees/tokens-design (the Go module is /Users/wido/LocalStorage/GitHub/agentic-tools-m1e/.claude/worktrees/tokens-design/metasystem; every path below is relative to /Users/wido/LocalStorage/GitHub/agentic-tools-m1e/.claude/worktrees/tokens-design/metasystem unless absolute). Write exactly one file: plans/seats-spend-tokens-in-bounded-sessions-design.md. Change nothing else. Run no tests, start no process, make no commit.

Call budget: 45 tool calls in total, reads included. Page ceiling: 450 lines. Grep for headings before reading a long file, and read only the sections named. If the budget runs short, write the page with what you have and say what you did not read.

## Why

On 2026-09-15 the three seats spent 871.7M Claude tokens by 12:45 CEST (97 to 99 percent cache reads), against a fence line of 250M per day. The diagnosis (records/misc/token-diagnosis-2026-09-15.md, 375 lines, read it whole; sections 2, 2c, 2d, 2e, 3, 6 and 7 matter most) attributes tokens to what started each turn: background notifications 299.7M (34 percent), stop-hook refusals 284.2M (33 percent; 54 percent of m1c's day), peer messages 118.2M (14 percent). Context size multiplies every cause: main sessions carry 89 percent of the day and 89 percent of that comes from calls at 400K context or more; mean context per wake is 555K to 720K, peaks 966K. Delegates are 79.8M (9 percent), large reads 4.7M. If the 348 mid-turn message arrivals count as turns of their own, peers rank second and the stop hook third.

The goal's DONE (plans/goals/seats-spend-tokens-in-bounded-sessions.md, read it whole): a seat's main session stays under a stated context size by construction; long pipelines and reads run in bounded delegates; the session hands off to a fresh one at a stated size through the existing context handoff; waiting on registered background work costs no stop-hook turns; tokens per landed unit are measured per seat per day and shown in health; proven by a working day of three seats under stated thresholds against the 2026-09-15 baseline.

## Context pack, read in this order

1. plans/goals/seats-spend-tokens-in-bounded-sessions.md, then records/misc/token-diagnosis-2026-09-15.md.
2. docs/design/design-principles.md: the unit-slicing rule (300 changed lines per unit) and what a unit must state. Page shape: plans/coordinator-wakes-on-events-not-polls-design.md (264 lines) is the shape to follow; read its headings and its decisions and units sections.
3. The mechanisms this design builds on, never beside:
   a. The context handoff. plans/coordinator-context-stays-under-budget-design.md (940 lines): grep its headings; read the sections on the handoff verb, what it captures, how a fresh session resumes from it, and amendment 8c. Its goal record plans/goals/coordinator-context-stays-under-budget.md (next-step line). Code: cmd/metasystem/context_verbs.go (356 lines, read the handoff verb's entry and its refusal list) and internal/steward/handoff_capture.go (grep for the refusal codes only). Goal 1:2 (plans/goals/context-handoff-accepts-every-waiter-state.md) is being built now: the reader accepts every waiter state; take that as landed.
   b. The stop gate. scripts/agents/supervision-hook.sh (2530 lines; grep for `claude stop`, `stop-status`, `systemMessage` and `session_id`, and read only those regions: how the hook learns the session id and the transcript path, and how it returns a block). internal/goal/turnverdict.go around line 1006 (the idle-backlog verdict and the "refusal N of 3 for this unchanged backlog" rule) and internal/report/stoppresentation.go around line 1322. cmd/metasystem/session_stop.go (126 lines). Goal records plans/goals/stop-gate-sees-harness-tracked-work.md and plans/goals/registered-wait-matches-the-runtime-session.md. The wait registry record shape: internal/run/waiter.go, the Waiter struct only (grep `type Waiter struct`).
   c. Wakes. plans/coordinator-wakes-on-events-not-polls-design.md and plans/goals/coordinator-wakes-on-events-not-polls.md (three members landed; the parent owes clause 5 and the two legs).
   d. The spend fence. plans/goals/spend-fence-reports-tokens-per-model-and-cause.md; internal/spend (list the package's files and read the reader's type and function headings only) and the fence line in internal/steward/health.go (grep `spend`).
   e. Sibling goals to reference, not redo: plans/goals/design-delegates-run-fresh-and-bounded.md and plans/goals/code-reads-and-critiques-run-bounded.md.

## Harness facts, take as given; do not design around changing Claude Code

- A session's context only grows. There is no compaction verb a seat can call; the only reset is a fresh session that resumes from a handoff.
- The Stop hook runs supervision-hook.sh with a JSON payload on stdin (session_id, transcript_path, cwd). A block returns JSON with a systemMessage, which becomes a new turn at the session's full context, about four calls per refusal.
- Background Bash tasks, delegates (the Agent tool), Monitor events and peer messages (SendMessage) each wake the session with a notification turn, about six calls per wake at full context. A message that arrives mid-turn attaches to the running turn.
- The transcript ~/.claude/projects/<project-dir>/<session>.jsonl records every API call's usage (input_tokens, cache_creation_input_tokens, cache_read_input_tokens, output_tokens) with a timestamp; the context of a call is input + cache_creation + cache_read. The harness writes a delegate's transcript under <project-dir>/<session>/subagents/ and a background task's output under a tasks directory; the diagnosis script shows the shapes.
- A fresh delegate starts at zero context; a resumed delegate re-reads its whole transcript on every call.

## Decisions the page must make, each with numbers

Q1 Context cap. The context size at which a seat hands off, chosen from the diagnosis (means 555K to 720K, peaks 966K, 89 percent of tokens above 400K): compare 200K, 250K and 300K by expected cost per wake, refusal and peer message, and pick one. The measurement: the transcript's last call usage, read by the stop hook or by a verb. Where the trigger fires and what the seat sees: the stop hook's report line, a health line, a systemMessage that says hand off now, or a refusal of further work. What the handoff needs to succeed (no in-flight wait; registered background work, see Q2) and what the fresh session reads first. Say what changes in the existing handoff verb and what does not.

Q2 The stop gate sees harness-tracked work. How the hook learns that the session has live delegates or background tasks: the harness's task and subagent files under the project directory, a seat-registered wait in the wait registry with a pid or a file to watch, or both. The liveness rule for each. The verdict while such work is live: allow Stop with no systemMessage, so no turn is spent, and one wake when the work completes. The change to the idle-backlog rule (today three refusals per unchanged backlog; the diagnosis counts 103 refusals in a day).

Q3 Fewer wakes. The shape of one launcher per unit (brief, build, verification, read, sections, landing script) that notifies once; whether poll scripts and Monitor are allowed as separate background tasks; how peer messages are shortened or batched (a seat rule, and a mechanism only if it pays). Take the wakes goal's landed members as given and say what its remaining legs add.

Q4 Measurement. What 1:5 builds so the DONE can be proven: tokens per seat per day by cause (the diagnosis's classes) and by model, cache reads weighted separately from fresh input, tokens per landed unit, shown in health and the spend fence. The DONE thresholds against the baseline: 871.7M for the three-seat day and 264.4M for m1e's own fence figure. State the expected day after every unit lands, as a number with its derivation.

Q5 Seat practice from tomorrow, without a build: rules the seats follow at once (handoff size, one launcher per unit, delegates fresh with a call budget, message length), stated so a reader can check compliance from a transcript.

## Units

Slice into units of at most 300 changed lines each, ordered by tokens saved per line built. For each: id; the goal it lands under (an existing goal from the pack, or a new goal to open, with a proposed id, tier and one-line intent); Boundary (files); Ceiling; the rules, each with the witness test name that fails when that rule alone is removed; the expected token effect in numbers. The lead goal 1:1 keeps the measurement and the DONE proof; the mechanisms land under their own goals. Every unit is buildable by Codex from the page and its brief alone.

## Non-goals

No change to Claude Code itself. No Codex accounting (Codex is outside the 871.7M). No rework of the handoff's authority rules (C1a and C1b landed). No redesign of design-delegates-run-fresh-and-bounded or code-reads-and-critiques-run-bounded beyond referencing them.

## Page shape

Title and goal; evidence (the numbers); decisions Q1 to Q5, each with the chosen answer, the alternatives rejected in one line each and the number that decided it; the units table; open questions for Wido (at most three, each with a recommendation); verification rows (one per rule, with the witness name); a records line pointing at records/misc/token-diagnosis-2026-09-15.md and its script.

## Return

At most 15 lines: the page path and line count; the context cap chosen and its expected effect; the stop-gate mechanism chosen; the unit list (id, goal, lines, tokens saved); the open questions; the calls used.
