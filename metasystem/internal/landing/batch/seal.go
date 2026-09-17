package batch

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/gittree"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testpolicy"
)

func sealBatch(root, baseTree, owner string, at time.Time, record *Record, plan func(string, string, string) (testpolicy.Plan, error), gate batchGateExec, assemble func(string, string, []Unit) ([]string, error)) error {
	prefixes, err := assemble(root, baseTree, record.Units)
	if err != nil {
		return err
	}
	for index, unit := range record.Units {
		boundary := baseTree
		if index > 0 {
			boundary = prefixes[index-1]
		}
		check, checkErr := assemble(root, boundary, []Unit{unit})
		if checkErr != nil || len(check) != 1 || check[0] != prefixes[index] {
			return fmt.Errorf("batch prefix boundary %s did not reproduce %s: %v", unit.GoalID, prefixes[index], checkErr)
		}
	}
	candidate := *record
	candidate.BaseTree, candidate.PrefixTrees = baseTree, prefixes
	candidate.TipTree = prefixes[len(prefixes)-1]
	if err := runSealGate(root, candidate.TipTree, record.Units, gate); err != nil {
		return err
	}
	candidate.SelectedGroups = nil
	candidate.Seal = map[string]Claim{}
	for _, unit := range record.Units {
		selection, planErr := plan(root, unit.GoalID, candidate.TipTree)
		if planErr != nil {
			return planErr
		}
		recordSelection(&candidate, selection)
		claim, claimErr := claimAt(root, candidate.TipTree, candidate.BatchID, unit.GoalID)
		if claimErr != nil {
			return claimErr
		}
		candidate.Seal[unit.GoalID] = claim
	}
	candidate.Transition(StateSealed, at, "seal", owner, "")
	*record = candidate
	return nil
}

func recordSelection(record *Record, plan testpolicy.Plan) {
	record.SelectedGroups = append(record.SelectedGroups, plan.SelectedGroups...)
	slices.Sort(record.SelectedGroups)
	record.SelectedGroups = slices.Compact(record.SelectedGroups)
	if plan.RequiredMode == testpolicy.ModeDeep {
		record.ClosedReason = "deep-ceiling"
	}
}

func runSealGate(root, tree string, units []Unit, execute batchGateExec) error {
	paths := patchChange{}
	for _, unit := range units {
		patch, err := os.ReadFile(filepath.Join(root, "artifacts", "agents", "landing-batches", "chains", unit.Chain, "diff.patch"))
		if err != nil {
			return err
		}
		for path, deleted := range patchChangedPaths(patch) {
			paths[path] = deleted
		}
	}
	steps := []gateStep{{Name: "fast gate", Args: []string{"bash", "scripts/agents/go-gate.sh", "--fast"}}}
	for _, pkg := range changedGoPackages(paths) {
		steps = append(steps, gateStep{Name: "package " + pkg, Args: []string{"go", "test", "-count=1", "-timeout", "900s", pkg}})
	}
	for _, step := range steps {
		result := execute(tree, step)
		if result.ExitCode != 0 || result.RunID == "" {
			return refuseBatch("BATCH_SEAL_GATE_RED", fmt.Sprintf("%s exited %d", step.Name, result.ExitCode))
		}
	}
	return nil
}

func claimAt(root, tree, batchID, goalID string) (Claim, error) {
	data, present, err := (gittree.Workspace{Dir: root}).FileAt(tree, filepath.ToSlash(filepath.Join("plans", "goals", goalID+".md")))
	if err != nil || !present {
		return Claim{}, fmt.Errorf("goal ledger entry %s is absent from tree %s: %w", goalID, tree, err)
	}
	file, problems := goal.ParseFile(data)
	if len(problems) != 0 || file.Claimed == nil || file.Claimed.HandedOver.Batch != batchID {
		return Claim{}, fmt.Errorf("goal ledger entry %s has no valid handed-over claim for batch %s: %v", goalID, batchID, problems)
	}
	return Claim{Revision: file.Claimed.Revision, AccountingRevision: file.Claimed.AccountingRevision}, nil
}
