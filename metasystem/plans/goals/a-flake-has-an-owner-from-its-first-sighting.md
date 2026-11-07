# a-flake-has-an-owner-from-its-first-sighting

- State: queued
- Priority: 1
- Sequence: 126
- Risk: severity=2 novelty=1 exposure=2 accumulation=1 basis="an engine defect in one owner, found by the 2026-11-07 fixes analysis and its two Astra rounds; the unit check and the lane proof gate it"
- Tier: 2
- Intent: A flake gets an owner at its first sighting, and an owner whose repair failed again is reopened (steward_flake.go:42, flakefix.go:43-49)
- Origin: human
- DesignGate: off
- Next step: Build without a design from N7 and section 7 of /Users/wido/agentic-tools-evidence/lane-test-20261009/designs/analysis-fixes-2026-11-07.md: three states (repair pending, landed awaiting closure passes, recurring after repair); only the last reopens; abandoned owners keep the person route. Approve only after the incident dispositions of row 33 are done
- OpenedAt: 2026-11-07T09:53:08Z
- Revision: 3
- Budget: elapsedLimit=4h attemptLimit=6 reservedJobMinutesLimit=720 activeJobLimit=1 reviewRoundLimit=20
- BudgetExceptions: 0

History:
- 2026-11-07T09:53:08Z 5C28T70NWK63AYYY33RFSA32T1-m1e-1dae276b open actor=human:Wido targets=a-flake-has-an-owner-from-its-first-sighting
- 2026-11-07T09:53:17Z MVH1847W5ZWHJVG8C866V47VRG-m1e-1dae276b edit actor=human:Wido targets=a-flake-has-an-owner-from-its-first-sighting reason=Allowed: build-without-design why=Allowed: build-without-design. One unit from a recorded finding whose root cause is read in the fixes analysis revision 3 and critiqued twice by Astra impact=This lets goal a-flake-has-an-owner-from-its-first-sighting be built and landed without an accepted, reviewed design. The risk: nobody checks the approach before code is written, so a wrong one is found only in code review or after it lands. It holds for every later build and landing of a-flake-has-an-owner-from-its-first-sighting until withdrawn. To undo: metasystem goal disallow a-flake-has-an-owner-from-its-first-sighting build-without-design
- 2026-11-07T09:53:26Z 5TBWB3S26F01FCFJTNMN45GKKF-m1e-1dae276b set-priority actor=human:Wido targets=a-flake-has-an-owner-from-its-first-sighting reason=priority-order subject=a-flake-has-an-owner-from-its-first-sighting from=unranked to=1:126 requested-sequence=append
Integrity: sha256=967d6ead89583c800c397bbd34454de7f22d5a8699368742fa9adbca5fdcd197
