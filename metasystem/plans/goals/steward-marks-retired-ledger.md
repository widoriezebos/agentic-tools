# steward-marks-retired-ledger

- State: claimed
- Intent: The steward's ledger progress mark survives the goals cutover: CurrentMarks still hashes the retired plans/goals.md (internal/steward/marks.go:36), so OpidDigest is permanently the no-ledger sentinel and ledger movement is invisible to stall detection (found 2026-08-27 during the actionable-metrics fact sweep)
- Origin: main
- Next step: Appetite: 1h. Re-point the ledger mark at the accepted goals ref tip (refs/metasystem/goals/accepted) or deliberately retire the mark's ledger half — either way the mark's name and meaning agree again; fixture proves ledger-only movement reads as progress (or that the retired half is gone). Coordinate with the steward tick's Observe/decideNow consumers (internal/steward/tick.go:81,100).
- OpenedAt: 2026-08-27T06:27:19Z
- Revision: 3
- Budget: elapsedLimit=1h attemptLimit=3 reservedJobMinutesLimit=15 activeJobLimit=1
- Claimed: machine=m2 lineage=mac-coordinator at=2026-08-30T11:36:40Z revision=3
- StopCapability: generation=3 revision=3 machine=m2 claimEpoch=1 fenceEpoch=0

History:
- 2026-08-27T06:27:19Z QY008RF33761Q5FDQ52JBKG5JJ-m1-bf243850 open actor=m1+coordinator targets=steward-marks-retired-ledger
- 2026-08-30T11:36:25Z BNP7D32M5DM2ZAZBWQQ1WVD9F9-m2-bc1be9cb set-budget actor=human:wido targets=steward-marks-retired-ledger
- 2026-08-30T11:36:40Z 7D8BYXGNFVR1XH1C57G3SZ8QRH-m2-bc1be9cb claim actor=m2+mac-coordinator targets=steward-marks-retired-ledger
Integrity: sha256=419fe4700ae4ffcacf820ffb89462f8c811fd17b7de89761ef7181aa608821bc
