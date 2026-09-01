Working Mode: design
Orchestrator Identity: m0b+main-1788250419-3170380-8a1fb3 (dispatch delegate under goal native-spend-cap-retirement)
Date: 2026-09-01

# Goal

Revision 4, the final fold of plans/spend-cap-retirement-design.md: Wido's
recorded word R-43-m0b (memory/rulings.md, in your worktree) chooses the
design's own fully-specified Option-B alternative — the CLEAN KILL. Promote
it to the specification.

# The mandate

1. The specification becomes: ClaudeBudget passes NO --max-budget-usd when
   METASYSTEM_CLAUDE_MAX_BUDGET_USD is unset; a set override is validated as
   before (invalid_native_budget survives for a malformed value); time
   (wall-clock caps, watchdog) and count (turn limit) are named as the law,
   with the in-process turn limit explicitly recorded as the surviving
   in-process coarse dollar bound.
2. The $200 backstop material (level derivation, horizon coupling, overshoot
   sizing) moves to the rejected-alternative record with one line saying why
   it lost (R-43-m0b's reasoning: a forever-recalibrated constant buying a
   tighter number on a bound count already provides, in a never-observed
   scenario).
3. SCR-R3-LEVEL-001 is dispositioned as DISSOLVED BY DECISION — the number
   it calibrated no longer exists; cite R-43-m0b.
4. The build specification updates to the Option-B shape: the exact
   ClaudeBudget change (no default), the exact test changes (the assertion
   pinning "5.00" and the argv byte-order pins — state what each becomes
   when the flag may be absent), the mission-host paragraph (host budget
   export unchanged — it exports only when present, already Option-B
   compatible), and nothing else.
5. Open scenario uncapped-delegate-fanout and the enforcer-liveness
   residuals stay recorded exactly as they are — unchanged ownership.

Self-grade updated; the reject condition becomes: reject if the shipped
diff must touch anything beyond the ClaudeBudget function, its tests, and
the doc comment.

# Constraints

Wall-clock budget: 15 minutes. Promotion of an already-specified
alternative, not new design.

# Expected Return

Version-2 implementer JSON; diffBoundary exactly
metasystem/plans/spend-cap-retirement-design.md.

# Gap Rule

stop and report a gap; never fill it silently.
