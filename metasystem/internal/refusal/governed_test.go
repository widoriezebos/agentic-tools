package refusal

import "testing"

func TestDesignGateGovernedBy(t *testing.T) {
	t.Parallel()
	want := map[string]string{
		"BUILD_DESIGN_NOT_ACCEPTED":   "R-146-m1k",
		"LANDING_DESIGN_NOT_STANDING": "R-146-m1k",
		"GOAL_BRANCH_NOT_HOLDER":      "R-147-m1k",
	}
	if len(GovernedBy) != len(want) {
		t.Fatalf("governed gates = %v, want %v", GovernedBy, want)
	}
	for code, id := range want {
		if GovernedBy[code] != id {
			t.Errorf("%s is governed by %q, want %s", code, GovernedBy[code], id)
		}
	}
	registered := make(map[string]bool, len(Rows))
	for _, row := range Rows {
		registered[row.Code] = true
	}
	for code := range GovernedBy {
		if !registered[code] {
			t.Errorf("governed gate %s has no refusal register entry", code)
		}
	}
}
