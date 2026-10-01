package batch

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"slices"
	"strings"
	"sync"
	"testing"

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

func witness(t *testing.T, ok bool, format string, args ...any) {
	t.Helper()
	if !ok {
		t.Fatalf(format, args...)
	}
}

type committedGoalReply struct {
	module, tree, goal string
	data               []byte
	present            bool
	err                error
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

func TestBatchJoinConflictNamesFiles(t *testing.T) {
	bed := assemblyFixture(t)
	_, err := assembleUnits(bed.root, bed.base, append(bed.record.Units, Unit{GoalID: "goal-conflict", Chain: "conflict", State: UnitJoined}))
	if err == nil || !strings.Contains(err.Error(), "goal-conflict") || !strings.Contains(err.Error(), "a.go") || !strings.Contains(err.Error(), "b.go") {
		t.Fatalf("certified patch conflict=%v", err)
	}
}

func TestBatchSealAssemblyPreservesMovedTrunkContent(t *testing.T) {
	t.Parallel()
	bed := assemblyFixture(t)
	prefixes, err := assembleUnits(bed.root, bed.moved, bed.record.Units)
	must(t, err)
	if len(prefixes) != 2 || bedGit(t, bed.root, "show", prefixes[1]+":trunk") != "moved" {
		t.Fatalf("moved trunk missing from composed tree: %v", prefixes)
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
