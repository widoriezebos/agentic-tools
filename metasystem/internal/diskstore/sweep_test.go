package diskstore

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"golang.org/x/sys/unix"
)

// fakeProof decides by the record's owner reference: "dead" releases,
// "alive" keeps, anything else is Unknown and pending. Its apply removes the
// marker store; blockRemoval stalls the removal until the context ends.
type fakeProof struct {
	kind         OwnerKind
	blockRemoval bool
	// cancel ends the pass's budget when the removal stalls, the way the
	// pass deadline would.
	cancel  context.CancelFunc
	applied []string
}

func (p *fakeProof) Kind() OwnerKind { return p.kind }
func (p *fakeProof) Observe(_ context.Context, record Record) Verdict {
	switch record.Owner.Ref {
	case "dead":
		return Verdict{Decision: Release, Reason: "owner dead"}
	case "alive":
		return Verdict{Decision: Keep, Reason: "owner alive", Command: "metasystem session stop fixture"}
	}
	return Verdict{Decision: Pending, Reason: "owner liveness unknown", Command: "metasystem disk show"}
}
func (p *fakeProof) Apply(ctx context.Context, critical *Critical) error {
	p.applied = append(p.applied, critical.Record().ID)
	if p.blockRemoval {
		return removeStore(ctx, critical.Record(), func(string) {
			p.cancel()
			<-ctx.Done()
		})
	}
	return RemoveStore(ctx, critical.Record())
}

func fixedClock() func() time.Time { return func() time.Time { return testNow } }

// plainStore registers and creates a marker store with one file.
func plainStore(t *testing.T, registry Registry, root, name, ref string) Record {
	t.Helper()
	path := filepath.Join(root, name)
	registration := plainRegistration(path)
	registration.Owner = Owner{Kind: OwnerProcess, Ref: ref}
	record, err := registry.Register(registration, testNow, rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(path, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := WriteMarker(record); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(path, "payload"), []byte(strings.Repeat("x", 1000)), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := registry.Accept(record.ID); err != nil {
		t.Fatal(err)
	}
	return record
}

func passOptions(registry Registry, root string, classes ...Class) PassOptions {
	return PassOptions{Kind: "checkout", Name: root, Registry: registry, LockPath: filepath.Join(registry.Dir, ".sweep.flock"),
		ReportPath: filepath.Join(root, "report.json"), PlanDir: filepath.Join(root, "plans"), Mode: ModeApply, Now: testNow,
		Clock: fixedClock(), Entropy: rand.Reader, Classes: classes, FloorMinAge: time.Hour}
}

// Only the owner kind's proof releases (R3): dead goes, alive is kept with
// its command, Unknown and a kind with no proof are pending; an
// unregistered directory beside them survives every pass and is reported.
func TestPassReleasesOnlyByTheOwnersProof(t *testing.T) {
	t.Parallel()
	root := realDir(t)
	registry := Registry{Dir: filepath.Join(root, "stores")}
	dead := plainStore(t, registry, root, "dead", "dead")
	alive := plainStore(t, registry, root, "alive", "alive")
	unknown := plainStore(t, registry, root, "unknown", "unknown")
	orphanKind := plainRegistration(filepath.Join(root, "launch"))
	orphanKind.Owner = Owner{Kind: OwnerLaunch, Ref: "l1"}
	noProof, err := registry.Register(orphanKind, testNow, rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	temp := filepath.Join(root, "tmp")
	unregistered := filepath.Join(temp, "metasystem-left-behind")
	if err := os.MkdirAll(unregistered, 0o700); err != nil {
		t.Fatal(err)
	}
	old := testNow.Add(-72 * time.Hour)
	if err := os.Chtimes(unregistered, old, old); err != nil {
		t.Fatal(err)
	}
	proof := &fakeProof{kind: OwnerProcess}
	options := passOptions(registry, root, RegisteredStores{Registry: registry, Proofs: map[OwnerKind]OwnerProof{OwnerProcess: proof}}, TempStrays{Roots: []string{temp}})
	for pass := 1; pass <= 2; pass++ {
		report, err := RunPass(context.Background(), options)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := os.Stat(unregistered); err != nil {
			t.Fatalf("pass %d removed an unregistered directory: %v", pass, err)
		}
		if len(report.Strays) != 1 || report.Strays[0].Path != unregistered {
			t.Fatalf("pass %d strays = %+v", pass, report.Strays)
		}
		if pass == 1 && (len(report.Actions) != 1 || report.Actions[0].Path != dead.Path) {
			t.Fatalf("pass 1 actions = %+v", report.Actions)
		}
		if pass == 2 && len(report.Actions) != 0 {
			t.Fatalf("pass 2 acted again: %+v (R-129)", report.Actions)
		}
		kept, pending := lineMap(report.Kept), lineMap(report.Pending)
		if kept[alive.Path].Command != "metasystem session stop fixture" || pending[unknown.Path].Reason == "" || !strings.Contains(pending[noProof.Path].Reason, "no proof for owner kind launch") {
			t.Fatalf("pass %d kept %v pending %v", pass, report.Kept, report.Pending)
		}
	}
	if _, err := os.Stat(dead.Path); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("the dead owner's store survived: %v", err)
	}
	if record, _ := registry.Load(dead.ID); record.State != StateReleased {
		t.Fatalf("dead store record = %s", record.State)
	}
	for _, kept := range []Record{alive, unknown} {
		if _, err := os.Stat(filepath.Join(kept.Path, "payload")); err != nil {
			t.Fatalf("%s lost its payload: %v", kept.Path, err)
		}
	}
	if len(proof.applied) != 1 {
		t.Fatalf("the proof applied %v", proof.applied)
	}
}

func lineMap(lines []Line) map[string]Line {
	byPath := map[string]Line{}
	for _, line := range lines {
		byPath[line.Path] = line
	}
	return byPath
}

// A held record lock is pending and a held pass lock is "a pass is
// running"; neither is waited on.
func TestHeldLocksArePendingNeverWaited(t *testing.T) {
	t.Parallel()
	root := realDir(t)
	registry := Registry{Dir: filepath.Join(root, "stores")}
	dead := plainStore(t, registry, root, "dead", "dead")
	entrant, err := registry.Enter(dead.ID)
	if err != nil {
		t.Fatal(err)
	}
	proof := &fakeProof{kind: OwnerProcess}
	options := passOptions(registry, root, RegisteredStores{Registry: registry, Proofs: map[OwnerKind]OwnerProof{OwnerProcess: proof}})
	report, err := RunPass(context.Background(), options)
	if err != nil {
		t.Fatal(err)
	}
	if line := lineMap(report.Pending)[dead.Path]; !strings.Contains(line.Reason, "record lock is held") {
		t.Fatalf("a store with an entrant = %+v", report.Pending)
	}
	if _, err := os.Stat(dead.Path); err != nil {
		t.Fatal("a store with an entrant was removed")
	}
	_ = entrant.Leave()

	lock, err := os.OpenFile(options.LockPath, os.O_RDWR, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer lock.Close()
	if err := flockRetry(lock, unix.LOCK_EX|unix.LOCK_NB); err != nil {
		t.Fatal(err)
	}
	report, err = RunPass(context.Background(), options)
	if err != nil || !report.Running {
		t.Fatalf("a pass under a held pass lock = %+v, %v; want running", report, err)
	}
}

// countingClass has n items; every apply counts, and the budget fake ends
// the context after `per` applications in one pass.
type countingClass struct {
	name    string
	items   int
	applied map[string]int
	spent   *int
	per     int
	cancel  *context.CancelFunc
}

func (c countingClass) Name() string { return c.name }
func (c countingClass) Plan(context.Context, *Pass) ([]Item, error) {
	var items []Item
	for index := range c.items {
		key := fmt.Sprintf("%s-%02d", c.name, index)
		if c.applied[key] == 0 {
			items = append(items, Item{Class: c.name, Key: key, Path: "/fixture/" + key, Verdict: Verdict{Decision: Release}})
		}
	}
	return items, nil
}
func (c countingClass) Apply(ctx context.Context, _ *Pass, item Item) Verdict {
	c.applied[item.Key]++
	*c.spent++
	if *c.spent == c.per {
		(*c.cancel)()
	}
	return Verdict{Decision: Release, Reason: "applied"}
}

// A large inventory under a small budget progresses across passes from its
// cursors, round-robin, and no class starves (3.3).
func TestSmallBudgetProgressesAcrossPassesAndNoClassStarves(t *testing.T) {
	t.Parallel()
	root := realDir(t)
	registry := Registry{Dir: filepath.Join(root, "stores")}
	applied := map[string]int{}
	spent := 0
	var cancel context.CancelFunc
	classes := []Class{
		countingClass{name: "a", items: 5, applied: applied, spent: &spent, per: 2, cancel: &cancel},
		countingClass{name: "b", items: 5, applied: applied, spent: &spent, per: 2, cancel: &cancel},
		countingClass{name: "c", items: 5, applied: applied, spent: &spent, per: 2, cancel: &cancel},
	}
	touched := map[string]bool{}
	for pass := 1; pass <= 3; pass++ {
		var ctx context.Context
		ctx, cancel = context.WithCancel(context.Background())
		spent = 0
		report, err := RunPass(ctx, passOptions(registry, root, classes...))
		cancel()
		if err != nil {
			t.Fatal(err)
		}
		if len(report.Actions) != 2 || len(report.Backlog) == 0 {
			t.Fatalf("pass %d: %d actions, backlog %v", pass, len(report.Actions), report.Backlog)
		}
		for _, action := range report.Actions {
			touched[strings.SplitN(filepath.Base(action.Path), "-", 2)[0]] = true
		}
	}
	if len(touched) != 3 {
		t.Fatalf("after three passes only classes %v progressed; one starved", touched)
	}
	for key, times := range applied {
		if times != 1 {
			t.Fatalf("%s applied %d times", key, times)
		}
	}
}

// stallingClass blocks its apply until the context ends: a stalled git or a
// held lock inside an owner. The stall ends the budget (cancel stands in
// for the pass deadline, so no test waits on wall time).
type stallingClass struct{ cancel context.CancelFunc }

func (stallingClass) Name() string { return "stalling" }
func (stallingClass) Plan(context.Context, *Pass) ([]Item, error) {
	return []Item{{Class: "stalling", Key: "s1", Path: "/fixture/s1", Verdict: Verdict{Decision: Release}},
		{Class: "stalling", Key: "s2", Path: "/fixture/s2", Verdict: Verdict{Decision: Release}}}, nil
}
func (c stallingClass) Apply(ctx context.Context, _ *Pass, _ Item) Verdict {
	c.cancel()
	<-ctx.Done()
	return Verdict{Decision: Pending, Reason: "owner call stalled: " + ctx.Err().Error(), Command: "metasystem disk clean"}
}

// A stalled owner call leaves the pass within its budget, the item pending
// and the rest backlog (DL2-17, R16).
func TestStalledOwnerLeavesThePassWithinBudget(t *testing.T) {
	t.Parallel()
	root := realDir(t)
	registry := Registry{Dir: filepath.Join(root, "stores")}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	report, _ := RunPass(ctx, passOptions(registry, root, stallingClass{cancel: cancel}))
	if len(report.Pending) != 1 || len(report.Backlog) == 0 {
		t.Fatalf("a stalled pass reported pending %v backlog %v", report.Pending, report.Backlog)
	}
}

// A stalled traversal leaves the store releasing within the budget, and the
// next pass finishes it (DL3B-04).
func TestStalledRemovalIsFinishedByTheNextPass(t *testing.T) {
	t.Parallel()
	root := realDir(t)
	registry := Registry{Dir: filepath.Join(root, "stores")}
	dead := plainStore(t, registry, root, "dead", "dead")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	proof := &fakeProof{kind: OwnerProcess, blockRemoval: true, cancel: cancel}
	class := RegisteredStores{Registry: registry, Proofs: map[OwnerKind]OwnerProof{OwnerProcess: proof}}
	report, err := RunPass(ctx, passOptions(registry, root, class))
	if err != nil {
		t.Fatal(err)
	}
	if record, _ := registry.Load(dead.ID); record.State != StateReleasing {
		t.Fatalf("a cut-short removal left the record %s, want releasing (report %+v)", record.State, report.Pending)
	}
	if _, err := os.Stat(filepath.Join(dead.Path, MarkerName)); err != nil {
		t.Fatalf("the marker went before the rest: %v", err)
	}
	proof.blockRemoval = false
	if _, err := RunPass(context.Background(), passOptions(registry, root, class)); err != nil {
		t.Fatal(err)
	}
	if record, _ := registry.Load(dead.ID); record.State != StateReleased {
		t.Fatalf("the next pass left %s", record.State)
	}
	if _, err := os.Stat(dead.Path); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("the next pass did not finish the removal: %v", err)
	}
}

// fakeCensus serves a fixed process table.
func fakeCensus(uses map[int64]identity.ProcessUse, gaps map[int64]bool) *CensusReader {
	var pids []int64
	for pid := range uses {
		pids = append(pids, pid)
	}
	for pid := range gaps {
		pids = append(pids, pid)
	}
	return &CensusReader{
		UID:        501,
		Pids:       func() ([]int64, error) { return pids, nil },
		ProcessUID: func(int64) (uint32, bool) { return 501, true },
		Use: func(pid int64) (identity.ProcessUse, error) {
			if gaps[pid] {
				return identity.ProcessUse{}, errors.New("descriptor list unreadable")
			}
			return uses[pid], nil
		},
		Command: func(pid int64) string { return fmt.Sprintf("fixture-%d", pid) },
	}
}

func worktreeStore(t *testing.T, registry Registry, root, name string) Record {
	t.Helper()
	worktree := filepath.Join(root, name)
	gitdir := filepath.Join(root, "common", ".git", "worktrees", name)
	for _, dir := range []string{worktree, gitdir} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(worktree, ".git"), []byte("gitdir: "+gitdir+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	record, err := registry.Register(Registration{Path: worktree, Git: true, Class: "goal-worktree", Owner: Owner{Kind: OwnerGoal, Ref: "dead"},
		Lifetime: LifetimeOwner, CapKind: CapNone}, testNow, rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := registry.Accept(record.ID); err != nil {
		t.Fatal(err)
	}
	return record
}

// gitProof releases a dead goal's worktree and records the removal instead
// of running git.
type gitProof struct{ removed []string }

func (*gitProof) Kind() OwnerKind { return OwnerGoal }
func (*gitProof) Observe(_ context.Context, record Record) Verdict {
	if record.Owner.Ref == "dead" {
		return Verdict{Decision: Release, Reason: "goal concluded, clean and landed"}
	}
	return Verdict{Decision: Keep, Reason: "goal open"}
}
func (p *gitProof) Apply(_ context.Context, critical *Critical) error {
	p.removed = append(p.removed, critical.Record().Path)
	return nil
}

// The checkout-use proof (DL3B-01): a process with its cwd elsewhere and
// one open file inside a worktree keeps it, naming the pid; an unreadable
// pid keeps every worktree store and names it with the --release route; a
// process started after the census is read inside the critical section.
func TestUseCensusKeepsWorktreesInUse(t *testing.T) {
	t.Parallel()
	root := realDir(t)
	registry := Registry{Dir: filepath.Join(root, "stores")}
	held := worktreeStore(t, registry, root, "held")
	free := worktreeStore(t, registry, root, "free")
	proof := &gitProof{}
	class := RegisteredStores{Registry: registry, Proofs: map[OwnerKind]OwnerProof{OwnerGoal: proof}}
	elsewhere := identity.ProcessUse{Cwd: "/elsewhere", Executable: "/usr/bin/editor", Files: []string{filepath.Join(held.Path, "notes.txt")}}

	options := passOptions(registry, root, class)
	options.Mode = ModeReport
	options.CensusReader = fakeCensus(map[int64]identity.ProcessUse{4242: elsewhere}, map[int64]bool{7: true})
	report, err := RunPass(context.Background(), options)
	if err != nil {
		t.Fatal(err)
	}
	for _, record := range []Record{held, free} {
		line := lineMap(report.Pending)[record.Path]
		if !strings.Contains(line.Reason, "pid 7") || line.Command != "metasystem disk clean --release "+record.ID {
			t.Fatalf("incomplete census for %s = %+v", record.Path, line)
		}
	}

	options.Mode = ModeApply
	options.CensusReader = fakeCensus(map[int64]identity.ProcessUse{4242: elsewhere}, nil)
	report, err = RunPass(context.Background(), options)
	if err != nil {
		t.Fatal(err)
	}
	if line := lineMap(report.Kept)[held.Path]; !strings.Contains(line.Reason, "pid 4242") || !strings.Contains(line.Command, "--release "+held.ID) {
		t.Fatalf("a worktree with an open file inside = %+v", report.Kept)
	}
	if len(proof.removed) != 1 || proof.removed[0] != free.Path {
		t.Fatalf("removed %v, want only %s", proof.removed, free.Path)
	}

	// A process that starts after the census inside the next candidate is
	// read in the critical section, and keeps it.
	late := worktreeStore(t, registry, root, "late")
	uses := map[int64]identity.ProcessUse{4242: elsewhere}
	reader := fakeCensus(uses, nil)
	pids := []int64{4242}
	reader.Pids = func() ([]int64, error) { return pids, nil }
	options.CensusReader = reader
	lateClass := lateStarter{RegisteredStores: class, start: func() {
		uses[5151] = identity.ProcessUse{Cwd: late.Path, Executable: "/bin/sh"}
		pids = append(pids, 5151)
	}}
	options.Classes = []Class{lateClass}
	report, err = RunPass(context.Background(), options)
	if err != nil {
		t.Fatal(err)
	}
	if line := lineMap(report.Kept)[late.Path]; !strings.Contains(line.Reason, "pid 5151") {
		t.Fatalf("a process started after the census = kept %+v pending %+v", report.Kept, report.Pending)
	}
	if len(proof.removed) != 1 {
		t.Fatalf("the late store was removed: %v", proof.removed)
	}
}

// lateStarter starts a process between the plan (which took the census) and
// the apply.
type lateStarter struct {
	RegisteredStores
	start func()
}

func (l lateStarter) Apply(ctx context.Context, pass *Pass, item Item) Verdict {
	l.start()
	return l.RegisteredStores.Apply(ctx, pass, item)
}

// Floor mode (3.3, R8, the 2026-09-29 amendment): below the floor the pass
// lowers ageing to the minimum (never zero), calls the trimmer once at half
// caps, raises the disk role with its remedy, and names the largest
// unregistered consumers with the command that reclaims each, deleting
// nothing.
func TestFloorModeNamesConsumersAndDeletesNothing(t *testing.T) {
	t.Parallel()
	root := realDir(t)
	registry := Registry{Dir: filepath.Join(root, "stores")}
	evidence := filepath.Join(root, "evidence")
	temp := filepath.Join(root, "tmp")
	for name, size := range map[string]int{filepath.Join(evidence, "big"): 64 << 10, filepath.Join(evidence, "small"): 1 << 10, filepath.Join(temp, "metasystem-bed"): 8 << 10} {
		if err := os.MkdirAll(name, 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(name, "data"), make([]byte, size), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	before := snapshotTree(t, root)
	trims := 0
	options := passOptions(registry, root)
	options.Volumes = []string{root}
	options.FloorBytes = 50 << 30
	options.FloorMinAge = 0
	options.Headroom = func(paths []string, floor int64) ([]VolumeFree, error) {
		return []VolumeFree{{Path: paths[0], FreeBytes: 277 << 20, FloorBytes: floor}}, nil
	}
	options.Trim = func(context.Context) (string, error) {
		trims++
		return "trimmed the engine cache to half its cap; keep window unchanged", nil
	}
	options.Consumers = func(ctx context.Context, census *UseCensus) []Consumer {
		return InventoryConsumers(ctx, testNow, []ConsumerRoot{{Path: evidence, Kind: "evidence root", Children: true},
			{Path: temp, Kind: "tmpdir", Children: true, Engine: true}}, nil, census)
	}
	report, err := RunPass(context.Background(), options)
	if err != nil {
		t.Fatal(err)
	}
	if report.Floor == nil || !report.Floor.Active || report.Floor.MinAge != "1h0m0s" || trims != 1 {
		t.Fatalf("floor = %+v after %d trims", report.Floor, trims)
	}
	if report.Health.Status != HealthAttention || report.Health.Remedy != "metasystem disk clean --preview" {
		t.Fatalf("health = %+v", report.Health)
	}
	consumers := report.Floor.Consumers
	if len(consumers) != 3 || consumers[0].Path != filepath.Join(evidence, "big") {
		t.Fatalf("consumers = %+v", consumers)
	}
	for _, consumer := range consumers {
		want := "rm -rf -- '" + consumer.Path + "'"
		if consumer.Kind == "tmpdir" {
			want = "metasystem disk clean --strays"
		}
		if !strings.Contains(consumer.Command, want) {
			t.Fatalf("consumer %s names %q, want %q", consumer.Path, consumer.Command, want)
		}
	}
	// Only the pass's own report and cursors were written.
	after := snapshotTree(t, root)
	for _, line := range strings.Split(after, "\n") {
		if !strings.Contains(before, line) && !strings.Contains(line, "report.json") && !strings.Contains(line, "stores") {
			t.Fatalf("floor mode changed %s", line)
		}
	}
	if len(before) == 0 || !strings.Contains(after, "/evidence/big/data") {
		t.Fatal("a consumer was deleted")
	}
}

// A preview writes exactly its plan: no report, no cursor, no record, no
// lock file in the checkout registry, and it releases nothing.
func TestPreviewWritesOnlyThePlan(t *testing.T) {
	t.Parallel()
	root := realDir(t)
	registry := Registry{Dir: filepath.Join(root, "stores")}
	dead := plainStore(t, registry, root, "dead", "dead")
	proof := &fakeProof{kind: OwnerProcess}
	options := passOptions(registry, root, RegisteredStores{Registry: registry, Proofs: map[OwnerKind]OwnerProof{OwnerProcess: proof}})
	options.Mode = ModePreview
	before := snapshotTree(t, root)
	report, err := RunPass(context.Background(), options)
	if err != nil {
		t.Fatal(err)
	}
	if report.Plan == "" || len(report.Planned) != 1 || len(proof.applied) != 0 {
		t.Fatalf("preview = plan %q planned %v applied %v", report.Plan, report.Planned, proof.applied)
	}
	after := snapshotTree(t, root)
	var added []string
	for _, line := range strings.Split(after, "\n") {
		if !strings.Contains(before, line) {
			added = append(added, line)
		}
	}
	if len(added) != 2 || !strings.HasPrefix(added[0], "/plans ") || !strings.HasPrefix(added[1], "/plans/"+report.Plan+".json ") {
		t.Fatalf("a preview changed more than its plan: %v", added)
	}
	if _, err := os.Stat(dead.Path); err != nil {
		t.Fatal("a preview removed a store")
	}
	plan, err := ReadPlan(options.PlanDir, report.Plan)
	if err != nil || len(plan.Items) != 1 || plan.Items[0].Record != dead.ID || plan.Items[0].Inode == 0 {
		t.Fatalf("plan = %+v, %v", plan, err)
	}
}
