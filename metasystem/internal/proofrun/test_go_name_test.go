package proofrun

import "testing"

func TestGoTestNameMatchesGoEntrypoints(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name string
		want bool
	}{
		{"Test", true}, {"TestPresent", true}, {"TestÉPresent", true},
		{"Testhelper", false}, {"TestéHelper", false}, {"TestMain", false},
		{"Fuzz", true}, {"FuzzPresent", true}, {"FuzzÉPresent", true}, {"FuzzMain", true},
		{"Fuzzhelper", false}, {"FuzzéHelper", false}, {"Helper", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := goTestName(tc.name); got != tc.want {
				t.Fatalf("goTestName(%q) = %t, want %t", tc.name, got, tc.want)
			}
		})
	}
}
