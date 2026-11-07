# the-census-ignores-its-own-report

- State: queued
- Risk: severity=2 novelty=1 exposure=2 accumulation=1 basis="an engine defect in one owner, root cause read in a recorded finding (556, 557); the unit check and the lane proof gate it"
- Tier: 2
- Intent: The disk use census does not read its own disk.json gap list as an ownership record: a bare pid without a start is no record, so a process proved unrelated resolves and sweeps are not held by the census own output (finding 557; internal/steward/disk_role.go:726-730, internal/diskstore/ours.go:49,85-97, census.go:150-151)
- Origin: human
- DesignGate: off
- Next step: Build without a design from finding 557: leave disk.json out of recordedProcesses (or ignore a bare pid without a start in scan); a test with a gap list as the only record resolves the process; owned processes with pidStartedAt stay protected; about 20 lines
- OpenedAt: 2026-11-07T15:03:04Z
- Revision: 2
- Budget: elapsedLimit=4h attemptLimit=6 reservedJobMinutesLimit=720 activeJobLimit=1 reviewRoundLimit=20
- BudgetExceptions: 0

History:
- 2026-11-07T15:03:04Z W78DD14D6696ECX8QVF2BNG4JD-m1e-5a3e856b open actor=human:Wido targets=the-census-ignores-its-own-report
- 2026-11-07T15:03:14Z BHC4954VPF0BXWV7Q6EHP20YW0-m1e-5a3e856b edit actor=human:Wido targets=the-census-ignores-its-own-report reason=Allowed: build-without-design why=Allowed: build-without-design. One unit from a recorded finding (557) whose root cause is read; deletion stays fail-closed for owned processes impact=This lets goal the-census-ignores-its-own-report be built and landed without an accepted, reviewed design. The risk: nobody checks the approach before code is written, so a wrong one is found only in code review or after it lands. It holds for every later build and landing of the-census-ignores-its-own-report until withdrawn. To undo: metasystem goal disallow the-census-ignores-its-own-report build-without-design
Integrity: sha256=33f4a52942e3abdb216e5df71ed903961cde1b1795eafcc2c0783d14f70bd737
