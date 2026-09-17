package batch

import (
	"os"
	"os/exec"
	"slices"
	"strings"
	"testing"
	"time"

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
func sealedFixture(t *testing.T) (assemblyBed, Record, []string, int, func(string, string, string) (testpolicy.Plan, error)) {
	bed := assemblyFixture(t)
	record := bed.record
	var trees []string
	assemblies := 0
	plan := func(_, goalID, tree string) (testpolicy.Plan, error) {
		trees = append(trees, tree)
		decision := "reused:G"
		if strings.Contains(bedGit(t, bed.root, "show", tree+":a.go"), "A = 1") {
			decision = "execute:G"
		}
		return testpolicy.Plan{RequiredMode: testpolicy.ModeStandard, SelectedGroups: []string{"shared", goalID, decision}}, nil
	}
	gate := func(tree string, _ gateStep) gateStepResult {
		trees = append(trees, tree)
		return gateStepResult{RunID: "seal-green"}
	}
	assemble := func(root, base string, units []Unit) ([]string, error) {
		assemblies++
		return assembleUnits(root, base, units)
	}
	must(t, sealBatch(bed.root, bed.moved, "landing+owner", time.Unix(1, 0), &record, plan, gate, assemble))
	return bed, record, trees, assemblies, plan
}
func witness(t *testing.T, ok bool, format string, args ...any) {
	if !ok {
		t.Fatalf(format, args...)
	}
}
func sealWitness(t *testing.T, kind string) {
	bed, record, trees, assemblies, plan := sealedFixture(t)
	switch kind {
	case "freeze":
		store := NewStore(bed.root, scriptedProber{})
		must(t, store.Create(record))
		err := checkMembership(store, testBatchID, "goal-c", "chain-c")
		witness(t, err != nil && strings.Contains(err.Error(), "BATCH_SEALED"), "join refusal=%v", err)
	case "conflict":
		_, err := assembleUnits(bed.root, bed.base, append(bed.record.Units, Unit{GoalID: "goal-conflict", Chain: "conflict", State: UnitJoined}))
		witness(t, err != nil && strings.Contains(err.Error(), "goal-conflict") && strings.Contains(err.Error(), "a.go") && strings.Contains(err.Error(), "b.go"), "conflict=%v", err)
	case "regate":
		witness(t, record.BaseTree == bed.moved && record.TipTree != bed.record.TipTree && bedGit(t, bed.root, "show", record.TipTree+":trunk") == "moved" && len(trees) == 4, "moved assembly=%+v seam trees=%v", record, trees)
		red := bed.record
		err := sealBatch(bed.root, bed.base, "owner", time.Time{}, &red, func(string, string, string) (testpolicy.Plan, error) { return testpolicy.Plan{}, nil }, func(string, gateStep) gateStepResult { return gateStepResult{RunID: "red", ExitCode: 1} }, assembleUnits)
		witness(t, err != nil && strings.Contains(err.Error(), "BATCH_SEAL_GATE_RED"), "red gate=%v", err)
	case "ceiling":
		open := Record{BatchID: testBatchID, State: StateOpen}
		recordSelection(&open, testpolicy.Plan{RequiredMode: testpolicy.ModeStandard, SelectedGroups: []string{"b", "a"}})
		recordSelection(&open, testpolicy.Plan{RequiredMode: testpolicy.ModeDeep, SelectedGroups: []string{"c", "a"}})
		witness(t, slices.Equal(open.SelectedGroups, []string{"a", "b", "c"}), "groups=%v", open.SelectedGroups)
		err := joinRefusal(open)
		witness(t, err != nil && strings.Contains(err.Error(), "BATCH_CLOSED"), "join refusal=%v", err)
	case "inputs":
		witness(t, !slices.ContainsFunc(trees, func(tree string) bool { return tree != record.TipTree }), "tip=%s seam trees=%v", record.TipTree, trees)
		u, err := assembleUnits(bed.root, bed.base, []Unit{bed.record.Units[1]})
		must(t, err)
		reuse, err := plan(bed.root, "goal-b", u[0])
		must(t, err)
		witness(t, slices.Contains(record.SelectedGroups, "execute:G") && slices.Contains(reuse.SelectedGroups, "reused:G"), "tip groups=%v unit groups=%v", record.SelectedGroups, reuse.SelectedGroups)
	case "boundary":
		witness(t, assemblies == len(record.Units)+1 && len(record.PrefixTrees) == len(record.Units), "assembly calls=%d prefixes=%v", assemblies, record.PrefixTrees)
		claim := record.Seal["goal-a"]
		witness(t, claim.Revision == 2 && claim.AccountingRevision == 1, "sealed claims=%+v", record.Seal)
	}
}

func TestBatchSealFreezesMembership(t *testing.T)         { sealWitness(t, "freeze") }
func TestBatchJoinConflictNamesFiles(t *testing.T)        { sealWitness(t, "conflict") }
func TestBatchSealRegates(t *testing.T)                   { sealWitness(t, "regate") }
func TestBatchSelectionUnionClosesAtCeiling(t *testing.T) { sealWitness(t, "ceiling") }
func TestBatchSealExecutesChangedInputs(t *testing.T)     { sealWitness(t, "inputs") }
func TestBatchSealDryRunsTheBoundary(t *testing.T)        { sealWitness(t, "boundary") }
