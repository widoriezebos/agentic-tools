# a-start-that-never-launched-is-collected

- State: queued
- Risk: severity=2 novelty=1 exposure=2 accumulation=1 basis="an engine defect in one owner, found by the 2026-11-07 fixes analysis and its two Astra rounds; the unit check and the lane proof gate it"
- Tier: 2
- Intent: A step saved as starting whose launch never existed is offered to its seat like an ended one (finding 500; read_sequence.go:72-76, seat_idle.go:120-127, person_wait.go:112-160)
- Origin: human
- DesignGate: off
- Next step: Build without a design from N3 and section 7 of /Users/wido/agentic-tools-evidence/lane-test-20261009/designs/analysis-fixes-2026-11-07.md: one never-launched predicate with the same clock and custody evidence for discovery and reconciliation; the question stays until the start advances or is cancelled; test successive ticks; about 40 lines
- OpenedAt: 2026-11-07T09:50:38Z
- Revision: 2
- Budget: elapsedLimit=4h attemptLimit=6 reservedJobMinutesLimit=720 activeJobLimit=1 reviewRoundLimit=20
- BudgetExceptions: 0

History:
- 2026-11-07T09:50:38Z TF94QQDXGFHX5CN2RN7T4HT3SG-m1e-1dae276b open actor=human:Wido targets=a-start-that-never-launched-is-collected
- 2026-11-07T09:50:48Z YDS3RP9RD5TPZSZDZCGTZ9FY4M-m1e-1dae276b edit actor=human:Wido targets=a-start-that-never-launched-is-collected reason=Allowed: build-without-design why=Allowed: build-without-design. One unit from a recorded finding whose root cause is read in the fixes analysis revision 3 and critiqued twice by Astra impact=This lets goal a-start-that-never-launched-is-collected be built and landed without an accepted, reviewed design. The risk: nobody checks the approach before code is written, so a wrong one is found only in code review or after it lands. It holds for every later build and landing of a-start-that-never-launched-is-collected until withdrawn. To undo: metasystem goal disallow a-start-that-never-launched-is-collected build-without-design
Integrity: sha256=d93cdd444427a753b71b24777fcb3bf2d331facdcb9d84d5e33f818393be25e7
