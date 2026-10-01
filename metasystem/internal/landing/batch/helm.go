package batch

// HelmHeldSeat is the one decision whether a batch is held whole because a
// seat that joined it is at the helm: every reader of the lane says so from
// it. It names the first such
// unit's seat root; active nil holds nothing.
func HelmHeldSeat(record Record, active func(seatRoot string) bool) (string, bool) {
	for _, unit := range record.Units {
		if active != nil && unit.SeatRoot != "" && active(unit.SeatRoot) {
			return unit.SeatRoot, true
		}
	}
	return "", false
}
