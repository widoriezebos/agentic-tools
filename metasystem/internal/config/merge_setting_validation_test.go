package config

import (
	"strings"
	"testing"
)

func TestSettingValueProblemPreservesLoadAndDeadlineChecks(t *testing.T) {
	t.Parallel()
	for _, row := range []struct {
		key, value string
		valid      bool
	}{
		{"host.load-max", "0.5", true},
		{"host.load-max", "8", true},
		{"host.load-max", "0", false},
		{"host.load-max", "-1", false},
		{"host.load-max", "NaN", false},
		{"host.load-max", "+Inf", false},
		{"host.load-max", "bad", false},
		{"proof.deadline", "1", true},
		{"proof.deadline", "153722867", true},
		{"proof.deadline", "153722868", false},
		{"proof.deadline", "0", false},
		{"proof.deadline", "-1", false},
		{"proof.deadline", "0.5", false},
		{"proof.deadline", "bad", false},
	} {
		t.Run(row.key+"/"+row.value, func(t *testing.T) {
			t.Parallel()
			err := SettingValueProblem(row.key, row.value)
			if (err == nil) != row.valid {
				t.Fatalf("%s=%s: %v; valid=%v", row.key, row.value, err, row.valid)
			}
			if err != nil && !strings.Contains(err.Error(), row.key) {
				t.Fatalf("refusal does not name its setting: %v", err)
			}
		})
	}
}
