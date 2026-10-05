package readsubject

import "testing"

func TestSupersededPlaceholderRegister(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name       string
		round      any
		resolution string
		wantErr    bool
	}{
		{"retry", 1, "superseded by round 2", false},
		{"real finding", nil, "superseded by round 2", true},
		{"same round", 2, "superseded by round 2", true},
		{"malformed marker", "1", "superseded by round 2", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			entry := modernRegisterEntry("resolved", tc.resolution)
			entry["decisionOpid"] = ""
			if tc.round != nil {
				entry["placeholderRound"] = tc.round
			}
			clean, err := CleanRegister([]any{entry})
			if (err != nil) != tc.wantErr || !tc.wantErr && !clean {
				t.Fatalf("clean register: %v, %v", clean, err)
			}
			landable, risks, err := LandableRegister([]any{entry})
			if (err != nil) != tc.wantErr || !tc.wantErr && !landable || len(risks) != 0 {
				t.Fatalf("landable register: %v, %v, %v", landable, risks, err)
			}
		})
	}
}
