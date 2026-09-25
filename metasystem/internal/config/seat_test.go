package config

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestParseSeatPresenceStaleMinutes(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		value string
		want  uint64
		fails bool
	}{
		{value: "30", want: 30},
		{value: "1", want: 1},
		{value: "0", fails: true},
		{value: "", fails: true},
		{value: "-5", fails: true},
		{value: " 5", fails: true},
		{value: "5m", fails: true},
		{value: "99999999999999999999999", fails: true},
	} {
		minutes, err := parseSeatPresenceStaleMinutes(test.value)
		if test.fails {
			if err == nil || !strings.Contains(err.Error(), SeatPresenceStaleMinutesKey) {
				t.Errorf("%q: minutes=%d err=%v, want refusal naming %s", test.value, minutes, err, SeatPresenceStaleMinutesKey)
			}
			continue
		}
		if err != nil || minutes != test.want {
			t.Errorf("%q: minutes=%d err=%v, want %d", test.value, minutes, err, test.want)
		}
	}
}

func TestParseSeatPresenceNamespace(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		value string
		want  string
		fails bool
	}{
		{value: "", want: ""},
		{value: "   ", want: ""},
		{value: seatMetasystemNamespace, want: seatMetasystemNamespace},
		{value: seatBranchNamespace + "/", want: seatBranchNamespace},
		{value: "  " + seatMetasystemNamespace + "/ ", want: seatMetasystemNamespace},
		{value: "refs/heads/other", fails: true},
		{value: "refs/metasystem", fails: true},
		{value: "/", want: ""},
	} {
		namespace, err := parseSeatPresenceNamespace(test.value)
		if test.fails {
			if err == nil || !strings.Contains(err.Error(), SeatPresenceNamespaceKey) {
				t.Errorf("%q: namespace=%q err=%v, want refusal naming %s", test.value, namespace, err, SeatPresenceNamespaceKey)
			}
			continue
		}
		if err != nil || namespace != test.want {
			t.Errorf("%q: namespace=%q err=%v, want %q", test.value, namespace, err, test.want)
		}
	}
}

// seatProductionRoot returns a temporary conf path that reads through the
// committed-only production gate. TestMain clears the inherited seat keys.
func seatProductionRoot(t *testing.T) string {
	t.Helper()
	conf := filepath.Join(t.TempDir(), "metasystem.conf")
	putFile(t, conf, "")
	if fixtureBudgetLawRoot(conf) {
		t.Fatal("precondition: an empty temporary root must not be fixture-authorized")
	}
	return conf
}

func TestSeatPresenceProductionReadsCommittedValues(t *testing.T) {
	t.Parallel()
	conf := seatProductionRoot(t)

	minutes, err := SeatPresenceStaleMinutes(conf)
	if err != nil || minutes != DefaultSeatPresenceStaleMinutes {
		t.Fatalf("absent key: minutes=%d err=%v", minutes, err)
	}
	namespace, err := SeatPresenceNamespace(conf)
	if err != nil || namespace != "" {
		t.Fatalf("absent key: namespace=%q err=%v", namespace, err)
	}

	putFile(t, conf, SeatPresenceStaleMinutesKey+"=45\n"+SeatPresenceNamespaceKey+"="+seatBranchNamespace+"/\n")
	if minutes, err = SeatPresenceStaleMinutes(conf); err != nil || minutes != 45 {
		t.Fatalf("committed value: minutes=%d err=%v", minutes, err)
	}
	if namespace, err = SeatPresenceNamespace(conf); err != nil || namespace != seatBranchNamespace {
		t.Fatalf("committed value: namespace=%q err=%v", namespace, err)
	}

	putFile(t, conf, SeatPresenceStaleMinutesKey+"=0\n"+SeatPresenceNamespaceKey+"=refs/heads/elsewhere\n")
	if _, err = SeatPresenceStaleMinutes(conf); err == nil || !strings.Contains(err.Error(), "positive integer") {
		t.Fatalf("committed zero window accepted: %v", err)
	}
	if _, err = SeatPresenceNamespace(conf); err == nil || !strings.Contains(err.Error(), "refs/heads/elsewhere") {
		t.Fatalf("committed foreign namespace accepted: %v", err)
	}
}

func TestSeatPresenceProductionRefusesLocalSource(t *testing.T) {
	t.Parallel()
	conf := seatProductionRoot(t)
	putFile(t, conf, SeatPresenceStaleMinutesKey+"=45\n")
	putFile(t, conf+".local", SeatPresenceStaleMinutesKey+"=5\n"+SeatPresenceNamespaceKey+"="+seatMetasystemNamespace+"\n")

	if _, err := SeatPresenceStaleMinutes(conf); err == nil ||
		!strings.Contains(err.Error(), "resolve "+SeatPresenceStaleMinutesKey) ||
		!strings.Contains(err.Error(), "committed root configuration") {
		t.Fatalf("production accepted local stale-window authority: %v", err)
	}
	if _, err := SeatPresenceNamespace(conf); err == nil ||
		!strings.Contains(err.Error(), "resolve "+SeatPresenceNamespaceKey) ||
		!strings.Contains(err.Error(), "committed root configuration") {
		t.Fatalf("production accepted local namespace authority: %v", err)
	}
}

func TestSeatPresenceFixtureRootAcceptsLocalOverrides(t *testing.T) {
	t.Parallel()
	conf := filepath.Join(t.TempDir(), "metasystem.conf")
	putFile(t, conf, "metasystem.runtimes=fake\n"+SeatPresenceStaleMinutesKey+"=45\n")
	putFile(t, conf+".local", SeatPresenceStaleMinutesKey+"=5\n"+SeatPresenceNamespaceKey+"="+seatMetasystemNamespace+"\n")
	if !fixtureBudgetLawRoot(conf) {
		t.Fatal("precondition: a fake-runtime root must be fixture-authorized")
	}

	if minutes, err := SeatPresenceStaleMinutes(conf); err != nil || minutes != 5 {
		t.Fatalf("fixture override: minutes=%d err=%v", minutes, err)
	}
	if namespace, err := SeatPresenceNamespace(conf); err != nil || namespace != seatMetasystemNamespace {
		t.Fatalf("fixture override: namespace=%q err=%v", namespace, err)
	}
}
