# steward-marks-retired-ledger

- State: queued
- Intent: The steward's ledger progress mark survives the goals cutover: CurrentMarks still hashes the retired plans/goals.md (internal/steward/marks.go:36), so OpidDigest is permanently the no-ledger sentinel and ledger movement is invisible to stall detection (found 2026-08-27 during the actionable-metrics fact sweep)
- Origin: main
- Next step: Appetite: 1h. Re-point the ledger mark at the accepted goals ref tip (refs/metasystem/goals/accepted) or deliberately retire the mark's ledger half — either way the mark's name and meaning agree again; fixture proves ledger-only movement reads as progress (or that the retired half is gone). Coordinate with the steward tick's Observe/decideNow consumers (internal/steward/tick.go:81,100).
- OpenedAt: 2026-08-27T06:27:19Z
- Revision: 1

History:
- 2026-08-27T06:27:19Z QY008RF33761Q5FDQ52JBKG5JJ-m1-bf243850 open actor=m1+coordinator targets=steward-marks-retired-ledger
Integrity: sha256=6e2e870f0db88479e98429d2b8f02b12d187c772f684aff1fb4641872245468b
