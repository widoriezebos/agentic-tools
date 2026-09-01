Working Mode: design
Orchestrator Identity: m0b+main-1788250419-3170380-8a1fb3 (dispatch delegate under goal native-spend-cap-retirement)
Date: 2026-09-01

# Goal

Revision 3 of plans/spend-cap-retirement-design.md: fold the two material
findings of records/misc/spend-cap-critique-r2.md (in your worktree; the
critic's full return is durable at
artifacts/agents/design-critic-0671f76f743c4926c1a39e8f/rounds/1/return.json)
and fix the one non-material impossible-scenario sentence it names.

# The mandate, by id

- SCR-R2-LEVEL-001: rederive the level against the TRUE permitted horizon —
  the 150-minute watcher cap, not 120 — and satisfy the design's own
  strictly-greater rule with stated headroom. Follow the critic's arithmetic;
  if that lands the default at 200.00, write 200.00; the rule decides, not
  round-number aesthetics.
- SCR-R2-PROTECTION-002: restate the worst-single-call overshoot at the size
  the critic's evidence establishes; propagate it through the overshoot
  bound.
- SCR-R2-OWNER-003 (non-material): delete or correct the impossible
  scenario sentence the critic quotes.
- Record Wido's disposition (R-42-m0b, landed in memory/rulings.md): the
  uncapped-fanout open scenario does not block this change; it is owned by
  goal uncapped-delegate-fanout.

Self-grade updated. Nothing else changes.

# Constraints

Wall-clock budget: 15 minutes. Numeric folds and one sentence.

# Expected Return

Version-2 implementer JSON; diffBoundary exactly
metasystem/plans/spend-cap-retirement-design.md.

# Gap Rule

stop and report a gap; never fill it silently.
