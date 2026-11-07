# a-boundary-waits-only-on-its-own-questions

- State: queued
- Risk: severity=2 novelty=1 exposure=2 accumulation=1 basis="an engine defect in one owner, found by the 2026-11-07 fixes analysis and its two Astra rounds; the unit check and the lane proof gate it"
- Tier: 2
- Intent: A seat unit boundary waits only on questions about the goals it holds (finding 538: ObserveUnitBoundary returns early on any open channel question, internal/steward/unit_boundary.go:77-83)
- Origin: human
- Next step: Build without a design from N1 of /Users/wido/agentic-tools-evidence/lane-test-20261009/designs/analysis-fixes-2026-11-07.md: filter the question walk to held goals (work.Claimed, work.Landing) and machine questions whose subject is a held goal; test both cases; about 15 lines
- OpenedAt: 2026-11-07T09:49:10Z
- Revision: 1
- Budget: elapsedLimit=4h attemptLimit=6 reservedJobMinutesLimit=720 activeJobLimit=1 reviewRoundLimit=20
- BudgetExceptions: 0

History:
- 2026-11-07T09:49:10Z VY1D9MEXXK0T71JXK98RN18M0A-m1e-1dae276b open actor=human:Wido targets=a-boundary-waits-only-on-its-own-questions
Integrity: sha256=3e092dba18182b43a8a182ed582694cc7675695a9334016ae88892e1888cfe44
