# the-census-ignores-its-own-report

- State: queued
- Risk: severity=2 novelty=1 exposure=2 accumulation=1 basis="an engine defect in one owner, root cause read in a recorded finding (556, 557); the unit check and the lane proof gate it"
- Tier: 2
- Intent: The disk use census does not read its own disk.json gap list as an ownership record: a bare pid without a start is no record, so a process proved unrelated resolves and sweeps are not held by the census own output (finding 557; internal/steward/disk_role.go:726-730, internal/diskstore/ours.go:49,85-97, census.go:150-151)
- Origin: human
- Next step: Build without a design from finding 557: leave disk.json out of recordedProcesses (or ignore a bare pid without a start in scan); a test with a gap list as the only record resolves the process; owned processes with pidStartedAt stay protected; about 20 lines
- OpenedAt: 2026-11-07T15:03:04Z
- Revision: 1
- Budget: elapsedLimit=4h attemptLimit=6 reservedJobMinutesLimit=720 activeJobLimit=1 reviewRoundLimit=20
- BudgetExceptions: 0

History:
- 2026-11-07T15:03:04Z W78DD14D6696ECX8QVF2BNG4JD-m1e-5a3e856b open actor=human:Wido targets=the-census-ignores-its-own-report
Integrity: sha256=aba4f675140943256aefabc8384a154313cedca28ce24bd8fb69384a6dad062c
