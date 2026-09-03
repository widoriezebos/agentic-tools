Working Mode: build
Orchestrator Identity: m2+main-1788441779-14484-82d6ed (dispatch delegate under goal severity-tiered-rigor)
Date: 2026-09-04

# Goal

Second correction on chain str-build1c (your reviewed tree 115151fe):
one material finding from the re-review (str-build1c-cc2), F-9.

# The defect

Fixtures approve goals with a temporary human word and
`--review-by 2026-09-06`: goal-cli-fixtures.sh (the approve_fixture_goal
helper, lines 250 to 255, and the classification-sweep set-budget) and
dispatch-fixtures.sh line 1095. The relayed-word proof is checked
against the real wall clock and the governance horizon (2026-09-06),
so from 2026-09-07 no review date is accepted and every approving
fixture fails.

# The change

Fixtures must never depend on the wall clock or the governance
horizon. Approve fixture goals through the proof form the fixture
suite already uses for human acts that need enrolled authority (the
same form the goal CLI fixtures use for set-obligation or resume, or
the enrolled-terminal simulation the human-authority fixtures use); if
no such form exists for approve, add the smallest one: a fixture-only
authority proof accepted under the fixture root (never outside it),
and say so in the return. No fixed dates anywhere in a fixture. Run
goal-cli-fixtures.sh and the dispatch-fixtures scenario that approves.

# Gate

As before, plus the two fixture scripts touched. Declare the boundary
as every file that differs from main.

# Constraints

Wall-clock budget: 40 minutes; return before it ends even if something
is red, naming it. Gap rule: stop and report a gap with your proposed
contract written out.
