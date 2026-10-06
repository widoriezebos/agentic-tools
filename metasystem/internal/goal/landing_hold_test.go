package goal

import "testing"

func TestLandingIncidentAllowsAnyOpenFixGoal(t *testing.T) {
	t.Parallel()
	entries := []TrunkRedEntry{
		{ID: "first", Class: TrunkRedClassTrunkRed},
		{ID: "fix", FixGoal: "repair", Class: TrunkRedClassTrunkRed},
	}
	if entry, held := LandingIncident(entries, "feature"); !held || entry.ID != "first" {
		t.Fatalf("feature must name the first open incident: %+v %v", entry, held)
	}
	if _, held := LandingIncident(entries, "repair"); held {
		t.Fatal("the fix goal was held by another incident")
	}
	entries[1].Closed = &TrunkRedClosure{}
	if _, held := LandingIncident(entries, "repair"); !held {
		t.Fatal("a closed incident's fix goal bypassed the open incident")
	}
}
