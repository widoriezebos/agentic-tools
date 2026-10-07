# Task: split and accept plans/designs/lane-drain-and-fresh-claims.md after critique round 3

Working Mode: Design
Round 3 found 1 material (down from 3): it repeats the seat-ladder class in Decision 4 / U4, so under the plan's design stop rule Decision 4 and U4 leave this goal, and U5 (claim preparation, which uses U4's adoption) leaves with it, to a follow-up goal person-claims. U1a, U1b and U3 were clean in round 3 and are accepted. Never open any metasystem.conf.local. Edit only this page and create the one brief below.

A. Remove Decisions 4 and 5 and units U4 and U5 and everything that depends on them; name person-claims under "Not in this goal". U1a, U1b, U3 remain; re-estimate.
B. Write plans/person-claims-design-brief.md: the removed Decisions 4 and 5 text, round-3 finding 1 with its change (key the seat-ladder exclusion on executability, not epoch: in SeatWorldFrom seat_ladder.go:75-77 a seat-lineage claim with no current approval or budget is held but not due, Wait "awaits a person's approval"; the same in holdsUnderSeatLineage seat_start.go:466; say whether it keeps the seat's one-working-claim notification; test: adopt an unapproved reservation, end the seat, steward tick, no launcher call; mutation: exclude only epoch 0), the round-3 note on PlanSeat starting a seat for unrelated ready work, the round-2 notes on lineage and areas, and the five design questions.
C. U1a: one sentence that an actual AdvanceDrain write error does not hold the keeper (a failed held write leaves the drain draining), asserted in the injected-failure case.
D. Set Status: accepted and add under the header lines: "- Critique: closed at round 3; U1a, U1b, U3 clean (rounds 6, 3, 1); Decision 2 split to ledger-reads-are-fresh and Decisions 4-5 to person-claims under the plan's design stop rule".
Return the units table.
