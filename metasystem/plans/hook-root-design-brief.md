Working Mode: design
Orchestrator Identity: m0b+main-1788250419-3170380-8a1fb3 (dispatch delegate under goal supervision-hook-wrong-root)
Date: 2026-09-02

# Goal

Author metasystem/plans/supervision-hook-root-design.md: the design fixing
the harness hook's root resolution so every seat's turn signal reports the
metasystem world it actually works in. Small goal, 4-hour box.

# The proven defect (three seats, plus a consequence specimen)

metasystem/scripts/agents/supervision-hook.sh line 65 resolves
`git -C "$cwd" rev-parse --show-toplevel` — the OUTER repository root on
this fleet's layout (the metasystem checkout is a subdirectory of a wrapper
repository). Every downstream decision (goal ledger read, health, digest,
turn generation) then runs against a root with no metasystem world:
hook-freshness has been dead since enrollment on m2, m3, and m0b
identically, and on 2026-09-01 the blind signal failed to refuse a seat's
idle turn-end (goal record: plans/goals/supervision-hook-wrong-root.md).
The hook's own resolution contract (its header comment) and its benign-exit
discipline are binding context.

# Design questions to settle mechanically

1. ROOT RESOLUTION: from the resolved cwd, find the governing metasystem
   root — the nearest ancestor-or-descendant directory carrying the
   metasystem markers. Decide the marker set (the engine at bin/metasystem
   beside plans/goals/? the artifacts directory?) and the search rule
   (ascend from cwd first; on reaching the git toplevel, check
   toplevel/metasystem — the fleet's actual layout), with the
   single-checkout and nested layouts both resolving to the same answer.
2. FAILURE SHAPE: preserve the hook's benign-exit discipline — no
   metasystem root found stays exit 0, never a guess.
3. FIXTURE: the test covers the general nested layout (repo root with
   metasystem/ subdirectory), the flat layout (metasystem root IS the git
   toplevel), and a cwd deep inside either; assert the resolved root by
   the world the hook then reports (the goal ledger it reads), not by
   string comparison alone.
4. BLAST: enumerate every consumer of the hook's resolved root inside the
   script; state why each behaves correctly under the new resolution.

Self-grade with reject condition per the house rule.

# Constraints

Wall-clock budget: 25 minutes. Design document only; write exactly one new
file metasystem/plans/supervision-hook-root-design.md.

# Expected Return

Version-2 implementer JSON; diffBoundary exactly
metasystem/plans/supervision-hook-root-design.md (that one file).

# Gap Rule

stop and report a gap; never fill it silently.
