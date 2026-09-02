Working Mode: design
Orchestrator Identity: m0b+main-1788250419-3170380-8a1fb3 (dispatch delegate under goal breach-clock-and-budget-honesty)
Date: 2026-09-02

# Goal

Revision 5 of metasystem/plans/breach-clock-and-budget-honesty-design.md
(revision 4 landed, in your worktree). Revision 4 marked ONE point OPEN for
the orchestrator (the consequence bullet "discharge → raise → raise" in Fix 1
and the test `TestSecondRaiseWithNoLiveObligation` in the proof plan). It is
decided in the addendum at the foot of
metasystem/records/misc/breach-design-critique-r3.md: INHERIT. When no
obligation is live at the raise, `rebindClaimKeepEpisode` carries the prior
claim binding's `episodeObligationRevision` forward unchanged; it writes the
live obligation's revision only when one is live. Fold it: rewrite the rule
sentence where the third key is written, resolve the OPEN bullet into a
decided fifth sequence (after the second raise the key is still 5 and the
start stays T0+3h), give the test its single expectation, strike every
"OPEN" marker the fold retires, and run the consistency pass over Fix 1,
the proof plan, the disposition table and the self-grade. Edit in place;
diffBoundary is that one file. Keep it under eight minutes; no reading
beyond the lines named.

Bump the header to revision 5 naming the fold. R-31: no benchmarks.

# Constraints

Wall-clock budget: 8 minutes. Design only; edit nothing but the design file.

# Expected Return

Version-2 implementer JSON; diffBoundary exactly the design file.

# Gap Rule

stop and report a gap; never fill it silently.
