package hostload

import (
	"strings"
	"testing"
	"time"
)

func TestReadAlwaysYieldsARecordableSample(t *testing.T) {
	now := time.Date(2026, 9, 12, 18, 0, 0, 0, time.UTC)
	sample := Read(now)
	if sample.At != "2026-09-12T18:00:00Z" || sample.Cores < 1 {
		t.Fatalf("sample = %+v", sample)
	}
	if sample.Available && (sample.Load1m < 0 || sample.Detail != "") {
		t.Fatalf("an available sample carries a detail or a negative load: %+v", sample)
	}
	if !sample.Available && sample.Detail == "" {
		t.Fatalf("an unavailable sample says nothing about why: %+v", sample)
	}
}

func TestSaturatedComparesTheMinuteLoadToTheCores(t *testing.T) {
	for _, test := range []struct {
		sample Sample
		want   bool
	}{
		{Sample{Available: true, Cores: 18, Load1m: 17.9}, false},
		{Sample{Available: true, Cores: 18, Load1m: 18}, true},
		{Sample{Available: true, Cores: 18, Load1m: 47.2}, true},
		{Sample{Available: false, Cores: 18, Load1m: 99}, false},
		{Sample{Available: true, Cores: 0, Load1m: 1}, false},
	} {
		if got := test.sample.Saturated(); got != test.want {
			t.Fatalf("%+v saturated = %v, want %v", test.sample, got, test.want)
		}
	}
}

func TestParseProcLoadavg(t *testing.T) {
	one, five, fifteen, err := parseProcLoadavg("0.52 1.25 2.00 3/1024 4242\n")
	if err != nil || one != 0.52 || five != 1.25 || fifteen != 2 {
		t.Fatalf("parsed %v %v %v, %v", one, five, fifteen, err)
	}
	for _, malformed := range []string{"", "1.0 2.0", "a b c"} {
		if _, _, _, err := parseProcLoadavg(malformed); err == nil || !strings.Contains(err.Error(), "/proc/loadavg") {
			t.Fatalf("%q parsed: %v", malformed, err)
		}
	}
}
