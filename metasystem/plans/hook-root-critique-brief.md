Working Mode: design
Orchestrator Identity: m0b+main-1788250419-3170380-8a1fb3 (dispatch delegate under goal supervision-hook-wrong-root)
Date: 2026-09-02

# Goal

Independent critique of metasystem/plans/supervision-hook-root-design.md
(landed, in your worktree): the fix for the harness hook resolving the
wrapper repository instead of the metasystem world (three seats' dead turn
signals; the shipped defect is
metasystem/scripts/agents/supervision-hook.sh resolving the git toplevel).

# Attack

1. The marker predicate and search rule: false positives (a wrapper that
   itself carries a metasystem.conf), false negatives (a checkout where the
   conf is machine-local only or renamed), multiple candidates (nested
   worlds — a delegate worktree inside artifacts/ carries the marker too:
   which world wins when the hook fires from inside a worktree, and is
   that the RIGHT world for turn reporting?).
2. The benign-exit discipline: every new failure path stays exit 0 without
   masking a real world.
3. The fixture plan: does it actually pin the layouts that broke (wrapper
   layout, flat layout, deep cwd, worktree cwd)?
4. The consumer enumeration: complete against the shipped script?

Findings material and grounded, quoting the design and script. A clean
return closes the design phase; the build ships.

# Constraints

Wall-clock budget: 25 minutes. Return per the design-critic schema.

# Gap Rule

stop and report a gap; never fill it silently.
