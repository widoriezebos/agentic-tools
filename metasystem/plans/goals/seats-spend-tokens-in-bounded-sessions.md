# seats-spend-tokens-in-bounded-sessions

- State: queued
- Risk: severity=3 novelty=2 exposure=3 accumulation=3 basis="severity 3: about 244M tokens in a day for one seat, most of it re-reading one session's context; novelty 2: delegate-bounded coordination and handoff at a size, on top of existing context and stop-gate machinery; exposure 3: every seat, every turn; accumulation 3: session context only grows until handed off"
- Tier: 3
- Intent: On 2026-09-15 the three seats spent about 302M tokens in a day against a 250M alert line; seat m1e alone spent about 244M, because its coordinator session grew to about 590K tokens of context and re-read it on every turn, including every stop-hook refusal while it waited on background work and every background-task notification. Claude Code gives a seat no way to compact or restart its own session, so a long-running coordinator only grows. Wido prioritized this over everything else (2026-09-15). DONE: a seat's main session stays under a stated context size by construction: long pipelines (a unit's brief, build, verification, reads, sections and landing) and long reads run in bounded delegates that return short summaries; the session hands off to a fresh one at a stated size through the existing context handoff; waiting on registered background work costs no stop-hook turns; tokens per landed unit are measured per seat per day and shown in health; proven by a working day of three seats whose tokens per landed unit and per day fall below stated thresholds against the 2026-09-15 baseline.
- Origin: human
- Next step: Diagnose first, cheaply: split 2026-09-15's tokens per seat by cause (turns started by stop-hook refusals, background notifications, peer messages, user turns, and large reads) from the session transcripts and the spend fence, and name the top three causes with numbers. Then a Claude Fable design that builds on coordinator-context-stays-under-budget (context handoff), stop-gate-sees-harness-tracked-work and coordinator-wakes-on-events-not-polls rather than beside them; one Codex critique; build in units. Meanwhile m1e runs as a thin coordinator with delegates (memory m1e-handoff-2026-09-15-1400).
- OpenedAt: 2026-09-15T09:35:07Z
- Revision: 1
- Budget: elapsedLimit=1d attemptLimit=10 reservedJobMinutesLimit=1200 activeJobLimit=1 reviewRoundLimit=3
- BudgetExceptions: 0

History:
- 2026-09-15T09:35:07Z N1MEMJWFAH427K3GFXZAWFWVZ5-m1e-c6925449 open actor=human:Wido targets=seats-spend-tokens-in-bounded-sessions
Integrity: sha256=45d7f7fef62d97551ba29e2b4d6a73b8961063062fbefe2b66e5bf7deeb0e821
