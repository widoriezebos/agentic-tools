# a-retained-read-brief-keeps-its-context

- State: queued
- Risk: severity=2 novelty=1 exposure=2 accumulation=1 basis="an engine defect in one owner, found by the 2026-11-07 fixes analysis and its two Astra rounds; the unit check and the lane proof gate it"
- Tier: 2
- Intent: A correction round read keeps its read-context.md beside the retained brief (finding 546, decision 206; unit_step_failure.go:54-76, pack.go:49)
- Origin: human
- DesignGate: off
- Next step: Build without a design from N5 of /Users/wido/agentic-tools-evidence/lane-test-20261009/designs/analysis-fixes-2026-11-07.md: copy read-context.md with the retained brief; test a correction-round read passes the pack check; mutation skips the copy; about 10 lines
- OpenedAt: 2026-11-07T09:51:52Z
- Revision: 2
- Budget: elapsedLimit=4h attemptLimit=6 reservedJobMinutesLimit=720 activeJobLimit=1 reviewRoundLimit=20
- BudgetExceptions: 0

History:
- 2026-11-07T09:51:52Z 2MB0AZSAR03CF4H0KMTWWYASFE-m1e-1dae276b open actor=human:Wido targets=a-retained-read-brief-keeps-its-context
- 2026-11-07T09:52:02Z FQWV3VCV31KHX7V1PEZN3BC0F7-m1e-1dae276b edit actor=human:Wido targets=a-retained-read-brief-keeps-its-context reason=Allowed: build-without-design why=Allowed: build-without-design. One unit from a recorded finding whose root cause is read in the fixes analysis revision 3 and critiqued twice by Astra impact=This lets goal a-retained-read-brief-keeps-its-context be built and landed without an accepted, reviewed design. The risk: nobody checks the approach before code is written, so a wrong one is found only in code review or after it lands. It holds for every later build and landing of a-retained-read-brief-keeps-its-context until withdrawn. To undo: metasystem goal disallow a-retained-read-brief-keeps-its-context build-without-design
Integrity: sha256=5e799ed3fb2c62e696e94c2ade62a3171d7c5373abda3c4525239980b6806ad8
