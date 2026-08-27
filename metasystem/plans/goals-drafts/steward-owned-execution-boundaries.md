# steward-owned-execution: boundary rulings (provisional)

Status: PROVISIONAL COORDINATOR PICKS, 2026-08-26. Wido authorized
narrowing in his absence ("Narrow it myself... you overrule any of
them later"). Each pick takes the most conservative answer — the
narrowest widening of authority, the strongest attribution, the
strictest concurrency. Any of these flips on his word; the design
slice consumes them as inputs, not as settled law.

## Q1 AUTHORITY — registry-bounded, nothing ad hoc

The steward may execute exactly what the unit registry declares.
Registry changes are ordinary reviewed landings (design brief →
critique → certification like any other change). No verb exists
for "run this arbitrary command as the steward" — the registry IS
the authority boundary, keeping today's deliberately-narrow
steward authority narrow after the widening.

## Q2 IDENTITY — per-run announced identity, steward as parent

Each unit run gets its own announced identity that the steward
parents. Costlier than reusing the steward's identity, but: leases
are per-unit, incident attribution names the run rather than the
steward, and a wedged unit's cleanup cannot confuse the steward's
own standing. Conservative for custody and audit.

## Q3 CONCURRENCY — one unit at a time per checkout

Single-writer law holds: the steward runs one unit at a time
against a checkout. Batteries are the one sanctioned exception and
only via gate-run-freeze's isolated worktrees — the exception
lives in THAT design's isolation guarantees, not in a relaxation
of the steward's own rule. (The just-landed checkout execution
guard enforces the same law mechanically; the steward's runner
must acquire it like every other entrant, which it gets for free
by spawning through the guarded entrypoints.)

## Q4 GATE-RUN-FREEZE RELATIONSHIP — complementary, cross-cited

The freeze governs what runs DURING a battery; the steward governs
HOW unattended work runs at all. They stay two designs. The
battery unit's registry entry declares that it launches into the
freeze's isolated worktree; each design document references the
other at that seam. No merged mechanism.

## Effect on the proposed slices

Slice 1 (2h) proceeds with per-run identity (Q2 pick) and the
registry + execute verb + stamp. Slices 2 and 3 unchanged. Design
slice remains 3h per the goal's appetite; these picks are its
starting constraints.
