package main

import "testing"

// The lane's gate passes LANDING_ONLY="" when it is not replaying: that must
// select by impact, not run an empty replay that checks nothing.
func TestTestImpactEmptyLandingOnlyIsNotAReplay(t *testing.T) {
	for _, value := range []string{"", "   "} {
		t.Setenv("LANDING_ONLY", value)
		if _, replay := landingOnly(); replay {
			t.Fatalf("LANDING_ONLY=%q was taken as a replay", value)
		}
	}
	t.Setenv("LANDING_ONLY", "metasystem/internal/a")
	if only, replay := landingOnly(); !replay || only != "metasystem/internal/a" {
		t.Fatalf("a named selection = %q replay=%t", only, replay)
	}
}
