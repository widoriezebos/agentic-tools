Working Mode: design
Orchestrator Identity: m0b+main-1788250419-3170380-8a1fb3 (dispatch delegate under goal digest-landing-race)
Date: 2026-09-02

# Goal

Revision 2 of metasystem/plans/live-records-landing-design.md: fold all nine
findings of metasystem/records/misc/live-records-critique-r1.md (landed, in
your worktree). Four are critical and they interlock — fold them as one
coherent mechanism, not nine patches. The critic's evidence suggests the
shape: a landing-wide mutex (LR-007) is the missing serializer that
dissolves the carry-to-guard race (LR-004) and the crash-strand state
(LR-008) if the carry happens inside it; the rebase byte-destruction
(LR-005) needs the writer paused or the union re-verified after rebase; the
append-only premise (LR-001, LR-006) must become enforced registry
machinery, not advisory convention; the staging-contract break (LR-003)
bounds where the carry commit may run; the unexplained conflict trail
(LR-002) must be explained by the mechanism you specify (which path do
those conflicts take that the union driver does not cover — rebase apply?
autostash?) with the honest answer stated; the proof plan (LR-009)
exercises the REAL commit wrapper and gates.

Consistency pass; self-grade; reject condition restated plainly.

# Constraints

Wall-clock budget: 35 minutes. Wido's session-coexistence goal owns the
broader two-lander problem — your mutex may cite it as the shared seam but
must stand alone for one checkout.

# Expected Return

Version-2 implementer JSON; diffBoundary exactly
metasystem/plans/live-records-landing-design.md (that one file).

# Gap Rule

stop and report a gap; never fill it silently.
