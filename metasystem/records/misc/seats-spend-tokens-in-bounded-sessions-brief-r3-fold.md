# Fold brief: seats-spend-tokens-in-bounded-sessions (1:1), revision 3 folded

Kind: design. Designer: Claude Fable, one fresh delegate. Worktree root: /Users/wido/LocalStorage/GitHub/agentic-tools-m1e/.claude/worktrees/tokens-design; the Go module is its metasystem directory, and every path below is relative to it. Edit exactly one file, in place: plans/seats-spend-tokens-in-bounded-sessions-design.md (revision 3, 186 lines). Change nothing else. Run no tests, start no process, make no commit, send no message.

Call budget: 20 tool calls in total, reads included. Page ceiling: 230 lines. This is a fold, not a redesign: change only what the rulings below name, and keep every other decision as written.

## Why

Critique round 3 (artifacts/reports/tokens-design-critique-r3.md, 232 lines, read it whole) was the goal's last review round: 11 material findings, verdict rework. Wido decided on 2026-09-15 (option a): one small fresh fold with no further critique round, then land the page as the program record. The findings fold, move, or stay open as the seat rules below say.

## Seat rulings, binding

Fold these on the page:
- SSTB-302. Every over-trigger context Stop and every idle Stop keeps blocking until the running signal (the consumed intent's launchStamped plus a running continuation job with a probed pid); no Stop is allowed and no session is marked handed off before that signal. Delete option 1(b), because it brings back predecessor death. A capture fault does not block, so there is no refusal loop: it is a named fault with the attention item `context-handoff-capture-fault`, and the seat keeps working under S1.
- SSTB-306. The baseline ends at 12:51 CEST, the diagnosis's real end. Ruling R11's 12:45 was the seat's error; say so in one clause, and use 12:51 everywhere.
- SSTB-308. U1 (registered-wait-matches-the-runtime-session) lands before U3c. Name the dependency in the P7 map and in U3c's cell.
- SSTB-309. S3 reads: "a design revision or correction is a fresh delegate; `delegate --follow-up` only for critique rounds on the one critic chain". Fix S3's check to match.
- SSTB-311. One P7 line: every owning goal's design gives each rule a focused witness a builder runs without a fixture bed or a live session (not only U1).
- S1 today: add one clause. Recording the handoff today starts no successor (F2), so the seat ignores the gate's "end this session" display and keeps working.
- SSTB-312 (non-material): refer to goals by id only; drop priority sequence numbers, because they shift.

Move these into U3c's design questions under coordinator-context-stays-under-budget, and retract the page's sentences that assumed them:
- SSTB-301. Replace "under the same hooks" (near line 62) with the requirement: a headless continuation must receive a Stop trigger and can record its own next handoff. U3c's design names the hook contract, because delegates exit the supervision hook today (supervision-hook.sh:1451-1475, :1581-1586) and job settings install only SessionStart (claude.go:122-131).
- SSTB-305. The predecessor's silent Stop must be an output the hook and the adapter accept: the hook treats an empty display as unreadable (supervision-hook.sh:2368), and internal/adapter/stopoutput.go:91-97 writes systemMessage on every allow. Add both files to U3c's cell.
- SSTB-310. The pid and job exclusions need identities, not counts: add internal/steward/census.go workersFromVerdict, verdict.go:47-64 and revive.go:280-285 to U3c's cell.

Record these as open, with their owners:
- SSTB-303 goes to U3c's design as a required decision: how claims and lineage authority move to the continuation. Claims key on machine plus lineage (verbs.go:502-506, :968-969), HandoffBinding and the stage.go intent carry no claimant, and dispatch.sh clears the claim epoch and main id. Say P2's successor may not claim or land until that decision is built.
- SSTB-304 goes to Wido by folding it into open question 4: what the DONE's "stays under a stated context size by construction" promises. Either the Stop-sampled trigger with a stated observed overshoot (the DONE wording changes), or the in-turn bound U3e (the DONE stands). Recommend one, with the number.
- SSTB-307 goes to 1:1's proof step. Mark the per-seat lines (the 150M line and the ten-refusal line) as forecasts, not proof thresholds. Before the proof day, the thresholds are derived per seat from the spend-fence verb's own cause counts, with a named landing source. The three-seat total and the main-share lines stay as stated.

## Page additions

Header: "revision 3, folded after critique round 3 by Wido's decision (option a), 2026-09-15; no further critique round". Critique record: one line per round-3 finding, SSTB-301 to 312, with its disposition (folded, moved to U3c, or open with owner). Keep the four open questions, with question 4 now carrying SSTB-304.

## Return

At most 10 lines: the page path and line count; each ruling done (yes or no, with the line numbers touched); anything you could not fold and why; the calls used.
