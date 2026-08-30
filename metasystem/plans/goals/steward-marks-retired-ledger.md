# steward-marks-retired-ledger

- State: queued
- Intent: The steward's ledger progress mark survives the goals cutover: CurrentMarks still hashes the retired plans/goals.md (internal/steward/marks.go:36), so OpidDigest is permanently the no-ledger sentinel and ledger movement is invisible to stall detection (found 2026-08-27 during the actionable-metrics fact sweep)
- Origin: main
- Next step: Appetite: 1h. Re-point the ledger mark at the accepted goals ref tip (refs/metasystem/goals/accepted) or deliberately retire the mark's ledger half — either way the mark's name and meaning agree again; fixture proves ledger-only movement reads as progress (or that the retired half is gone). Coordinate with the steward tick's Observe/decideNow consumers (internal/steward/tick.go:81,100).
- OpenedAt: 2026-08-27T06:27:19Z
- Revision: 2
- Budget: elapsedLimit=1h attemptLimit=3 reservedJobMinutesLimit=15 activeJobLimit=1

History:
- 2026-08-27T06:27:19Z QY008RF33761Q5FDQ52JBKG5JJ-m1-bf243850 open actor=m1+coordinator targets=steward-marks-retired-ledger
- 2026-08-30T11:36:25Z BNP7D32M5DM2ZAZBWQQ1WVD9F9-m2-bc1be9cb set-budget actor=human:wido targets=steward-marks-retired-ledger
Integrity: sha256=879e2c00bd173522a1910d157a024857be8e225aa0a1b792ae534033f763e41e
