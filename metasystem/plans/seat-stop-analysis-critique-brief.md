Working Mode: design
Orchestrator Identity: m0b+main-1788250419-3170380-8a1fb3 (dispatch delegate; Wido's direct order 2026-09-02)
Date: 2026-09-02

# Goal

Independent critique of metasystem/records/misc/seat-stop-analysis.md
(landed, in your worktree): the analysis of why seats stop with ready work
on the board, and the proposed deterministic turn-exit verdict. Wido's
order: machinery, not conduct, must make this impossible or nearly so.
Your critique gates the backlog item.

# Attack

1. The specimen analysis: is the common mechanism correctly identified, or
   is there a deeper cause the verdict would miss?
2. The verdict table: can READY be computed from existing engine records
   without re-implementing admission? Can INFLIGHT be proven from job
   records plus session lineage — check the actual record fields? Is the
   HUMANSTOP marker sound (who may set it, single-use semantics, the
   relayed-word path)?
3. The Stop-hook enforcement point: does the harness's Stop event actually
   support a blocking decision that re-prompts (check
   scripts/agents/supervision-hook.sh and the hook protocol it
   implements)? If not, name the real enforcement point.
4. The six listed failure modes: complete? Attack especially false-refusal
   loops and fail-open-on-unreadable-state.
5. Anything that makes this a WRONG mechanism (a nagging gate the seats
   learn to satisfy with hollow claims is worse than no gate — judge
   whether HUMANSTOP-by-relay reopens exactly the laundering this exists
   to close).

Findings material and grounded. Your return shapes the goal's intent.

# Constraints

Wall-clock budget: 25 minutes. Return per the design-critic schema.

# Gap Rule

stop and report a gap; never fill it silently.
