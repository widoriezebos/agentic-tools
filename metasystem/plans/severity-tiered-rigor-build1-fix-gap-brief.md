Working Mode: build
Orchestrator Identity: m2+main-1788441779-14484-82d6ed (dispatch delegate under goal severity-tiered-rigor)
Date: 2026-09-03

# Answer to the gap of the correction round on chain str-build1c

allow. A legacy NormApproval record without reviewRounds, on a goal
whose Budget is also legacy (no reviewRoundLimit), parses its
reviewRounds as the same inferred tier-box review-round value used for
that Budget (tier three while the goal is tierless); a legacy marker
is retained until the next write, which renders the explicit value.
Where only one of the two records is legacy, the explicit one wins and
the legacy one is inferred from the same box; a mismatch between an
explicit Budget round member and an explicit NormApproval reviewRounds
is a validation problem as it is today. One focused test for the
both-legacy case and one for the mixed case.

Then finish the correction round of
metasystem/plans/severity-tiered-rigor-build1-fix-brief.md: run the
complete gate (the module tests of the continuation brief, goal CLI
fixtures, and every fixture script you touched: dispatch, supervision,
adopt, channel) and return with the boundary. Wall-clock budget: 60
minutes from this round's start; return before it ends even if
something is red, naming it.
