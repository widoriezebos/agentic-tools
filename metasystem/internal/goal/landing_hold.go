package goal

const LandTrunkRedCode = "GOAL_LAND_TRUNK_RED"

// LandingIncident finds the open failure on main that holds this goal.
// A goal fixing any open incident may land while the others remain open.
func LandingIncident(entries []TrunkRedEntry, id string) (TrunkRedEntry, bool) {
	var red TrunkRedEntry
	found := false
	for _, entry := range entries {
		if entry.Closed != nil || entry.EntryClass() != TrunkRedClassTrunkRed {
			continue
		}
		if id != "" && entry.FixGoal == id {
			return TrunkRedEntry{}, false
		}
		if !found {
			red, found = entry, true
		}
	}
	return red, found
}
