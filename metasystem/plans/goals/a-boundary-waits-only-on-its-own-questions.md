# a-boundary-waits-only-on-its-own-questions

- State: queued
- Risk: severity=2 novelty=1 exposure=2 accumulation=1 basis="an engine defect in one owner, found by the 2026-11-07 fixes analysis and its two Astra rounds; the unit check and the lane proof gate it"
- Tier: 2
- Intent: A seat unit boundary waits only on questions about the goals it holds (finding 538: ObserveUnitBoundary returns early on any open channel question, internal/steward/unit_boundary.go:77-83)
- Origin: human
- DesignGate: off
- Next step: Build without a design from N1 of /Users/wido/agentic-tools-evidence/lane-test-20261009/designs/analysis-fixes-2026-11-07.md: filter the question walk to held goals (work.Claimed, work.Landing) and machine questions whose subject is a held goal; test both cases; about 15 lines
- OpenedAt: 2026-11-07T09:49:10Z
- Revision: 2
- Budget: elapsedLimit=4h attemptLimit=6 reservedJobMinutesLimit=720 activeJobLimit=1 reviewRoundLimit=20
- BudgetExceptions: 0

History:
- 2026-11-07T09:49:10Z VY1D9MEXXK0T71JXK98RN18M0A-m1e-1dae276b open actor=human:Wido targets=a-boundary-waits-only-on-its-own-questions
- 2026-11-07T09:49:21Z EXEVQWSHTS5TDXM4PYNS7BS3MQ-m1e-1dae276b edit actor=human:Wido targets=a-boundary-waits-only-on-its-own-questions reason=Allowed: build-without-design why=Allowed: build-without-design. One unit from a recorded finding whose root cause is read in the fixes analysis revision 3 and critiqued twice by Astra impact=This lets goal a-boundary-waits-only-on-its-own-questions be built and landed without an accepted, reviewed design. The risk: nobody checks the approach before code is written, so a wrong one is found only in code review or after it lands. It holds for every later build and landing of a-boundary-waits-only-on-its-own-questions until withdrawn. To undo: metasystem goal disallow a-boundary-waits-only-on-its-own-questions build-without-design
Integrity: sha256=18c3df24a733b3ce7ad2a3beeae7884c06c190eca47ba917ec1850845833002c
