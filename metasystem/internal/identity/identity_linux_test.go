//go:build linux

package identity

import "testing"

// The /proc stat parse, including the parsing trap the portability ledger
// names: field 2 is the executable name in parentheses and may itself
// contain spaces and closing parentheses.
func TestParseProcStat(t *testing.T) {
	cases := []struct {
		name  string
		stat  string
		ticks int64
		ppid  int64
		fails bool
	}{
		{"plain comm", "1234 (sleep) S 1 1234 1234 0 -1 4194304 100 0 0 0 0 0 0 0 20 0 1 0 555555 1000 1 18446744073709551615", 555555, 1, false},
		{"comm with spaces and parens", "1234 (a b) c)) S 77 1234 1234 0 -1 4194304 100 0 0 0 0 0 0 0 20 0 1 0 999 1000 1 0", 999, 77, false},
		{"no comm delimiter", "1234 broken S 1", 0, 0, true},
		{"short line", "1234 (x) S 1 2 3", 0, 0, true},
	}
	for _, tc := range cases {
		ticks, ppid, err := parseProcStat(tc.stat)
		if tc.fails {
			if err == nil {
				t.Fatalf("%s: expected parse failure", tc.name)
			}
			continue
		}
		if err != nil || ticks != tc.ticks || ppid != tc.ppid {
			t.Fatalf("%s: got (%d, %d, %v), want (%d, %d)", tc.name, ticks, ppid, err, tc.ticks, tc.ppid)
		}
	}
}
