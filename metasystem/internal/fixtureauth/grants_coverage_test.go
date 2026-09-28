package fixtureauth

import (
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// The group-ownership grant names the root it was issued for and admits the
// recorded launch proof only on a fake-runtime root; a nil authorization
// answers neither.
func TestGroupOwnershipGrantRootAndRecordedProof(t *testing.T) {
	t.Setenv(fixtureEnv, "")
	var none *Authorization
	if root := none.GroupOwnership().Root(); root != "" {
		t.Fatalf("nil authorization root = %q", root)
	}
	if none.GroupOwnership().AllowsRecordedGroupProof() {
		t.Fatal("a nil authorization allowed the recorded group proof")
	}

	production := fakeCheckout(t, "claude")
	authorization, err := New(production)
	if err != nil {
		t.Fatal(err)
	}
	if got := authorization.GroupOwnership().Root(); got != filepath.Clean(production) {
		t.Fatalf("root = %q, want %q", got, production)
	}
	if authorization.GroupOwnership().AllowsRecordedGroupProof() {
		t.Fatal("a production root allowed the recorded group proof")
	}

	fixture := fakeCheckout(t, "fake")
	authorization, err = New(fixture)
	if err != nil {
		t.Fatal(err)
	}
	if got := authorization.GroupOwnership().Root(); got != filepath.Clean(fixture) {
		t.Fatalf("root = %q, want %q", got, fixture)
	}
	if !authorization.GroupOwnership().AllowsRecordedGroupProof() {
		t.Fatal("a fake-runtime root refused the recorded group proof")
	}
}

// The mission-holder probe reads every fixture column, accepts the legacy
// "started" spelling, and answers "no entry" for an absent pid, an unreadable
// table, or a malformed one.
func TestMissionHolderReadsEveryFixtureColumn(t *testing.T) {
	root := fakeCheckout(t, "fake")
	t.Setenv(fixtureEnv, writeTable(t, `{
		"7":{"pidStartedAt":100,"pidStartedAtExactMicro":100000123,"pidStartTicks":55,"bootId":"boot-a","command":"runner --tag t","pgid":7,"terminal":true},
		"8":{"started":200}
	}`))
	authorization, err := New(root)
	if err != nil {
		t.Fatal(err)
	}
	entry, ok := authorization.MissionHolder().FixtureEntry(7)
	if !ok {
		t.Fatal("pid 7 not served")
	}
	if !entry.HasStartedAt || entry.StartedAt != 100 ||
		!entry.HasStartedAtExactMicro || entry.StartedAtExactMicro != 100000123 ||
		!entry.HasStartTicks || entry.StartTicks != 55 ||
		!entry.HasBootID || entry.BootID != "boot-a" ||
		!entry.HasCommand || entry.Command != "runner --tag t" ||
		!entry.HasPgid || entry.Pgid != 7 ||
		!entry.HasTerminal || !entry.Terminal {
		t.Fatalf("entry %+v", entry)
	}
	legacy, ok := authorization.MissionHolder().FixtureEntry(8)
	if !ok || !legacy.HasStartedAt || legacy.StartedAt != 200 || legacy.HasCommand || legacy.HasPgid || legacy.HasBootID {
		t.Fatalf("legacy started entry %+v ok=%v", legacy, ok)
	}
	if _, ok := authorization.MissionHolder().FixtureEntry(9); ok {
		t.Fatal("an absent pid was served")
	}

	t.Setenv(fixtureEnv, filepath.Join(t.TempDir(), "absent.json"))
	unreadable, err := New(root)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := unreadable.MissionHolder().FixtureEntry(7); ok {
		t.Fatal("an unreadable table served an entry")
	}

	t.Setenv(fixtureEnv, writeTable(t, `{"7":`))
	malformed, err := New(root)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := malformed.MissionHolder().FixtureEntry(7); ok {
		t.Fatal("a malformed table served an entry")
	}
	if pgid, command, ok := malformed.GroupOwnership().FixtureGroup(7); ok || pgid != 0 || command != "" {
		t.Fatalf("a malformed table authorized a group: %d %q", pgid, command)
	}
}

// The boot clock is one indivisible sample: fixture roots only, both halves
// together, and a non-negative integer nanosecond count.
func TestGoalBootClockIsOneIndivisibleFixtureSample(t *testing.T) {
	t.Setenv(fixtureEnv, "")
	t.Setenv(goalBootIDEnv, "boot-b")
	t.Setenv(goalBootNanosEnv, "1500")

	var none *Authorization
	if id, elapsed, ok, err := none.Clock().GoalBootClock(); ok || err != nil || id != "" || elapsed != 0 {
		t.Fatalf("nil authorization: %q %v %v %v", id, elapsed, ok, err)
	}
	production, err := New(fakeCheckout(t, "claude"))
	if err != nil {
		t.Fatal(err)
	}
	if _, _, ok, err := production.Clock().GoalBootClock(); ok || err != nil {
		t.Fatalf("a production root must ignore the boot clock: ok=%v err=%v", ok, err)
	}

	fixture, err := New(fakeCheckout(t, "fake"))
	if err != nil {
		t.Fatal(err)
	}
	id, elapsed, ok, err := fixture.Clock().GoalBootClock()
	if err != nil || !ok || id != "boot-b" || elapsed != 1500*time.Nanosecond {
		t.Fatalf("fixture boot clock: %q %v %v %v", id, elapsed, ok, err)
	}

	for _, tc := range []struct {
		id, nanos, want string
	}{
		{"", "", ""},
		{"boot-b", "", "must be set together"},
		{"", "1500", "must be set together"},
		{"boot-b", "soon", "non-negative duration in nanoseconds"},
		{"boot-b", "-1", "non-negative duration in nanoseconds"},
	} {
		t.Setenv(goalBootIDEnv, tc.id)
		t.Setenv(goalBootNanosEnv, tc.nanos)
		id, elapsed, ok, err := fixture.Clock().GoalBootClock()
		if tc.want == "" {
			if ok || err != nil || id != "" || elapsed != 0 {
				t.Fatalf("both unset must leave the kernel clock authoritative: %q %v %v %v", id, elapsed, ok, err)
			}
			continue
		}
		if ok || err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Fatalf("id=%q nanos=%q: ok=%v err=%v, want %q", tc.id, tc.nanos, ok, err, tc.want)
		}
	}
}

// GoalClock surfaces a refused authorization rather than falling back to the
// wall clock: a leaked identity fixture in a production checkout.
func TestGoalClockRefusesALeakedFixtureTable(t *testing.T) {
	t.Setenv(goalNowEnv, "")
	t.Setenv(fixtureEnv, writeTable(t, `{}`))
	wall := func() time.Time { return time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC) }
	clock, fixture, err := GoalClock(fakeCheckout(t, "claude"), wall)
	if err == nil || !strings.Contains(err.Error(), "metasystem.runtimes is not fake") || clock != nil || fixture {
		t.Fatalf("clock=%v fixture=%v err=%v", clock != nil, fixture, err)
	}
	// A fake root with no instant keeps the wall clock.
	clock, fixture, err = GoalClock(fakeCheckout(t, "fake"), wall)
	if err != nil || fixture || !clock().Equal(wall()) {
		t.Fatalf("fixture root without an instant: fixture=%v err=%v", fixture, err)
	}
}
