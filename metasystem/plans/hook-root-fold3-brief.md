Working Mode: design
Orchestrator Identity: m0b+main-1788250419-3170380-8a1fb3 (dispatch delegate under goal supervision-hook-wrong-root)
Date: 2026-09-02

# Goal

Revision 3 of metasystem/plans/supervision-hook-root-design.md: fold all
five findings of metasystem/records/misc/hook-root-critique-r2.md (landed,
in your worktree). These refine the installation-derivation mechanism; fold
them within it rather than pivoting again.

# The folds, by id

- SHR-R2-INSTALL-01: the invocation pathname is a candidate, not evidence —
  state what VALIDATES the derived installation (the engine's own identity
  answering from that root, per the critic's evidence) before any governed
  decision rides it.
- SHR-R2-WORKTREE-ENGINE-01: make the worktree decision operational in the
  real delegate layout the critic describes (the hook fires with a script
  path inside the primary checkout even when cwd is a linked worktree —
  ground the rule in the layout as shipped).
- SHR-R2-ENGINE-SKEW-01: a present-but-mismatched engine must NOT silently
  disable a governed Stop hook — decide the visible degradation (the
  drift-visible one-line report the critic sketches) while keeping true
  absence benign; this fleet rebuilds engines daily and lives this case.
- SHR-R2-WORKTREE-FALLBACK-01: a failed common-dir identification proves
  nothing — invert the fallback's proof burden per the critic's rule.
- SHR-R2-CONSUMER-01: add evidence garbage collection to the consumer
  enumeration and redo the one-world claim for mapped linked worktrees.

Consistency pass; self-grade; reject condition restated.

# Constraints

Wall-clock budget: 25 minutes. The five folds only.

# Expected Return

Version-2 implementer JSON; diffBoundary exactly
metasystem/plans/supervision-hook-root-design.md (that one file).

# Gap Rule

stop and report a gap; never fill it silently.
