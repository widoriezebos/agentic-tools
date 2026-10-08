# Task: fold critique round 4 into plans/designs/review-chain-stops-and-records.md and accept it

Working Mode: Design
Round 4 (the last) found 1 material with a concrete change (rounds 9, 4, 1, 1). Under the plan's design stop rule it folds as an acceptance item (a line "Acceptance item (round 4): ..." under unit-stop) and the page is accepted. Set Status: accepted; add under the header lines "- Critique: closed at round 4 on 1 material finding folded as an acceptance item (rounds 9, 4, 1, 1; Decision 2's drop and Decision 7 split to review-drops-and-design-convergence)". Never open any metasystem.conf.local. Edit only this page.

Finding: one ask per stop cannot be closed by goal accept-risk, which takes exactly one --finding (cmd/metasystem/intent_planning.go:1494-1520), when two or more findings are unresolved. Key the unit-round asks by (loop, subject, attempt, finding), one command each; an unrequired unit's ask lists its acceptable acts with the closure rule (the ask closes when any listed act's record names that finding, or the unit closes). The unit-stop test stops a unit with two unresolved findings, runs one accept-risk, and asserts that only that finding's ask closes (mutation: close the whole stop's ask on one accept-risk).
Return the final units table.
