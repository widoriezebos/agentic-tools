package crossing

// fixtureRuns names a fixture directory stateRoot; in a test file that name
// carries no state root.
func fixtureRuns() string {
	stateRoot := "/tmp/fixture"
	return runDir(stateRoot)
}
