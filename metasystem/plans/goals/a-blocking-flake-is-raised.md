# a-blocking-flake-is-raised

- State: approved
- Risk: severity=2 novelty=1 exposure=2 accumulation=1 basis="A flaky test with a fix goal open since 16 October cost five long runs on 2026-11-12 and nothing told the steward it had started to block (design a-unit-costs-what-its-change-costs G1, agreed with Astra round 3)."
- Tier: 2
- Intent: A flaky test that starts to block work is raised to the steward: the flake incident counts the runs it failed (a returned or held landing, a failed publication, a failed check), and at two within 24 hours the lane steward's confirmed-flake row becomes a routed row 'blocking' whose suggested act is the existing ordering act on the incident's fix goal; nothing reorders the frontier by computation and no test result is waived (design a-unit-costs-what-its-change-costs G1).
- Origin: main
- Next step: Build one unit (50, --last) from the design section G1: the count on the incident record, the routed row, its suggested act goal prioritize under the ordering class; tests for one run (no row), two runs in 24 hours (row), two runs further apart (no row).
- OpenedAt: 2026-11-13T09:16:57Z
- Revision: 3
- Pinned: ui
- Budget: elapsedLimit=2d attemptLimit=12 reservedJobMinutesLimit=2400 activeJobLimit=1 reviewRoundLimit=20
- BudgetExceptions: 0
- NormApproval: approvedRef=1JJQEWQ13DFFC9M8E7F5D0826X-m1e-16ea023a minutes=2400 reviewRounds=20 goalRevision=1
- Approved: by=human:Wido at=2026-11-13T09:17:16Z revision=2 opid=1JJQEWQ13DFFC9M8E7F5D0826X-m1e-16ea023a authority=proven digest=7bf988bf3fffaeb576391bc0e5463324b98acb4ba555514a2d71c6261fca8048 episode=2

History:
- 2026-11-13T09:16:57Z H7V2EWVX62CHKAN30RX9NDW51K-m1e-16ea023a open actor=human:Wido targets=a-blocking-flake-is-raised
- 2026-11-13T09:17:16Z 1JJQEWQ13DFFC9M8E7F5D0826X-m1e-16ea023a approve actor=human:Wido targets=a-blocking-flake-is-raised
- 2026-11-13T09:17:37Z VD55RBJX3G75CPXKEXRPHJCYX8-m1e-16ea023a set-pin actor=human:Wido targets=a-blocking-flake-is-raised
Integrity: sha256=cb4a3da9bd8d2082ffac5ade0e8453c14900b47a2f5cda668004a50a849ecaf3
