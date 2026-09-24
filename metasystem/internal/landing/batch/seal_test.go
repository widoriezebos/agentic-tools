package batch

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"golang.org/x/sys/unix"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/gittree"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testpolicy"
)

type assemblyBed struct {
	root, base, moved string
	record            Record
}

func bedGit(t *testing.T, root string, args ...string) string {
	command := exec.Command("git", append([]string{"-C", root}, args...)...)
	command.Env = gittree.ScrubbedEnviron()
	output, err := command.Output()
	must(t, err)
	return strings.TrimSpace(string(output))
}
func goalBed(id string) []byte {
	return goal.RenderFile(&goal.GoalFile{Id: id, State: goal.StateClaimed, Intent: "land " + id, Origin: goal.OriginHuman, OpenedAt: "2026-09-17T10:00:00Z", Revision: 2, Claimed: &goal.ClaimRecord{Machine: "landing", Lineage: "owner", At: "2026-09-17T10:00:00Z", Revision: 2, AccountingRevision: 1, HandedOver: goal.HandedOver{FromMachine: "seat", FromLineage: id, FromEpoch: 1, Batch: testBatchID}}, History: []goal.HistoryLine{{At: "2026-09-17T10:00:00Z", Opid: "01J5X0000000000000000000BY-human-1a2b3c4d", Verb: "open", Actor: "human:wido", Keep: -1}, {At: "2026-09-17T10:00:00Z", Opid: "01J5X0000000000000000000B1-landing-1a2b3c4d", Verb: "claim", Actor: "landing+owner", Keep: -1}}})
}
func assemblyFixture(t *testing.T) assemblyBed {
	root := t.TempDir()
	must(t, os.MkdirAll(root+"/plans/goals", 0o755))
	for _, id := range []string{"goal-a", "goal-b", "goal-c"} {
		must(t, os.WriteFile(root+"/plans/goals/"+id+".md", goalBed(id), 0o644))
	}
	script := `set -eu; cd "$1"; git init -q -b main; git config user.name Test; git config user.email test@example.com; printf 'package p\nvar A = 0\n' >a.go; printf 'package p\nvar B = 0\n' >b.go; printf 'package p\nvar C = 0\n' >c.go; printf 'base\n' >trunk; git add .; git commit -qm base
base=$(git rev-parse HEAD); mkdir -p artifacts/agents/landing-batches/chains/{chain-a,chain-b,chain-c,conflict}; printf 'package p\nvar A = 1\n' >a.go; git diff --binary HEAD >artifacts/agents/landing-batches/chains/chain-a/diff.patch; git add a.go; git commit -qm chain-a; printf 'package p\nvar B = 1\n' >b.go; git diff --binary HEAD >artifacts/agents/landing-batches/chains/chain-b/diff.patch; git add b.go; git commit -qm chain-b; printf 'package p\nvar C = 1\n' >c.go; git diff --binary HEAD >artifacts/agents/landing-batches/chains/chain-c/diff.patch; git add c.go; git commit -qm chain-c
git reset -q --hard "$base"; printf 'package p\nvar A = 2\n' >a.go; printf 'package p\nvar B = 2\n' >b.go; git diff --binary HEAD >artifacts/agents/landing-batches/chains/conflict/diff.patch; git reset -q --hard "$base"; printf 'moved\n' >trunk; git add trunk; git commit -qm moved`
	command := exec.Command("bash", "-c", script, "ba4-fixture", root)
	command.Env = gittree.ScrubbedEnviron()
	must(t, command.Run())
	base := bedGit(t, root, "rev-parse", "HEAD~1^{tree}")
	moved := bedGit(t, root, "rev-parse", "HEAD^{tree}")
	units := []Unit{
		{GoalID: "goal-a", Chain: "chain-a", Claim: Claim{Machine: "seat", Lineage: "l", Epoch: 1, Revision: 7, AccountingRevision: 5}, State: UnitJoined},
		{GoalID: "goal-b", Chain: "chain-b", Claim: Claim{Machine: "seat", Lineage: "l", Epoch: 1, Revision: 8, AccountingRevision: 6}, State: UnitJoined},
	}
	return assemblyBed{root: root, base: base, moved: moved, record: Record{Schema: 1, BatchID: testBatchID, TipTree: base, State: StateOpen, Units: units, batchRecordFields: batchRecordFields{BaseTree: base}}}
}

// sealPolicyFixture keeps the record and flock real while declaring every
// repository fact Seal may request.
type sealPolicyFixture struct {
	bed               assemblyBed
	store             Store
	first, tip, third string
}

func witness(t *testing.T, ok bool, format string, args ...any) {
	t.Helper()
	if !ok {
		t.Fatalf(format, args...)
	}
}

func newSealPolicyFixture(t *testing.T, nested bool, goals ...string) sealPolicyFixture {
	t.Helper()
	root := t.TempDir()
	if nested {
		module := filepath.Join(root, "metasystem")
		must(t, os.Mkdir(module, 0o755))
		must(t, os.WriteFile(filepath.Join(module, "go.mod"), []byte("module example.invalid/metasystem\n\ngo 1.27\n"), 0o644))
	}
	if len(goals) == 0 {
		goals = []string{"goal-a", "goal-b"}
	}
	base, moved := testCommit(101), testCommit(102)
	units := make([]Unit, 0, len(goals))
	for index, id := range goals {
		units = append(units, Unit{GoalID: id, Chain: "chain-" + strings.TrimPrefix(id, "goal-"),
			Claim: Claim{Machine: "seat", Lineage: "l", Epoch: 1, Revision: uint64(7 + index), AccountingRevision: uint64(5 + index)}, State: UnitJoined})
	}
	bed := assemblyBed{root: root, base: base, moved: moved, record: Record{Schema: 1, BatchID: testBatchID,
		BaseTree: base, TipTree: base, State: StateOpen, Units: units}}
	store := NewStore(root, nil)
	must(t, store.Create(bed.record))
	return sealPolicyFixture{bed: bed, store: store, first: testCommit(103), tip: testCommit(104), third: testCommit(105)}
}

func (f *sealPolicyFixture) expectAssembly(t *testing.T, base string, goals, chains, prefixes []string) {
	t.Helper()
	expected := []expectedReassembly{expectedAssembly(base, goals, chains, prefixes)}
	for index := range goals {
		boundary := base
		if index > 0 {
			boundary = prefixes[index-1]
		}
		expected = append(expected, expectedAssembly(boundary, goals[index:index+1], chains[index:index+1], prefixes[index:index+1]))
	}
	strictReassembly(t, &f.store, expected...)
}

func (f *sealPolicyFixture) expectTwo(t *testing.T, base string) {
	t.Helper()
	f.expectAssembly(t, base, []string{"goal-a", "goal-b"}, []string{"chain-a", "chain-b"}, []string{f.first, f.tip})
}

type committedGoalReply struct {
	module, tree, goal string
	data               []byte
	present            bool
	err                error
}

func expectedGoal(f sealPolicyFixture, goal string) committedGoalReply {
	return committedGoalReply{module: ModuleRoot(f.bed.root), tree: f.tip, goal: goal, data: goalBed(goal), present: true}
}

func expectCommittedGoals(t *testing.T, store *Store, replies ...committedGoalReply) {
	t.Helper()
	var mu sync.Mutex
	called := 0
	store.committedGoal = func(module, tree, goal string) ([]byte, bool, error) {
		mu.Lock()
		defer mu.Unlock()
		if called >= len(replies) {
			t.Errorf("unexpected committed goal read: module=%q tree=%q goal=%q", module, tree, goal)
			return nil, false, fmt.Errorf("unexpected committed goal read")
		}
		want := replies[called]
		called++
		if module != want.module || tree != want.tree || goal != want.goal {
			t.Errorf("committed goal read %d: got module=%q tree=%q goal=%q, want %+v", called, module, tree, goal, want)
			return nil, false, fmt.Errorf("unexpected committed goal read")
		}
		return bytes.Clone(want.data), want.present, want.err
	}
	t.Cleanup(func() {
		mu.Lock()
		defer mu.Unlock()
		if called != len(replies) {
			t.Errorf("committed goal reads=%d, want %d", called, len(replies))
		}
	})
}

func expectValidClaims(t *testing.T, f *sealPolicyFixture, goals ...string) {
	t.Helper()
	replies := make([]committedGoalReply, 0, len(goals))
	for _, id := range goals {
		replies = append(replies, expectedGoal(*f, id))
	}
	expectCommittedGoals(t, &f.store, replies...)
}

func standardSealPlan(root string) func(string, string, string) (testpolicy.Plan, error) {
	return func(gotRoot, goalID, _ string) (testpolicy.Plan, error) {
		if gotRoot != root {
			return testpolicy.Plan{}, fmt.Errorf("planner root=%q, want %q", gotRoot, root)
		}
		return testpolicy.Plan{RequiredMode: testpolicy.ModeStandard, SelectedGroups: []string{"shared", goalID}}, nil
	}
}

func TestBatchSealFreezesMembership(t *testing.T) {
	f := newSealPolicyFixture(t, false)
	f.expectTwo(t, f.bed.moved)
	expectValidClaims(t, &f, "goal-a", "goal-b")
	must(t, Seal(f.store, testBatchID, f.bed.moved, "owner", time.Unix(1, 0), standardSealPlan(f.bed.root)))
	err := checkMembership(f.store, testBatchID, "goal-c", "chain-c")
	if err == nil || !strings.Contains(err.Error(), "BATCH_SEALED") {
		t.Fatalf("join refusal=%v", err)
	}
}

func TestBatchJoinConflictNamesFiles(t *testing.T) {
	bed := assemblyFixture(t)
	_, err := assembleUnits(bed.root, bed.base, append(bed.record.Units, Unit{GoalID: "goal-conflict", Chain: "conflict", State: UnitJoined}))
	if err == nil || !strings.Contains(err.Error(), "goal-conflict") || !strings.Contains(err.Error(), "a.go") || !strings.Contains(err.Error(), "b.go") {
		t.Fatalf("certified patch conflict=%v", err)
	}
}

func TestBatchSealAssemblyPreservesMovedTrunkContent(t *testing.T) {
	bed := assemblyFixture(t)
	prefixes, err := assembleUnits(bed.root, bed.moved, bed.record.Units)
	must(t, err)
	if len(prefixes) != 2 || bedGit(t, bed.root, "show", prefixes[1]+":trunk") != "moved" {
		t.Fatalf("moved trunk missing from composed tree: %v", prefixes)
	}
}

func TestBatchSealRegates(t *testing.T) {
	f := newSealPolicyFixture(t, false)
	f.expectTwo(t, f.bed.moved)
	expectValidClaims(t, &f, "goal-a", "goal-b")
	var trees []string
	plan := func(root, goalID, tree string) (testpolicy.Plan, error) {
		if root != f.bed.root {
			t.Fatalf("planner root=%q", root)
		}
		trees = append(trees, tree)
		return testpolicy.Plan{SelectedGroups: []string{"shared", goalID}}, nil
	}
	must(t, Seal(f.store, testBatchID, f.bed.moved, "owner", time.Unix(1, 0), plan))
	record := load(t, f.store)
	if record.BaseTree != f.bed.moved || record.TipTree != f.tip || !slices.Equal(record.PrefixTrees, []string{f.first, f.tip}) || !slices.Equal(trees, []string{f.tip, f.tip}) ||
		!slices.Equal(record.SelectedGroups, []string{"goal-a", "goal-b", "shared"}) {
		t.Fatalf("moved assembly=%+v planner trees=%v", record, trees)
	}
	red := newSealPolicyFixture(t, false)
	red.expectTwo(t, red.bed.base)
	expectCommittedGoals(t, &red.store)
	err := Seal(red.store, testBatchID, red.bed.base, "owner", time.Unix(1, 0), func(string, string, string) (testpolicy.Plan, error) {
		return testpolicy.Plan{}, errors.New("selection red")
	})
	if err == nil || err.Error() != "selection red" || load(t, red.store).State != StateOpen {
		t.Fatalf("selection refusal=%v", err)
	}
}

func TestBatchSelectionUnionClosesAtCeiling(t *testing.T) {
	open := Record{BatchID: testBatchID, State: StateOpen}
	recordSelection(&open, testpolicy.Plan{RequiredMode: testpolicy.ModeStandard, SelectedGroups: []string{"b", "a"}})
	recordSelection(&open, testpolicy.Plan{RequiredMode: testpolicy.ModeDeep, SelectedGroups: []string{"c", "a"}})
	if !slices.Equal(open.SelectedGroups, []string{"a", "b", "c"}) || open.ClosedReason != "deep-ceiling" {
		t.Fatalf("groups=%v closure=%q", open.SelectedGroups, open.ClosedReason)
	}
	err := joinRefusal(open)
	if err == nil || !strings.Contains(err.Error(), "BATCH_CLOSED") {
		t.Fatalf("join refusal=%v", err)
	}
}

func TestBatchSealExecutesChangedInputs(t *testing.T) {
	f := newSealPolicyFixture(t, false)
	f.expectTwo(t, f.bed.moved)
	expectValidClaims(t, &f, "goal-a", "goal-b")
	type request struct{ root, goal, tree string }
	want := []request{{f.bed.root, "goal-a", f.tip}, {f.bed.root, "goal-b", f.tip}, {f.bed.root, "goal-b", testCommit(106)}}
	called := 0
	plan := func(root, goal, tree string) (testpolicy.Plan, error) {
		got := request{root, goal, tree}
		if called >= len(want) || got != want[called] {
			t.Fatalf("planner request %d=%+v, want %v", called, got, want)
		}
		called++
		decision := "execute:G"
		if tree == testCommit(106) {
			decision = "reused:G"
		}
		return testpolicy.Plan{SelectedGroups: []string{decision}}, nil
	}
	must(t, Seal(f.store, testBatchID, f.bed.moved, "owner", time.Unix(1, 0), plan))
	record := load(t, f.store)
	reuse, err := plan(f.bed.root, "goal-b", testCommit(106))
	must(t, err)
	if called != len(want) || !slices.Contains(record.SelectedGroups, "execute:G") || !slices.Contains(reuse.SelectedGroups, "reused:G") {
		t.Fatalf("planner calls=%d tip groups=%v standalone=%v", called, record.SelectedGroups, reuse.SelectedGroups)
	}
}

func TestBatchSealDryRunsTheBoundary(t *testing.T) {
	f := newSealPolicyFixture(t, false)
	f.expectTwo(t, f.bed.moved)
	expectValidClaims(t, &f, "goal-a", "goal-b")
	must(t, Seal(f.store, testBatchID, f.bed.moved, "owner", time.Unix(1, 0), standardSealPlan(f.bed.root)))
	record := load(t, f.store)
	if !slices.Equal(record.PrefixTrees, []string{f.first, f.tip}) {
		t.Fatalf("prefixes=%v", record.PrefixTrees)
	}
	claim := record.Seal["goal-a"]
	if claim.Revision != 2 || claim.AccountingRevision != 1 || claim.Machine != "" || claim.Lineage != "" {
		t.Fatalf("sealed claim=%+v", claim)
	}
}

func TestBatchSealReadsClaimsFromTheNestedModuleWithTheStoreAtTheRepositoryTop(t *testing.T) {
	t.Parallel()
	f := newSealPolicyFixture(t, true)
	f.expectTwo(t, f.bed.moved)
	expectValidClaims(t, &f, "goal-a", "goal-b")
	must(t, Seal(f.store, testBatchID, f.bed.moved, "owner", time.Unix(1, 0), standardSealPlan(f.bed.root)))
	for _, id := range []string{"goal-a", "goal-b"} {
		claim := load(t, f.store).Seal[id]
		if claim.Revision != 2 || claim.AccountingRevision != 1 {
			t.Fatalf("sealed claim for %s=%+v", id, claim)
		}
	}
}

func TestBatchSealExcludesReturnedUnits(t *testing.T) {
	f := newSealPolicyFixture(t, false, "goal-a", "goal-b", "goal-c")
	must(t, RequestReturn(f.store, testBatchID, "goal-b", UnitEjected, "red", "owner", time.Unix(2, 0)))
	must(t, f.store.locked(func() error {
		return settleReturn(f.store, testBatchID, "goal-b", ReturnHandedBack, "owner", time.Unix(3, 0))
	}))
	f.expectAssembly(t, f.bed.base, []string{"goal-a", "goal-c"}, []string{"chain-a", "chain-c"}, []string{f.first, f.tip})
	expectValidClaims(t, &f, "goal-a", "goal-c")
	must(t, Seal(f.store, testBatchID, f.bed.base, "owner", time.Unix(4, 0), standardSealPlan(f.bed.root)))
	record := load(t, f.store)
	if !slices.Equal(record.PrefixTrees, []string{f.first, f.tip}) || record.TipTree != f.tip {
		t.Fatalf("survivor series=%+v", record)
	}
	if _, sealed := record.Seal["goal-b"]; sealed {
		t.Fatalf("returned member remained sealed: %+v", record.Seal)
	}
}

func appendThirdJoined(t *testing.T, store Store, f sealPolicyFixture) {
	t.Helper()
	must(t, store.Update(testBatchID, func(record *Record) error {
		unit := joiningUnit("goal-c", "chain-c")
		unit.State = UnitJoined
		unit.Admission = &JoinAdmission{Tree: testCommit(106), Status: "verified"}
		unit.SelectedGroups = []string{"goal-c"}
		record.Units = append(record.Units, unit)
		record.PrefixTrees = []string{f.first, f.tip, f.third}
		record.TipTree = f.third
		record.SelectedGroups = []string{"goal-c"}
		appendUnitHistory(record, time.Unix(2, 0), "join", "seat+goal-c", "goal-c", "", UnitJoining)
		appendUnitHistory(record, time.Unix(2, 0), "join", "seat+goal-c", "goal-c", UnitJoining, UnitJoined)
		return nil
	}))
}

func TestBatchSealReleasesLockDuringSelectionAndRefusesChangedCandidate(t *testing.T) {
	t.Parallel()
	f := newSealPolicyFixture(t, false)
	f.expectTwo(t, f.bed.base)
	expectValidClaims(t, &f, "goal-a", "goal-b")
	var lockDepth atomic.Int32
	f.store.seams.flock = func(fd, operation int) error {
		if operation == unix.LOCK_UN {
			lockDepth.Add(-1)
			return unix.Flock(fd, operation)
		}
		if err := unix.Flock(fd, operation); err != nil {
			return err
		}
		lockDepth.Add(1)
		return nil
	}
	planStarted, releasePlan := make(chan struct{}), make(chan struct{})
	var once, held atomic.Bool
	plan := func(_, goalID, _ string) (testpolicy.Plan, error) {
		if once.CompareAndSwap(false, true) {
			held.Store(lockDepth.Load() != 0)
			close(planStarted)
			<-releasePlan
		}
		return testpolicy.Plan{SelectedGroups: []string{goalID}}, nil
	}
	sealDone := make(chan error, 1)
	go func() { sealDone <- Seal(f.store, testBatchID, f.bed.base, "owner", time.Unix(3, 0), plan) }()
	select {
	case <-planStarted:
	case err := <-sealDone:
		t.Fatalf("seal returned before selection: %v", err)
	}
	if held.Load() {
		close(releasePlan)
		<-sealDone
		t.Fatal("selection held the batch flock")
	}
	joinStore := f.store
	joinStore.seams.flock = func(fd, operation int) error {
		if operation == unix.LOCK_EX {
			operation |= unix.LOCK_NB
		}
		return unix.Flock(fd, operation)
	}
	appendThirdJoined(t, joinStore, f)
	close(releasePlan)
	err := <-sealDone
	var changed *SealChangedDuringGateRefusal
	if !errors.As(err, &changed) || changed.BatchID != testBatchID || !strings.Contains(err.Error(), "changed during seal preparation") {
		t.Fatalf("changed-candidate refusal=%T %v", err, err)
	}
	record := load(t, f.store)
	if record.State != StateOpen || len(joinedUnits(record.Units)) != 3 || record.Units[2].Admission == nil || record.Units[2].Admission.Status != "verified" || record.Seal != nil || record.CostForecast != nil {
		t.Fatalf("stale state=%+v", record)
	}
	if want := []HistoryEntry{
		{At: time.Unix(2, 0).UTC().Format(time.RFC3339Nano), Verb: "join", From: "", To: UnitJoining, Actor: "seat+goal-c", Detail: "goal-c joining"},
		{At: time.Unix(2, 0).UTC().Format(time.RFC3339Nano), Verb: "join", From: UnitJoining, To: UnitJoined, Actor: "seat+goal-c", Detail: "goal-c joined"},
	}; !slices.Equal(record.History, want) {
		t.Fatalf("join history=%+v, want %+v", record.History, want)
	}
	unchanged := newSealPolicyFixture(t, false)
	unchanged.expectTwo(t, unchanged.bed.base)
	expectValidClaims(t, &unchanged, "goal-a", "goal-b")
	must(t, Seal(unchanged.store, testBatchID, unchanged.bed.base, "owner", time.Unix(4, 0), plan))
	if sealed := load(t, unchanged.store); sealed.State != StateSealed || len(sealed.Seal) != 2 {
		t.Fatalf("unchanged candidate=%+v", sealed)
	}
}

func TestBatchSealReleasesLockDuringGateAndRefusesChangedCandidate(t *testing.T) {
	t.Parallel()
	f := newSealPolicyFixture(t, false)
	f.expectTwo(t, f.bed.base)
	expectValidClaims(t, &f, "goal-a", "goal-b")
	joinStore := f.store
	joinStore.seams.flock = func(fd, operation int) error {
		if operation == unix.LOCK_EX {
			operation |= unix.LOCK_NB
		}
		return unix.Flock(fd, operation)
	}
	sealErr := SealWithForecast(f.store, testBatchID, f.bed.base, "owner", time.Unix(3, 0), standardSealPlan(f.bed.root), func(candidate Record) (CostForecast, error) {
		if !slices.Equal(candidate.PrefixTrees, []string{f.first, f.tip}) || candidate.TipTree != f.tip {
			t.Errorf("forecast candidate=%+v", candidate)
		}
		appendThirdJoined(t, joinStore, f)
		return CostForecast{SchemaVersion: 1, Binding: CostBinding(candidate, candidate.Units, candidate.PrefixTrees)}, nil
	})
	var changed *SealChangedDuringGateRefusal
	if !errors.As(sealErr, &changed) || changed.BatchID != testBatchID {
		t.Fatalf("changed candidate result=%v", sealErr)
	}
	record := load(t, f.store)
	if record.State != StateOpen || len(joinedUnits(record.Units)) != 3 || record.Units[2].Admission == nil || record.Units[2].Admission.Status != "verified" || record.Seal != nil || record.CostForecast != nil {
		t.Fatalf("stale seal or forecast=%+v", record)
	}
	if want := []HistoryEntry{
		{At: time.Unix(2, 0).UTC().Format(time.RFC3339Nano), Verb: "join", From: "", To: UnitJoining, Actor: "seat+goal-c", Detail: "goal-c joining"},
		{At: time.Unix(2, 0).UTC().Format(time.RFC3339Nano), Verb: "join", From: UnitJoining, To: UnitJoined, Actor: "seat+goal-c", Detail: "goal-c joined"},
	}; !slices.Equal(record.History, want) {
		t.Fatalf("join history=%+v, want %+v", record.History, want)
	}
}

func TestBatchSealRejectsAbsentCommittedGoal(t *testing.T) {
	assertSealCommittedRefusal(t, committedGoalReply{present: false}, "absent from tree")
}
func TestBatchSealRejectsMalformedCommittedClaim(t *testing.T) {
	data := []byte("malformed claim\n")
	if _, problems := goal.ParseFile(data); len(problems) == 0 {
		t.Fatal("malformed fixture parsed as a valid goal")
	}
	assertSealCommittedRefusal(t, committedGoalReply{data: data, present: true}, "has no valid handed-over claim")
}
func TestBatchSealRejectsWrongBatchCommittedClaim(t *testing.T) {
	otherBatch := "01J5X0000000000000000000ZZ"
	file, problems := goal.ParseFile(goalBed("goal-a"))
	if len(problems) != 0 || file.Claimed == nil {
		t.Fatalf("source goal fixture is invalid: %v", problems)
	}
	file.Claimed.HandedOver.Batch = otherBatch
	data := goal.RenderFile(file)
	file, problems = goal.ParseFile(data)
	if len(problems) != 0 || file.Claimed == nil || file.Claimed.HandedOver.Batch != otherBatch {
		t.Fatalf("wrong-batch fixture is not a valid handed-over claim: problems=%v claim=%+v", problems, file.Claimed)
	}
	assertSealCommittedRefusal(t, committedGoalReply{data: data, present: true}, "has no valid handed-over claim")
}
func assertSealCommittedRefusal(t *testing.T, reply committedGoalReply, reason string) {
	t.Helper()
	f := newSealPolicyFixture(t, false)
	f.expectTwo(t, f.bed.base)
	reply.module, reply.tree, reply.goal = ModuleRoot(f.bed.root), f.tip, "goal-a"
	expectCommittedGoals(t, &f.store, reply)
	before := load(t, f.store)
	err := Seal(f.store, testBatchID, f.bed.base, "owner", time.Unix(3, 0), standardSealPlan(f.bed.root))
	want := fmt.Errorf("goal ledger entry %s is absent from tree %s: %w", reply.goal, reply.tree, reply.err).Error()
	if reply.present && reply.err == nil {
		_, problems := goal.ParseFile(reply.data)
		want = fmt.Sprintf("goal ledger entry %s has no valid handed-over claim for batch %s: %v", reply.goal, testBatchID, problems)
	}
	if err == nil || err.Error() != want || !strings.Contains(err.Error(), reason) {
		t.Fatalf("committed claim refusal=%v, want %q", err, want)
	}
	after := load(t, f.store)
	if !reflect.DeepEqual(after, before) || after.State != StateOpen || after.Seal != nil {
		t.Fatalf("refusal changed durable record: before=%+v after=%+v", before, after)
	}
}
