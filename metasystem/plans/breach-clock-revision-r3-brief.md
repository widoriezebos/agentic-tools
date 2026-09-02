Working Mode: design
Orchestrator Identity: m0b+main-1788250419-3170380-8a1fb3 (dispatch delegate under goal breach-clock-and-budget-honesty)
Date: 2026-09-02

# Goal

Revision 3 of metasystem/plans/breach-clock-and-budget-honesty-design.md
(revision 2 landed, in your worktree). Sol's round-2 register is
metasystem/records/misc/breach-design-critique-r2.md: THREE material
findings, BCD-R2-001, BCD-R2-002, BCD-R2-003, each with the orchestrator's
decision at the bottom of that file. Fold all three, mark each closure with
its id, extend the disposition table (design lines 905-916) with a round-2
block, refresh the self-grade, and stop. Edit in place; diffBoundary is that
one file. Keep it under fifteen minutes; verify each closure against the
tree lines named here, no wider reading.

# The folds

1. BCD-R2-001 (Fix 1, "Discharge proofs bind to the episode", lines 263-307).
   HOLD TODAY'S GOVERNANCE. Rewrite the eligibility rule so the episode axis
   replaces only the claim-revision filter: a proof counts when
   `goalId == file.Id`, `episodeRevision <= goalRevision <= Claimed.Revision`,
   `!consumedAt.Before(episodeAt)`, AND, whenever `file.Obligation != nil`,
   `obligationRevision == file.Obligation.Revision` exactly as
   metasystem/internal/dispatch/budget.go lines 133-149 demand today. When
   `file.Obligation == nil` after a raise (the rebind clears it, verbs.go
   lines 122-124) the episode-scoped proof counts and the durable green match
   uses the proof's own pair, as revision 2 already says. Consequences to
   state: a raise leaves the start where it was the moment before (closes
   BCD-R1-003 unchanged); a human `set-obligation` keeps its shipped meaning
   (a fresh obligation supersedes earlier discharges and the start returns to
   the episode origin), so no governance seam moves. Strike the "Consequence,
   stated because it changes a behavior" bullet (lines 294-303) and the
   self-grade's risk entry for it; replace them with one sentence naming the
   open question for Wido (should a later set-obligation inherit a discharge
   consumed inside the same episode) as NOT built by this goal. Rename or
   retarget `TestSetObligationDoesNotRewindADischargedClock` to pin today's
   behavior: set-obligation after a discharge DOES return the start to the
   episode origin, and a raise after a discharge does not move it.
2. BCD-R2-002 (Fix 2 proof plan, lines 734-790). Add a complete day-token
   inventory, one row per site, each classified as exactly one of: (a)
   legacy-reader coverage (a stored 1d read by the new binary, unchanged),
   (b) converted to hours (the row's intent survives with 8h or 216h), (c)
   explicit day-refusal coverage (the row now asserts the new refusal
   wording). The sites: metasystem/scripts/agents/goal-cli-fixtures.sh lines
   387 and 416 (typed 8h expected stored as 1d — becomes (b), asserting 8h
   stored verbatim), lines 448 and 457 (fresh 1d expecting the goal-norm
   refusal — decide (b) or (c) per row so the norm coverage is not weakened:
   keep one norm row in hours and add one (c) row), metasystem/scripts/agents/dispatch-fixtures.sh
   line 1092 (1d then a confirmed claim — (b)), metasystem/internal/goal/project_test.go
   lines 151-160 (formatter call — retire with the formatter or convert to
   the verbatim token, say which). Then re-run your search of
   scripts, docs, fixtures and _test.go files for any remaining d token and
   list every hit in the table; the table is complete or the revision says
   where it is not.
3. BCD-R2-003 (Fix 2, "Rollout and rollback", lines 406-432). Replace the
   two-writer statement with a table of every old-binary writer that passes a
   supplied tuple through the normalizing constructor in
   metasystem/cmd/metasystem/goalsync_mutations.go: `budgetTuple` (lines
   166-180), open-claim (217-224), resume (367-413), claim with a supplied
   budget (653-667), set-budget (669-675), plus journal recovery. For each:
   what an old binary writes after rollback (a new 216h becomes 27d), what it
   enforces (216 hours, unchanged), how the new binary reads it back (legacy
   reader, eight-hour days, enforcement identical), and how it is cured (the
   next write by a new binary). State plainly that the residue is
   display-only, per write, on any goal, until the fleet stops running the
   old binary; no rollback wall is built (a separate mechanism, out of this
   goal's scope) and say so.

Bump the header to revision 3 naming the round. R-31: no benchmarks.

# Constraints

Wall-clock budget: 15 minutes. Design only; edit nothing but the design
file.

# Expected Return

Version-2 implementer JSON; diffBoundary exactly the design file.

# Gap Rule

stop and report a gap; never fill it silently.
