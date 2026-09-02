Working Mode: design
Orchestrator Identity: m0b+main-1788250419-3170380-8a1fb3 (dispatch delegate under goal breach-clock-and-budget-honesty)
Date: 2026-09-02

# Goal

Revision 4 of metasystem/plans/breach-clock-and-budget-honesty-design.md
(revision 3 landed, in your worktree). Sol's round-3 register is
metasystem/records/misc/breach-design-critique-r3.md: ONE material finding,
BCD-R3-001 (high), with the orchestrator's decision at its foot. Everything
else held. One fold, then the consistency pass over Fix 1 (record and schema
changes, the discharge rule, its consequences), the proof plan, the
disposition table (add the round-3 row) and the self-grade. Edit in place;
diffBoundary is that one file. Keep it under twelve minutes; verify against
the tree lines named here, no wider reading.

# The fold (BCD-R3-001)

The sequence discharge (obligation revision 5) → human set-obligation
(revision 7) → set-budget must leave the start at the episode origin, not
move it forward to the old discharge. Decision: the raise carries forward
which obligation was live the moment before.

1. Record: a third episode key on the claim binding,
   `episodeObligationRevision` (uint64), written by `rebindClaimKeepEpisode`
   from `file.Obligation.Revision` at the moment of the raise (0, and the key
   absent, when no obligation was live); `bindClaim` and `clearClaimBinding`
   (metasystem/internal/goal/verbs.go lines 122-124, 128-140) leave it 0.
   State the render and parse rule next to the existing two episode keys
   (metasystem/internal/goal/file.go render around lines 767-772 and the
   parse that accepts `episodeAt`/`episodeRevision`): the key may appear
   only when `episodeRevision` is present, exact wording for the refusal
   when it appears alone. Extend `ValidateClaimRevision`'s episode checks
   with `EpisodeObligationRevision > 0` implying `EpisodeRevision > 0`. The
   hand-edit mapper already refuses any altered Claimed line, so nothing new
   there; say so.
2. Rule: with `file.Obligation == nil`, a proof is eligible only when
   `Claimed.EpisodeObligationRevision > 0 && obligationRevision ==
   Claimed.EpisodeObligationRevision` (plus the episode-axis claim-revision
   filter already stated). Zero means no proof counts and the start is the
   episode origin. With a live obligation the live filter stands as revision
   3 states. Revise the short-circuit accordingly: return `episodeAt` when
   `file.Obligation == nil && Claimed.EpisodeObligationRevision == 0`.
3. Consequences to state, each as a sequence with the start after every
   step: discharge→raise (start stays T0+3h); discharge→set-obligation
   (T0, today's meaning); discharge→set-obligation→raise (T0, the finding's
   case); discharge→raise→set-obligation→raise (T0). A raise never moves the
   start in either direction.
4. Proof plan: one Go test per sequence above at the projection seam
   (`obligationBudgetStart`), plus a file.go parse test for the third key
   alone (refused) and with the pair (accepted); rename
   `TestSetObligationReturnsTheStartToTheEpisodeOrigin` or split it so the
   discharge→set-obligation→raise ordering is its own test by name.

Bump the header to revision 4 naming the round. R-31: no benchmarks.

# Constraints

Wall-clock budget: 12 minutes. Design only; edit nothing but the design
file.

# Expected Return

Version-2 implementer JSON; diffBoundary exactly the design file.

# Gap Rule

stop and report a gap; never fill it silently.
