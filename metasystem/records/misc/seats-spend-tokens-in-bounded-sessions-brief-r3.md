# Design brief: seats-spend-tokens-in-bounded-sessions (1:1), revision 3

Designer: Claude Fable, one fresh delegate. Worktree root: /Users/wido/LocalStorage/GitHub/agentic-tools-m1e/.claude/worktrees/tokens-design, on trunk at a1d23839f. The Go module is its metasystem directory, and every path below is relative to it. Write exactly one file: revise plans/seats-spend-tokens-in-bounded-sessions-design.md in place as revision 3. Change nothing else. Run no tests, start no process, make no commit, send no message.

Call budget: 50 tool calls in total, reads included. Page ceiling: 350 lines. Grep for headings before reading a long file, and read only the sections you need. If the budget runs short, write the page with what you have and say what you did not check.

Revision 3 goes to critique round 3, the goal's last review round. A page that leaves a material program finding open cannot be critiqued again.

## What stays

Briefs r1 and r2 (artifacts/reports/tokens-design-brief-r1.md and -r2.md) still hold: the harness facts, the non-goals, and rulings R1 to R7. Where they conflict with this brief, this brief wins.

## Inputs, read in this order

1. This brief, then artifacts/reports/tokens-design-brief-r2.md (40 lines).
2. plans/seats-spend-tokens-in-bounded-sessions-design.md, revision 2 (247 lines): the page you revise.
3. artifacts/reports/tokens-design-critique-r2.md (173 lines), round 2, whole: 10 material findings (SSTB-201 to 210), 1 non-material (211). It reopens round-1 findings 102, 103, 105, 108, 110, 111, 112, 114, 115, 116 and 117; read those in artifacts/reports/tokens-design-critique-r1.md.
4. The code each program finding cites, only as needed. Start with internal/steward/handoff.go lines 85 to 110 (the pid holds), internal/steward/stage.go lines 140 to 170, the steward continuation role (grep scripts/agents/roles for steward-continuation), internal/report/contextreport.go lines 555 to 575, internal/run/waiter.go lines 55 to 70, and the events scripts/agents/supervision-hook.sh handles (grep `claude ` case labels) with the hooks .claude/settings.json installs.

## New binding facts since revision 2

F1. Wido, 2026-09-15: never stop work because of a limit. A context cap or a budget is a trigger to hand off to a delegate or a fresh session that continues the work. It is never a stop, and never a wait on Wido.
F2. m1c's live evidence, 2026-09-15: `metasystem context handoff` started no continuation three times (12:20, 14:30, and 15:05 with nonce aef24bbc5d8173d2). No successor session appeared, health context-budget stayed dead (287K, then 327K, against 200K), and the stop hook kept blocking on idle-backlog. So a seat stalls or runs over budget. Round 2 found the cause: handoff.go holds the intent while the predecessor pid lives and while any seat main lives, and an interactive seat cannot exit itself.

## Seat rulings for revision 3, binding

R8. Program page. Revision 3 is the lead page for the program, and it decides only program questions:
- P1, the trigger and its bound;
- P2, the successor start and the predecessor's end;
- P3, the stop-gate policy (R2, already closed in round 2: keep it, and change it only where P2 needs a change);
- P4, the measurement scope;
- P5, the forecast and the DONE thresholds;
- P6, today's seat rules (U6a);
- P7, the unit-to-goal map.
Each unit keeps an id, its goal, a one-line intent, an approximate size, its order and dependencies, and the design questions its goal must settle. Remove Boundary file lists, witness names and per-unit ceilings unless a program decision needs one. Each owning goal designs its own units from this page under its own review budget. Unit findings SSTB-207 (kind source detail under the spend-fence goal), 208 (every unit that adds a verb names cmd/metasystem/main.go; state that in one line), 209 (registered-wait-matches-the-runtime-session's own page owns the member allocations and the witness skip), 210 (units-run-through-one-launcher owns the rerun and critic re-entry), and reopened 116 and 117, move to the named goal's design questions in P7. Record each as moved, with its goal.

R9. P2, the successor (SSTB-202, 102, 103, 108, F1, F2). The successor starts without a human and without waiting for the predecessor to die, and the start is confirmed: a named, observable signal says the successor runs. Choose the launcher and give its numbers: the steward continuation with the pid holds in handoff.go changed, a headless successor the steward starts, or a relaunch of the seat's pane the steward drives. Then say what changes in handoff.go:93-101. Say how claims and lineage authority move to the successor. Say what the live predecessor does after the confirmation: its remaining Stops must cost no turns (the gate allows them with no systemMessage), and it must never be refused on idle-backlog again. The successor re-registers every wait it inherits with every field the waiter carries, including Chain and Poll; state the rule in one sentence and name the owning unit. Remove every path that ends a session with no successor started, including revision 2's lines 54, 56, 58, 60, 75 and 130. Open question 1 becomes: which automatic launcher. Pane relaunch by Wido is not an option.

R10. P1, the trigger (SSTB-201). One long turn can carry a Stop-time trigger far past the cap. Check whether the supervision hook already receives a tool-use event. If it does, bound growth inside a turn with it. If it does not, state the maximum overshoot as a number from the diagnosis (calls per turn and growth per call), and state the context size the DONE's "under a stated context size" then means. If that changes what the DONE promises, add it as an open question for Wido with a recommendation; the page may carry at most four open questions.

R11. P4, measurement (SSTB-204, 205, 206, reopened 111, 114, 115). One runtime scope: Claude tokens only. Codex engine-job tokens stay out, by an engine-window rule you state, and the baseline has the same scope (the critique computes 374.4M; check it). Say who owns the day selection and the 12:45 CEST end: the diagnosis script is a prototype, and the verb owns them. Give one landing identity with dedup: a landed unit counts once per landing commit, and revivals and multiple rows from one commit do not add. Give a semantic source for delegate kind (design, build read, critique, other) that does not rely on meta.json. Field-level detail goes to the spend-fence goal's design questions.

R12. P5, forecast (SSTB-203, reopened 105, 112). Use one handoff cost everywhere. Count sessions to the trigger, not to the ceiling. Recompute every threshold, including the 40 and 50 percent main-share lines. Add a per-seat forecast for the 150M line and the ten-refusal line. Show the derivation in a short table.

R13. P6, U6a (SSTB-110 reopened, F1). Today no automatic successor exists (F2). So today's S1 says what a seat does over the trigger with no build: it keeps working, moves every remaining multi-call step into fresh bounded delegates, records the handoff at a quiet point, and never stops or waits. Give each rule a check a reader can run on a transcript. Keep the critique-chain rule and context resume first.

R14. Non-material 211 (three wrong anchors): fix them.

## Page additions

Header: revision 3, with what changed from revision 2 in at most six lines, including the move to a program page. Critique record: one line per round-2 finding (201 to 211) and per reopened round-1 finding, each with its disposition: folded where, or moved to which goal's design.

## Return

At most 15 lines: the page path and line count; the trigger and its bound; the successor launcher, its confirmation signal and the handoff.go change; what the predecessor does after the confirmation; the measurement scope and baseline figure; the recomputed expected day and thresholds; the U6a rules for today; the unit-to-goal map (id, goal, approximate size); the open questions; anything not folded and why; the calls used.
