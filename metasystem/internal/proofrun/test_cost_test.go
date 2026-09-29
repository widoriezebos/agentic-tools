package proofrun

import (
	"slices"
	"testing"
)

func TestFixtureScenarioSelectionKeepsOrdinaryAndComparisonSets(t *testing.T) {
	t.Parallel()
	all, err := FixtureScenarios("dispatcher", "all")
	if err != nil || len(all) != 10 || all[0] != "dispatch" || all[9] != "seat-refused" {
		t.Fatalf("ordinary dispatcher scenarios = %v, %v", all, err)
	}
	selected, err := FixtureScenarios("dispatcher", "comparison")
	if err != nil || !slices.Equal(selected, []string{"adapter-selftest", "steward-continuation"}) {
		t.Fatalf("dispatcher comparison scenarios = %v, %v", selected, err)
	}
	if _, err := FixtureScenarios("dispatcher", "partial"); err == nil {
		t.Fatal("unowned partial fixture selection was accepted")
	}
}
