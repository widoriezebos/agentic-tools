package launch

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeFollowUp(t *testing.T) string {
	t.Helper()
	follow := filepath.Join(t.TempDir(), "follow")
	if err := os.WriteFile(follow, []byte("Declared size: 1 changed lines\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	return follow
}

func requireRecordedRounds(t *testing.T, fixture unitFixture, run string, want int) UnitRunRecord {
	t.Helper()
	record, err := fixture.runner.Status(run)
	if err != nil || record.ID != run || len(record.Rounds) != want {
		t.Fatalf("status=%+v err=%v, want %d rounds", record, err, want)
	}
	return record
}

// Continue reaches a named unit's run by id under the unit's reservation: a
// plain repeat reads the judgement back without launching, a follow-up
// is refused while the reserved inputs differ, and runs once they are
// restored. A caller already holding the unit's name is told which run.
func TestContinueNamedRunIsBoundToItsReservation(t *testing.T) {
	t.Parallel()
	fixture := newUnitFixture(t, "", "branch", "branch", "round", "branch", "round")
	first, err := fixture.runner.AdvanceNamed(fixture.plan)
	if err != nil || first.Record.State != "awaiting-judgement" {
		t.Fatalf("result=%+v err=%v", first, err)
	}
	for _, request := range []UnitRequest{{}, {Resume: first.Record.ID, Plan: fixture.plan}} {
		if _, err := fixture.runner.Continue(request); err == nil || !strings.Contains(err.Error(), "requires a run id and no plan") {
			t.Fatalf("request=%+v err=%v", request, err)
		}
	}
	if _, err := fixture.runner.Continue(UnitRequest{Resume: "../" + first.Record.ID}); err == nil || !strings.Contains(err.Error(), "invalid unit run id") {
		t.Fatalf("err=%v", err)
	}
	if _, err := fixture.runner.Continue(UnitRequest{Resume: "unknown-run"}); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("err=%v", err)
	}
	again, err := fixture.runner.Continue(UnitRequest{Resume: first.Record.ID})
	if err != nil || again.Record.ID != first.Record.ID || again.Record.State != "awaiting-judgement" || again.Round != 1 {
		t.Fatalf("result=%+v err=%v", again, err)
	}
	requireLaunchedOnce(t, fixture, 3)

	plan, _ := ReadUnitPlan(fixture.plan)
	_, key, _ := namedUnitIdentity(plan)
	lock, err := fixture.runner.namedLock(key, plan)
	if err != nil {
		t.Fatal(err)
	}
	follow := writeFollowUp(t)
	_, err = fixture.runner.Continue(UnitRequest{Resume: first.Record.ID, FollowUp: follow})
	releaseUnitLock(lock)
	if err == nil || !strings.Contains(err.Error(), "UNIT_RUN_BUSY") || !strings.Contains(err.Error(), "run="+first.Record.ID) {
		t.Fatalf("err=%v", err)
	}

	if err := os.WriteFile(plan.Read.Brief, []byte("read something else\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err = fixture.runner.Continue(UnitRequest{Resume: first.Record.ID, FollowUp: follow})
	if err == nil || !strings.Contains(err.Error(), "UNIT_NAMED_INPUT_CHANGED") || !strings.Contains(err.Error(), "run="+first.Record.ID) {
		t.Fatalf("err=%v", err)
	}
	requireRecordedRounds(t, fixture, first.Record.ID, 1)
	requireLaunchedOnce(t, fixture, 3)

	if err := os.WriteFile(plan.Read.Brief, []byte("read it\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	second, err := fixture.runner.Continue(UnitRequest{Resume: first.Record.ID, FollowUp: follow})
	if err != nil || second.Record.ID != first.Record.ID || len(second.Record.Rounds) != 2 || second.Record.Rounds[1].Outcome != "green" {
		t.Fatalf("result=%+v err=%v", second, err)
	}
	requireLaunchedOnce(t, fixture, 6)
}

// A run no named entry claims continues exactly as Advance and writes no
// entry; a run whose worktree is gone can still be read back but not
// continued.
func TestContinueLegacyRunAndAMissingWorktree(t *testing.T) {
	t.Parallel()
	t.Run("legacy", func(t *testing.T) {
		t.Parallel()
		fixture := newUnitFixture(t, "", "branch", "round", "branch", "round")
		first, err := fixture.runner.Advance(UnitRequest{Plan: fixture.plan})
		if err != nil {
			t.Fatal(err)
		}
		second, err := fixture.runner.Continue(UnitRequest{Resume: first.Record.ID, FollowUp: writeFollowUp(t)})
		if err != nil || second.Record.ID != first.Record.ID || len(second.Record.Rounds) != 2 || second.Record.State != "awaiting-judgement" {
			t.Fatalf("result=%+v err=%v", second, err)
		}
		if _, err := os.Stat(filepath.Join(fixture.runner.Root, ".named")); !os.IsNotExist(err) {
			t.Fatalf("legacy continuation wrote a named entry: %v", err)
		}
		requireLaunchedOnce(t, fixture, 6)
	})
	t.Run("worktree-gone", func(t *testing.T) {
		t.Parallel()
		fixture := newUnitFixture(t, "", "branch", "branch", "round")
		first, err := fixture.runner.AdvanceNamed(fixture.plan)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.RemoveAll(fixture.worktree); err != nil {
			t.Fatal(err)
		}
		read, err := fixture.runner.Continue(UnitRequest{Resume: first.Record.ID})
		if err != nil || read.Record.ID != first.Record.ID || read.Record.State != "awaiting-judgement" || read.Round != 1 {
			t.Fatalf("result=%+v err=%v", read, err)
		}
		if _, err := fixture.runner.Continue(UnitRequest{Resume: first.Record.ID, FollowUp: writeFollowUp(t)}); err == nil || !strings.Contains(err.Error(), "UNIT_PLAN_INVALID") {
			t.Fatalf("err=%v", err)
		}
		requireRecordedRounds(t, fixture, first.Record.ID, 1)
		requireLaunchedOnce(t, fixture, 3)
	})
}

// A prepared unit is generated once into its named input directory. The same
// request reuses the retained plan and run without preparing again; another
// request is refused with the run's id. The unit's own model and round limit
// are recorded, bound into the reservation and honoured by continuation.
func TestAdvancePreparedBindsTheRequestAndItsOptions(t *testing.T) {
	t.Parallel()
	fixture := newUnitFixture(t, "", "branch", "branch", "round", "branch", "round", "branch")
	source, err := os.ReadFile(fixture.plan)
	if err != nil {
		t.Fatal(err)
	}
	directory, err := fixture.runner.NamedInputDirectory(fixture.worktree, "goal", "U")
	if err != nil {
		t.Fatal(err)
	}
	prepared := []string{}
	prepare := func(into string) (string, error) {
		prepared = append(prepared, into)
		path := filepath.Join(into, "plan.json")
		return path, os.WriteFile(path, source, 0o600)
	}
	options := UnitOptions{BuildModel: "unit-model", BuildEffort: "high", MaxRounds: 2}
	request := []byte(`{"brief":"one"}`)
	first, err := fixture.runner.AdvancePrepared(fixture.worktree, "goal", "U", request, options, prepare)
	if err != nil || first.Record.State != "awaiting-judgement" || first.Record.Rounds[0].Outcome != "green" {
		t.Fatalf("result=%+v err=%v", first, err)
	}
	if len(prepared) != 1 || prepared[0] != directory {
		t.Fatalf("prepared=%v, want once into %s", prepared, directory)
	}
	if retained, err := os.ReadFile(filepath.Join(directory, "request.json")); err != nil || string(retained) != string(request) {
		t.Fatalf("retained request=%q err=%v", retained, err)
	}
	record := first.Record
	if record.BuildModel != "unit-model" || record.BuildEffort != "high" || record.MaxRounds != 2 || stepNamed(t, record.Rounds[0], "build").Model != "unit-model" {
		t.Fatalf("record options=%q %q %d build=%+v", record.BuildModel, record.BuildEffort, record.MaxRounds, stepNamed(t, record.Rounds[0], "build"))
	}

	again, err := fixture.runner.AdvancePrepared(fixture.worktree, "goal", "U", request, options, prepare)
	if err != nil || again.Record.ID != first.Record.ID || len(prepared) != 1 {
		t.Fatalf("repeat=%+v prepared=%v err=%v", again, prepared, err)
	}
	_, err = fixture.runner.AdvancePrepared(fixture.worktree, "goal", "U", []byte(`{"brief":"two"}`), options, prepare)
	if err == nil || !strings.Contains(err.Error(), "UNIT_NAMED_INPUT_CHANGED") || !strings.Contains(err.Error(), "run="+first.Record.ID) || len(prepared) != 1 {
		t.Fatalf("changed request err=%v prepared=%v", err, prepared)
	}
	if retained, _ := os.ReadFile(filepath.Join(directory, "request.json")); string(retained) != string(request) {
		t.Fatalf("changed request overwrote the retained one: %q", retained)
	}
	requireLaunchedOnce(t, fixture, 3)

	second, err := fixture.runner.Continue(UnitRequest{Resume: first.Record.ID, FollowUp: writeFollowUp(t)})
	if err != nil || len(second.Record.Rounds) != 2 || second.Record.Rounds[1].Outcome != "green" || stepNamed(t, second.Record.Rounds[1], "build").Model != "unit-model" {
		t.Fatalf("follow-up=%+v err=%v", second, err)
	}
	_, err = fixture.runner.Continue(UnitRequest{Resume: first.Record.ID, FollowUp: writeFollowUp(t)})
	if err == nil || !strings.Contains(err.Error(), "UNIT_ROUND_LIMIT") || !strings.Contains(err.Error(), "limit=2") {
		t.Fatalf("err=%v", err)
	}
	requireRecordedRounds(t, fixture, first.Record.ID, 2)
	requireLaunchedOnce(t, fixture, 6)
}

// A unit with no run reserves nothing when its preparation fails or yields a
// plan for another unit, and nothing is advanced without a launch manager.
func TestAdvancePreparedRefusesWithoutReserving(t *testing.T) {
	t.Parallel()
	fixture := newUnitFixture(t, "", []string{}...)
	source, err := os.ReadFile(fixture.plan)
	if err != nil {
		t.Fatal(err)
	}
	failed := func(string) (string, error) { return "", os.ErrPermission }
	if _, err := fixture.runner.AdvancePrepared(fixture.worktree, "goal", "U", []byte("a"), UnitOptions{}, failed); err != os.ErrPermission {
		t.Fatalf("err=%v", err)
	}
	other := func(into string) (string, error) {
		path := filepath.Join(into, "plan.json")
		return path, os.WriteFile(path, source, 0o600)
	}
	if _, err := fixture.runner.AdvancePrepared(fixture.worktree, "goal", "V", []byte("a"), UnitOptions{}, other); err == nil || !strings.Contains(err.Error(), "UNIT_PLAN_INVALID") {
		t.Fatalf("err=%v", err)
	}
	if _, err := fixture.runner.AdvancePrepared(filepath.Join(fixture.worktree, "missing"), "goal", "U", []byte("a"), UnitOptions{}, other); err == nil || !strings.Contains(err.Error(), "UNIT_PLAN_INVALID") {
		t.Fatalf("err=%v", err)
	}
	if entries, _ := filepath.Glob(filepath.Join(fixture.runner.Root, ".named", "*.json")); len(entries) != 0 {
		t.Fatalf("entries=%v", entries)
	}
	if runs := unitRunDirectories(t, fixture.runner.Root); len(runs) != 0 {
		t.Fatalf("runs=%v", runs)
	}
	requireLaunchedOnce(t, fixture, 0)
	store := &UnitRunner{Root: fixture.runner.Root}
	if _, err := store.AdvancePrepared(fixture.worktree, "goal", "U", []byte("a"), UnitOptions{}, other); err == nil || !strings.Contains(err.Error(), "manager is unavailable") {
		t.Fatalf("err=%v", err)
	}
	if _, err := store.Continue(UnitRequest{Resume: "x"}); err == nil || !strings.Contains(err.Error(), "manager is unavailable") {
		t.Fatalf("err=%v", err)
	}
}

// A unit's input directory is named by its real worktree, goal and unit:
// an alias of the worktree reaches the same directory, and another goal,
// unit or worktree reaches a different one.
func TestNamedInputDirectoryFollowsTheUnitIdentity(t *testing.T) {
	t.Parallel()
	fixture := baseUnitFixture(t)
	alias := filepath.Join(t.TempDir(), "alias")
	if err := os.Symlink(fixture.worktree, alias); err != nil {
		t.Fatal(err)
	}
	other := t.TempDir()
	directory, err := fixture.runner.NamedInputDirectory(fixture.worktree, "goal", "U")
	if err != nil || filepath.Dir(directory) != filepath.Join(fixture.runner.Root, ".inputs") {
		t.Fatalf("directory=%s err=%v", directory, err)
	}
	if aliased, err := fixture.runner.NamedInputDirectory(alias, "goal", "U"); err != nil || aliased != directory {
		t.Fatalf("alias=%s err=%v, want %s", aliased, err, directory)
	}
	seen := map[string]bool{directory: true}
	for _, identity := range [][3]string{{fixture.worktree, "goal", "V"}, {fixture.worktree, "other", "U"}, {other, "goal", "U"}} {
		named, err := fixture.runner.NamedInputDirectory(identity[0], identity[1], identity[2])
		if err != nil || seen[named] {
			t.Fatalf("identity=%v directory=%s err=%v", identity, named, err)
		}
		seen[named] = true
	}
	if _, err := fixture.runner.NamedInputDirectory(filepath.Join(other, "missing"), "goal", "U"); err == nil || !strings.Contains(err.Error(), "UNIT_PLAN_INVALID") {
		t.Fatalf("err=%v", err)
	}
}
