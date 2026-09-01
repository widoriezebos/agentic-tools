Working Mode: design
Orchestrator Identity: m0b+main-1788250419-3170380-8a1fb3 (dispatch delegate under goal native-spend-cap-retirement)
Date: 2026-09-01

# Goal

Revision 5, one-finding fold of plans/spend-cap-retirement-design.md:
SCR-R4-COMMENT-PROVENANCE-001 (records/misc/spend-cap-critique-r4.md, in
your worktree). The design's specified ClaudeBudget doc comment cites
provenance (rulings, history); the repository's comment law says source
comments state the constraint in the system's own language — components,
invariants, failure modes — never the process that produced it. Rewrite the
specified comment text accordingly (e.g.: the budget flag is omitted unless
the operator sets METASYSTEM_CLAUDE_MAX_BUDGET_USD; the worker is bounded
by its wall-clock cap and turn limit; a set value is validated and a
malformed one is a protocol error). Disposition the non-material
SCR-R4-REJECT-SCOPE-002 as ruled-sound. Nothing else changes.

# Constraints

Wall-clock budget: 10 minutes. One comment specification and one
disposition line.

# Expected Return

Version-2 implementer JSON; diffBoundary exactly
metasystem/plans/spend-cap-retirement-design.md.

# Gap Rule

stop and report a gap; never fill it silently.
