package lane

import "testing"

func TestLaneRemedyTranslatesOnlyKnownPersonMechanisms(t *testing.T) {
	t.Parallel()
	for command, want := range map[string]string{
		"metasystem landing run --goals a,b":                                   "metasystem landing resume",
		"metasystem landing restart":                                           "metasystem landing resume",
		"metasystem landing return g --cause other --reason TEXT":              "metasystem landing give-back g --reason TEXT",
		"metasystem work land g --exception GOAL_LAND_TRUNK_RED --reason TEXT": "metasystem landing land-anyway g --reason TEXT",
	} {
		if got := Remedy(command); got != want {
			t.Errorf("%q: got %q, want %q", command, got, want)
		}
	}
	for _, command := range []string{"", "unknown", "metasystem landing run", "metasystem landing prove --classify 20261012T123456 --gate", "metasystem landing prove --trunk", "metasystem landing push", "metasystem landing resolve g", "metasystem landing set PATH", "metasystem landing unset", "metasystem landing drain", "metasystem system enroll --name Wido", "metasystem system start --repo fixture", "metasystem helm return --repo fixture", "git config metasystem.machine fixture", "metasystem work land g --exception OTHER", "metasystem landing give-back g --reason chosen"} {
		if got := Remedy(command); got != command {
			t.Errorf("changed prerequisite or executable act %q to %q", command, got)
		}
	}
}
