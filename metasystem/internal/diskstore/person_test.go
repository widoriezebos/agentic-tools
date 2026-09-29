package diskstore

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
)

// ageTree sets every entry under root to at.
func ageTree(t *testing.T, root string, at time.Time) {
	t.Helper()
	var paths []string
	if err := filepath.WalkDir(root, func(path string, _ fs.DirEntry, err error) error {
		paths = append(paths, path)
		return err
	}); err != nil {
		t.Fatal(err)
	}
	for index := len(paths) - 1; index >= 0; index-- {
		if err := os.Chtimes(paths[index], at, at); err != nil {
			t.Fatal(err)
		}
	}
}

func strayFixture(t *testing.T, temp, name string, at time.Time) string {
	t.Helper()
	path := filepath.Join(temp, name)
	if err := os.MkdirAll(filepath.Join(path, "inner"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(path, "inner", "data"), []byte("bed output"), 0o600); err != nil {
		t.Fatal(err)
	}
	ageTree(t, path, at)
	return path
}

// --strays removes exactly the planned strays that are still the same
// entry, still idle a day and held by no readable process; a young stray, a
// replaced one and a held one are declined with the reason and what to run;
// foreign entries are never in the plan; a repeat writes nothing (3.8, R4).
func TestStraysRemoveOnlyWhatThePreviewShowedAndStillHolds(t *testing.T) {
	t.Parallel()
	root := realDir(t)
	temp := filepath.Join(root, "tmp")
	old := testNow.Add(-72 * time.Hour)
	idle := strayFixture(t, temp, "metasystem-audit.idle", old)
	young := strayFixture(t, temp, "metasystem-audit.young", testNow.Add(-time.Hour))
	replaced := strayFixture(t, temp, "goal-txn-replaced", old)
	held := strayFixture(t, temp, "metasystem-bed.held", old)
	foreign := strayFixture(t, temp, "tmp.foreign", old)
	registry := Registry{Dir: filepath.Join(root, "stores")}
	options := passOptions(registry, root, TempStrays{Roots: []string{temp}})
	options.Mode = ModePreview
	report, err := RunPass(context.Background(), options)
	if err != nil {
		t.Fatal(err)
	}
	plan, err := ReadPlan(options.PlanDir, report.Plan)
	if err != nil {
		t.Fatal(err)
	}
	for _, item := range plan.Items {
		if item.Path == foreign || strings.HasPrefix(filepath.Base(item.Path), "tmp.") {
			t.Fatalf("a foreign entry is in the strays plan: %+v", item)
		}
	}
	// The replaced entry is recreated under the same name after the preview.
	if err := RemoveTree(context.Background(), replaced); err != nil {
		t.Fatal(err)
	}
	strayFixture(t, temp, "goal-txn-replaced", old)
	census := &UseCensus{Taken: true, Processes: []CensusProcess{{Pid: 4242, UID: 501, Command: "bash fixture.sh", Cwd: "/elsewhere",
		Files: []string{filepath.Join(held, "inner", "data")}}}, Unreadable: []CensusGap{{Pid: 7, Reason: "descriptor list unreadable"}}}
	outcomes := map[string]PersonOutcome{}
	for _, outcome := range ExecuteStrays(context.Background(), plan, testNow, census, []string{temp}) {
		outcomes[outcome.Path] = outcome
	}
	if !outcomes[idle].Done {
		t.Fatalf("the idle stray = %+v", outcomes[idle])
	}
	if _, err := os.Stat(idle); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("the idle stray survived: %v", err)
	}
	for path, want := range map[string]string{young: "idle a day", replaced: "replaced since the preview", held: "in use by pid 4242"} {
		outcome := outcomes[path]
		if outcome.Done || !strings.Contains(outcome.Reason, want) || !strings.HasPrefix(outcome.Command, "metasystem disk clean") {
			t.Errorf("%s = %+v; want declined with %q and a public command", filepath.Base(path), outcome, want)
		}
		if _, err := os.Stat(path); err != nil {
			t.Errorf("%s was removed: %v", path, err)
		}
	}
	if _, err := os.Stat(foreign); err != nil {
		t.Fatalf("the foreign entry was touched: %v", err)
	}
	before := snapshotTree(t, root)
	again := ExecuteStrays(context.Background(), plan, testNow, census, []string{temp})
	for _, outcome := range again {
		if outcome.Path == idle && (!outcome.Done || outcome.Reason != "already gone") {
			t.Fatalf("the repeat for the removed stray = %+v", outcome)
		}
	}
	if snapshotTree(t, root) != before {
		t.Fatal("a repeat changed the disk")
	}
	// A plan item outside the roots, or not engine-named, is never removed.
	outside := Plan{Items: []Item{{Path: filepath.Join(root, "metasystem-elsewhere"), Stray: true}, {Path: filepath.Join(temp, "notes"), Stray: true}}}
	for _, outcome := range ExecuteStrays(context.Background(), outside, testNow, census, []string{temp}) {
		if outcome.Done {
			t.Fatalf("an item outside the engine's namespace was acted on: %+v", outcome)
		}
	}
}

// --release (DL3B-12): with the owner's proof holding and only an
// incomplete census in the way, a person releases the store; a readable
// process inside declines it naming the pid; a kind with no proof is kept;
// a repeat succeeds and writes nothing.
func TestReleaseByPersonSuppliesOnlyTheUseJudgement(t *testing.T) {
	t.Parallel()
	root := realDir(t)
	registry := Registry{Dir: filepath.Join(root, "stores")}
	store := worktreeStore(t, registry, root, "g1")
	proof := &gitProof{}
	options := passOptions(registry, root, RegisteredStores{Registry: registry, Proofs: map[OwnerKind]OwnerProof{OwnerGoal: proof}})
	options.Mode = ModePreview
	options.CensusReader = fakeCensus(map[int64]identity.ProcessUse{1: {Cwd: "/", Executable: "/sbin/launchd"}}, map[int64]bool{7: true})
	report, err := RunPass(context.Background(), options)
	if err != nil {
		t.Fatal(err)
	}
	if line := lineMap(report.Pending)[store.Path]; line.Command != "metasystem disk clean --release "+store.ID || !strings.Contains(line.Reason, "pid 7") {
		t.Fatalf("the preview = %+v", report.Pending)
	}
	holding := &UseCensus{Taken: true, Processes: []CensusProcess{{Pid: 4242, UID: 501, Command: "vim notes", Cwd: store.Path}},
		Unreadable: []CensusGap{{Pid: 7}}}
	verdict, err := ReleaseByPerson(context.Background(), registry, store.ID, proof, holding, "Wido")
	if err != nil || verdict.Decision != Keep || !strings.Contains(verdict.Reason, "pid 4242") || !strings.Contains(verdict.Command, "--release "+store.ID) {
		t.Fatalf("a held store = %+v, %v", verdict, err)
	}
	if len(proof.removed) != 0 {
		t.Fatal("a held store was removed")
	}
	if verdict, _ := ReleaseByPerson(context.Background(), registry, store.ID, nil, holding, "Wido"); verdict.Decision != Keep || !strings.Contains(verdict.Reason, "no proof for owner kind goal") {
		t.Fatalf("no proof = %+v", verdict)
	}
	gaps := &UseCensus{Taken: true, Unreadable: []CensusGap{{Pid: 7}}}
	verdict, err = ReleaseByPerson(context.Background(), registry, store.ID, proof, gaps, "Wido")
	if err != nil || verdict.Decision != Release || len(proof.removed) != 1 {
		t.Fatalf("release with only gaps = %+v, %v (removed %v)", verdict, err, proof.removed)
	}
	if record, _ := registry.Load(store.ID); record.State != StateReleased || record.ReleasedBy != "person Wido" {
		t.Fatalf("record after release = %+v", record)
	}
	before := snapshotTree(t, registry.Dir)
	if verdict, err := ReleaseByPerson(context.Background(), registry, store.ID, proof, gaps, "Wido"); err != nil || verdict.Reason != "already released" {
		t.Fatalf("a repeat = %+v, %v", verdict, err)
	}
	if snapshotTree(t, registry.Dir) != before || len(proof.removed) != 1 {
		t.Fatal("a repeat wrote or removed something")
	}
	if _, err := ReleaseByPerson(context.Background(), registry, "01ARZ3NDEKTSV4RRFFQ69G5FAV", proof, gaps, "Wido"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("an unknown id = %v", err)
	}
	// Content proofs are never waived: an open goal is kept.
	open := worktreeStore(t, registry, root, "g2")
	record, _ := registry.Load(open.ID)
	record.Owner.Ref = "open"
	critical, _ := registry.TryCritical(open.ID)
	_ = critical.Write(record)
	_ = critical.Release()
	if verdict, _ := ReleaseByPerson(context.Background(), registry, open.ID, proof, gaps, "Wido"); verdict.Decision != Keep || verdict.Reason != "goal open" {
		t.Fatalf("an open goal's store = %+v", verdict)
	}
}
