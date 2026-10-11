package config

import "testing"

func TestLaneWaitHoursHasOneCompiledDefaultAndValidTimerValues(t *testing.T) {
	t.Parallel()
	value, _, err := Get(GetParams{Key: "landing.wait-hours", LookupEnv: func(string) (string, bool) { return "", false }})
	if err != nil || value != "12" {
		t.Fatalf("compiled lane waiting bound: %q %v", value, err)
	}
	for _, invalid := range []string{"0", "-1", "no", "9999999999999999999"} {
		if err := SettingValueProblem("landing.wait-hours", invalid); err == nil {
			t.Fatalf("unusable lane timer accepted: %s", invalid)
		}
	}
	if err := SettingValueProblem("landing.wait-hours", "24"); err != nil {
		t.Fatal(err)
	}
}
