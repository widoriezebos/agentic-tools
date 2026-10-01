package pattern

// Signals are what the readers fill once per cycle, one reader per record
// kind. A reader that fails marks its kind unreadable.
type Signals struct {
	// Trunk is origin's main as the private ref holds it.
	Trunk TrunkSignal
}
