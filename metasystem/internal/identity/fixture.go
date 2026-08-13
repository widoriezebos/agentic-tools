package identity

import (
	"encoding/json"
	"os"
	"strconv"
)

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

// FixtureEntryFor reads pid's entry from the fixture table. ok is false
// when the env var is unset, the table is unreadable, or the pid has no
// entry. pidStartedAt is the canonical start-time spelling (decision D14);
// the legacy "started" spelling is still read during the transition and
// drops once no fixture writer emits it.
func FixtureEntryFor(pid int64) (FixtureEntry, bool) {
	path := os.Getenv("METASYSTEM_FAKE_PROCESS_IDENTITY_FILE")
	if path == "" {
		return FixtureEntry{}, false
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return FixtureEntry{}, false
	}
	var table map[string]struct {
		PidStartedAt *int64  `json:"pidStartedAt"`
		Started      *int64  `json:"started"`
		Command      *string `json:"command"`
		Pgid         *int64  `json:"pgid"`
	}
	if json.Unmarshal(data, &table) != nil {
		return FixtureEntry{}, false
	}
	raw, present := table[strconv.FormatInt(pid, 10)]
	if !present {
		return FixtureEntry{}, false
	}
	entry := FixtureEntry{}
	if raw.PidStartedAt != nil {
		entry.StartedAt, entry.HasStartedAt = *raw.PidStartedAt, true
	} else if raw.Started != nil {
		entry.StartedAt, entry.HasStartedAt = *raw.Started, true
	}
	if raw.Command != nil {
		entry.Command, entry.HasCommand = *raw.Command, true
	}
	if raw.Pgid != nil {
		entry.Pgid, entry.HasPgid = *raw.Pgid, true
	}
	return entry, true
}
