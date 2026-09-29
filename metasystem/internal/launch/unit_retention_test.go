package launch

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/diskstore"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/lock"
)

// unitRecord writes one unit run record whose last step ended age ago, a
// launch its read step names, and a named entry that reaches it.
func (f *retentionFixture) unitRun(id, goal string, age time.Duration, bytes int) {
	f.t.Helper()
	at := f.now.Add(-age).Format(time.RFC3339Nano)
	record := UnitRunRecord{ID: id, Unit: "u", Goal: goal, Worktree: "/nowhere/" + goal, State: "awaiting-judgement",
		Rounds: []UnitRound{{Number: 1, Steps: []UnitStep{{Name: "read", LaunchID: id + "-read", State: StepPassed, StartedAt: at, FinishedAt: at}}}}}
	data, err := json.Marshal(record)
	if err != nil {
		f.t.Fatal(err)
	}
	dir := filepath.Join(f.units, id)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		f.t.Fatal(err)
	}
	for name, content := range map[string][]byte{"run.json": data, ".lock": nil, "payload": []byte(strings.Repeat("x", bytes))} {
		if err := os.WriteFile(filepath.Join(dir, name), content, 0o600); err != nil {
			f.t.Fatal(err)
		}
	}
	named, _ := json.Marshal(namedUnitEntry{Worktree: record.Worktree, Goal: goal, Unit: "u", Run: id, State: namedRecorded})
	if err := os.MkdirAll(filepath.Join(f.units, ".named"), 0o700); err != nil {
		f.t.Fatal(err)
	}
	for name, content := range map[string][]byte{"key-" + id + ".json": named, "key-" + id + ".lock": nil} {
		if err := os.WriteFile(filepath.Join(f.units, ".named", name), content, 0o600); err != nil {
			f.t.Fatal(err)
		}
	}
}

func (f *retentionFixture) unitRetention(target int64, ended map[string]bool) *UnitRetention {
	return &UnitRetention{Root: f.units, Target: target, Keep: 14 * day,
		GoalEnded: func(goal, _ string) (bool, bool) { value, known := ended[goal]; return value, known }}
}

func (f *retentionFixture) unitExists(id string) bool {
	_, err := os.Stat(filepath.Join(f.units, id))
	return err == nil
}

// The unit proof of 3.1 (U5d, DL3B-03): over disk.unit-target-mib a unit
// whose goal has concluded, whose locks are free and whose last step ended
// past disk.unit-keep-days goes, with its named entry; a unit of an open
// goal survives whatever its age and names the goal, and so does one whose
// goal state is unknown; one whose run lock is held is pending; a young one
// stays.
func TestUnitRetentionReleasesOnlyConcludedGoalsUnits(t *testing.T) {
	t.Parallel()
	f := newRetentionFixture(t)
	f.unitRun("unit-done", "done", 40*day, 4096)
	f.unitRun("unit-open", "open", 90*day, 4096)
	f.unitRun("unit-unknown", "unknown", 60*day, 4096)
	f.unitRun("unit-held", "done", 50*day, 4096)
	f.unitRun("unit-young", "done", 2*day, 4096)
	held, err := lock.File(filepath.Join(f.units, "unit-held", ".lock"), 0o600, lock.Exclusive)
	if err != nil {
		t.Fatal(err)
	}
	defer held.Release()
	report := f.pass(f.unitRetention(1, map[string]bool{"done": true, "open": false}))
	for id, want := range map[string]bool{"unit-done": false, "unit-open": true, "unit-unknown": true, "unit-held": true, "unit-young": true} {
		if got := f.unitExists(id); got != want {
			t.Errorf("unit %s present = %v, want %v", id, got, want)
		}
	}
	if _, err := os.Stat(filepath.Join(f.units, ".named", "key-unit-done.json")); !os.IsNotExist(err) {
		t.Errorf("the released unit's named entry goes with it: %v", err)
	}
	var openNamed, heldPending, unknownPending bool
	for _, line := range report.Kept {
		openNamed = openNamed || strings.Contains(line.Path, "unit-open") && strings.Contains(line.Command, "metasystem goal show open")
		unknownPending = unknownPending || strings.Contains(line.Path, "unit-unknown")
	}
	for _, line := range report.Pending {
		heldPending = heldPending || strings.Contains(line.Path, "unit-held")
	}
	if !openNamed || !heldPending || !unknownPending {
		t.Fatalf("open kept=%v held pending=%v unknown kept=%v\n%+v", openNamed, heldPending, unknownPending, report)
	}
	// With the unit gone, the launch its round named is no longer a root.
	if named, err := namedLaunches(f.units); err != nil || named["unit-done-read"] || !named["unit-open-read"] {
		t.Fatalf("named launches after the release: %v %v", named, err)
	}
}

// Under its target the unit store keeps every unit.
func TestUnitRetentionUnderTargetKeepsEverything(t *testing.T) {
	t.Parallel()
	f := newRetentionFixture(t)
	f.unitRun("unit-done", "done", 40*day, 10)
	if report := f.pass(f.unitRetention(256<<20, map[string]bool{"done": true})); len(report.Actions) != 0 || !f.unitExists("unit-done") {
		t.Fatalf("under target nothing goes: %+v", report)
	}
}

var _ diskstore.Class = (*UnitRetention)(nil)
