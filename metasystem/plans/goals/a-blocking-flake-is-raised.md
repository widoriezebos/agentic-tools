# a-blocking-flake-is-raised

- State: queued
- Risk: severity=2 novelty=1 exposure=2 accumulation=1 basis="A flaky test with a fix goal open since 16 October cost five long runs on 2026-11-12 and nothing told the steward it had started to block (design a-unit-costs-what-its-change-costs G1, agreed with Astra round 3)."
- Tier: 2
- Intent: A flaky test that starts to block work is raised to the steward: the flake incident counts the runs it failed (a returned or held landing, a failed publication, a failed check), and at two within 24 hours the lane steward's confirmed-flake row becomes a routed row 'blocking' whose suggested act is the existing ordering act on the incident's fix goal; nothing reorders the frontier by computation and no test result is waived (design a-unit-costs-what-its-change-costs G1).
- Origin: main
- Next step: Build one unit (50, --last) from the design section G1: the count on the incident record, the routed row, its suggested act goal prioritize under the ordering class; tests for one run (no row), two runs in 24 hours (row), two runs further apart (no row).
- OpenedAt: 2026-11-13T09:16:57Z
- Revision: 1
- Budget: elapsedLimit=4h attemptLimit=6 reservedJobMinutesLimit=720 activeJobLimit=1 reviewRoundLimit=20
- BudgetExceptions: 0

History:
- 2026-11-13T09:16:57Z H7V2EWVX62CHKAN30RX9NDW51K-m1e-16ea023a open actor=human:Wido targets=a-blocking-flake-is-raised
Integrity: sha256=a8a442eed8f3f974ba9b1c4ecf18bf0eee3f55b8beeba533db6d95dc6d1a726e
