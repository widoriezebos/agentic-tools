package testrun

import "fmt"

// The refusals in this file are machine protocol as well as text: the
// landing owner reads their leading code from the planning child's output
// (internal/landing/batchowner laneHold) and holds the batch on it. Their
// code stays first and byte-identical, so this file is the one place in
// the package where a code leads a line a person may read.

// laneEngineTooOld refuses a lane-charged plan on a pinned engine that
// predates --lane: the batch holds until the checkout's engine moves on.
func laneEngineTooOld(engine string) error {
	return fmt.Errorf("LANE_ENGINE_TOO_OLD: the pinned engine %s cannot charge tests to the landing lane, so the batch holds; "+
		"once this checkout runs a newer engine, run: metasystem landing restart", engine)
}
