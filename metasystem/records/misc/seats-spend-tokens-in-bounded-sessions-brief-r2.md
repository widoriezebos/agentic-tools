# Design brief: seats-spend-tokens-in-bounded-sessions (1:1), revision 2

Designer: Claude Fable, one fresh delegate. Worktree root: /Users/wido/LocalStorage/GitHub/agentic-tools-m1e/.claude/worktrees/tokens-design, now on trunk at a1d23839f. That tip contains 02677d248 (context-handoff-accepts-every-waiter-state) and e700328e1 (code-reads-and-critiques-run-bounded). The Go module is the worktree's metasystem directory, and every path below is relative to it. Write exactly one file: revise plans/seats-spend-tokens-in-bounded-sessions-design.md in place as revision 2. Change nothing else. Run no tests, start no process, make no commit, send no message.

Call budget: 55 tool calls in total, reads included. Page ceiling: 450 lines. Grep for headings before reading a long file, and read only the sections you need. If the budget runs short, write the page with what you have and say what you did not check.

## What stays

The revision 1 brief, artifacts/reports/tokens-design-brief-r1.md, still holds: its Why, context pack, harness facts, decisions Q1 to Q5, unit rules, non-goals and page shape. The seat rulings below override it where they differ.

## Inputs, read in this order

1. artifacts/reports/tokens-design-brief-r1.md (58 lines), whole.
2. plans/seats-spend-tokens-in-bounded-sessions-design.md, revision 1 (184 lines): the page you revise.
3. artifacts/reports/tokens-design-critique-r1.md (254 lines), Codex round 1, whole. It has 17 material findings (SSTB-101 to 117), 2 non-material (118, 119), and the verdict rework. The seat accepts every material finding; the rulings say how each theme closes.
4. The code and pages each finding cites, only as needed. New in the pack: plans/registered-wait-matches-the-runtime-session-design.md (grep its headings; read its decisions and members A, B1, B2, B3 and C), and `git show --stat e700328e1` with the receipt fields read_tokens and read_calls it added.

## Seat rulings, binding

R1. Cap and forced handoff (SSTB-102, 103, 108). The trigger fires before a call crosses the cap, not after it. Choose the sample (the last call's usage in the transcript), the margin, and the path by which the hook's transcript_path reaches the reader. A sample that is unknown or unreadable never blocks; it shows a report line. At 02677d248 the handoff still refuses registering and pending waits (HANDOFF_WAIT_IN_FLIGHT, exit 9). Say exactly what the seat does when the cap fires with a wait in flight: wait for it, cancel it, or change the handoff. Say how the seat recovers after exit 9. The successor registers its own waits fresh; it never inherits the predecessor's resume command. A change to what the handoff captures is allowed. A change to its authority rules is not (non-goal).

R2. Refusal policy and liveness (104, 107). Do not replace the separate refusal policies (unreadable ledger, idle escalation, unwatched digests, open-work signature, goal revisions) with one shared counter. Change only the idle-backlog rule, and state the third-Stop outcome and its safety rule for each block source you touch. Allowing Stop with no systemMessage is safe only when the harness will wake the seat when the work ends. State the outcome when the waiter crashes, when the child exits before delivery, and when an Agent run was never registered.

R3. Owners (101, 106, 116). Build on the registered-wait design: members A, B1, B2, B3 and C, in that order. Do not redraw its members. Redraw each unit's Boundary and Ceiling around the files that really own the rule, including the ones the critique names (handoff.go, intervene.go, stage.go, revive.go, internal/spend/cache.go). Split any unit that no longer fits 300 changed lines. U2 includes the pending human question half, or names the goal that owns it.

R4. Launcher, seat rules, witnesses (109, 110, 117). U4 builds on the existing metasystem delegate path with a judgement pause, not a Bash state machine beside it; if it cannot, say why in one line. SSTB-110's one-chain rule covers critique rounds only. A design revision still runs as a fresh delegate: that is Wido's rule, and the sibling goal design-delegates-run-fresh-and-bounded (m1c is building it now) owns it; reference it, do not redesign it. Split U6 into rules a seat can follow today with no build, and rules that wait for a named unit. No rule may conflict with the critique-chain rule or with running context resume first. Every rule gets a focused Go witness a builder can run without a fixture bed or a live session. Shell legs and live runs are proof of DONE, not witnesses.

R5. Measurement scope (111, 114, 115). State the time window (the baseline is the CEST day to 12:45 on 2026-09-15 across m1b, m1c and m1e), how the three seats are added up, a baseline of the same scope, and a landed-unit rule that counts each landing once (a release is not a landing). Cover delegate subtypes. Compare with provider usage for Claude only; Codex accounting stays a non-goal. Add the five-read token measurement that code-reads-and-critiques-run-bounded handed to 1:1: it reads the receipt fields read_tokens and read_calls from e700328e1.

R6. Forecast and open questions (105, 112, 113). Wido has not answered the three open questions. Keep all three, each with a recommendation, and make the units build under either answer. The cap is a configuration value, not a constant. Name the units that change for a pane relaunch versus a headless successor, and for a peer-message rule versus an inbox. Redo the expected day without the double-counted 20M and recompute the thresholds from it (the critique's corrected figure is 159.6M, with the main share at 42.3 percent, over the 40 percent line). Drop the 15.8M peer saving unless a unit causes it.

R7. Non-material 118 and 119: fold each if it costs a line or two; otherwise note it in the record.

## Page additions

Header: revision 2, and what changed from revision 1 in at most six lines. A critique record section: one line per finding, SSTB-101 to 119, with its disposition (folded where, or not folded and why). Keep unit ids stable where a unit survives; new units get new ids.

## Return

At most 15 lines: the page path and line count; the trigger and its margin; what happens when the cap fires with a wait in flight; the stop gate's liveness rule; the unit list (id, goal, changed lines, tokens saved); the U6 rules that apply today; the open questions; any finding not folded and why; the calls used.
