package identity

// The fake-identity fixture table (METASYSTEM_FAKE_PROCESS_IDENTITY_FILE)
// has ONE reader (review lease-census-3): five packages parsed it with
// private structs and two different start-time spellings, and the shell
// fixtures wrote both keys into every entry to satisfy them all.

// FixtureEntry is one pid's recorded identity in the fixture table. The
// Has* flags distinguish an absent field from a zero value.
type FixtureEntry struct {
	StartedAt    int64
	HasStartedAt bool
	Command      string
	HasCommand   bool
	Pgid         int64
	HasPgid      bool
}

// FixtureProbe is the neutral seam fixture-capable identity decisions
// accept (agnosticism phase B1): internal/fixtureauth implements it
// behind root-checked authorization; a nil probe refuses every
// fixture read. The file reader itself moved to fixtureauth — this
// foundation package no longer touches the environment.
type FixtureProbe interface {
	FixtureEntry(pid int64) (FixtureEntry, bool)
}

// probeEntry is the nil-safe read every consumer in this package uses.
func probeEntry(probe FixtureProbe, pid int64) (FixtureEntry, bool) {
	if probe == nil {
		return FixtureEntry{}, false
	}
	return probe.FixtureEntry(pid)
}
