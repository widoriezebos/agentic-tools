package config

import (
	"path/filepath"
	"testing"
)

func TestLandingFixAllowanceDefaultsAndOverrides(t *testing.T) {
	t.Parallel()
	conf := filepath.Join(t.TempDir(), "metasystem.conf")
	putFile(t, conf, "# lane allowance\n")
	for key, want := range map[string]int{"landing.fix-rounds": 1, "landing.fix-minutes": 120} {
		if got := LandingFixLimit(conf, key); got != want {
			t.Fatalf("%s = %d, want %d", key, got, want)
		}
	}
	for _, row := range []struct {
		text            string
		rounds, minutes int
	}{
		{"landing.fix-rounds=0\nlanding.fix-minutes=7\n", 0, 7},
		{"landing.fix-rounds=2\nlanding.fix-minutes=31\n", 2, 31},
		{"landing.fix-rounds=-1\nlanding.fix-minutes=oops\n", 0, 0},
	} {
		putFile(t, conf, row.text)
		if r, m := LandingFixLimit(conf, "landing.fix-rounds"), LandingFixLimit(conf, "landing.fix-minutes"); r != row.rounds || m != row.minutes {
			t.Fatalf("%q: %d/%d", row.text, r, m)
		}
	}
}
