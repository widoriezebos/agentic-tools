package batch

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/gittree"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testpolicy"
)

// Seal re-derives the batch at the fetched base, dry-runs every prefix
// boundary, and freezes member selections and revisions before proof admission.
func Seal(store Store, id, baseTree, owner string, at time.Time, plan func(string, string, string) (testpolicy.Plan, error), gate GateExecutor) error {
	var snapshot Record
	if err := store.locked(func() error {
		record, err := store.Load(id)
		if err != nil {
			return err
		}
		if err := sealableRecord(record); err != nil {
			return err
		}
		snapshot = record
		return nil
	}); err != nil {
		return err
	}
	revision := sealRevisionOf(snapshot)
	candidate := snapshot
	units, err := prepareSealCandidate(store.root, baseTree, &candidate, assembleUnits)
	if err != nil {
		return err
	}
	gateErr := runSealGate(store.root, candidate.TipTree, units, gate)
	if gateErr == nil {
		gateErr = selectSealCandidate(store.root, &candidate, plan)
	}
	return store.Update(id, func(current *Record) error {
		if !reflect.DeepEqual(revision, sealRevisionOf(*current)) {
			return &SealChangedDuringGateRefusal{BatchID: id}
		}
		if gateErr != nil {
			return gateErr
		}
		current.BaseTree = candidate.BaseTree
		current.PrefixTrees = slices.Clone(candidate.PrefixTrees)
		current.TipTree = candidate.TipTree
		current.SelectedGroups = slices.Clone(candidate.SelectedGroups)
		current.ClosedReason = candidate.ClosedReason
		current.Seal = candidate.Seal
		current.Transition(StateSealed, at, "seal", owner, "")
		return nil
	})
}

// SealChangedDuringGateRefusal means the gate ran on a candidate that is no
// longer the batch's candidate and therefore cannot seal it.
type SealChangedDuringGateRefusal struct{ BatchID string }

func (refusal *SealChangedDuringGateRefusal) Error() string {
	return fmt.Sprintf("BATCH_SEAL_CHANGED_DURING_GATE: batch %s changed during the gate", refusal.BatchID)
}

type sealMemberRevision struct {
	GoalID, State, Tree string
}

type sealRevision struct {
	BaseTree string
	Members  []sealMemberRevision
}

func sealRevisionOf(record Record) sealRevision {
	revision := sealRevision{BaseTree: record.BaseTree}
	prefix := 0
	for _, unit := range record.Units {
		tree := ""
		if unit.State == UnitJoining || unit.State == UnitJoined {
			if prefix < len(record.PrefixTrees) {
				tree = record.PrefixTrees[prefix]
			}
			prefix++
		}
		revision.Members = append(revision.Members, sealMemberRevision{GoalID: unit.GoalID, State: unit.State, Tree: tree})
	}
	return revision
}

func sealableRecord(record Record) error {
	if record.State != StateOpen {
		return fmt.Errorf("BATCH_PROOF_STATE_REFUSED: batch %s must be open before seal", record.BatchID)
	}
	if len(joinedUnits(record.Units)) == 0 {
		return fmt.Errorf("BATCH_PROOF_STATE_REFUSED: batch %s has no joined units", record.BatchID)
	}
	return nil
}

func sealBatch(root, baseTree, owner string, at time.Time, record *Record, plan func(string, string, string) (testpolicy.Plan, error), gate batchGateExec, assemble func(string, string, []Unit) ([]string, error)) error {
	units, err := prepareSealCandidate(root, baseTree, record, assemble)
	if err != nil {
		return err
	}
	if err := runSealGate(root, record.TipTree, units, gate); err != nil {
		return err
	}
	if err := selectSealCandidate(root, record, plan); err != nil {
		return err
	}
	record.Transition(StateSealed, at, "seal", owner, "")
	return nil
}

func prepareSealCandidate(root, baseTree string, record *Record, assemble func(string, string, []Unit) ([]string, error)) ([]Unit, error) {
	units := joinedUnits(record.Units)
	if len(units) == 0 {
		return nil, fmt.Errorf("BATCH_PROOF_STATE_REFUSED: batch %s has no joined units", record.BatchID)
	}
	prefixes, err := assemble(root, baseTree, units)
	if err != nil {
		return nil, err
	}
	for index, unit := range units {
		boundary := baseTree
		if index > 0 {
			boundary = prefixes[index-1]
		}
		check, checkErr := assemble(root, boundary, []Unit{unit})
		if checkErr != nil || len(check) != 1 || check[0] != prefixes[index] {
			return nil, fmt.Errorf("batch prefix boundary %s did not reproduce %s: %v", unit.GoalID, prefixes[index], checkErr)
		}
	}
	record.BaseTree, record.PrefixTrees = baseTree, prefixes
	record.TipTree = prefixes[len(prefixes)-1]
	return units, nil
}

func selectSealCandidate(root string, candidate *Record, plan func(string, string, string) (testpolicy.Plan, error)) error {
	candidate.SelectedGroups = nil
	candidate.Seal = map[string]Claim{}
	units := joinedUnits(candidate.Units)
	for _, unit := range units {
		selection, planErr := plan(root, unit.GoalID, candidate.TipTree)
		if planErr != nil {
			return planErr
		}
		recordSelection(candidate, selection)
		claim, claimErr := claimAt(root, candidate.TipTree, candidate.BatchID, unit.GoalID)
		if claimErr != nil {
			return claimErr
		}
		claim.Machine, claim.Lineage = "", ""
		candidate.Seal[unit.GoalID] = claim
	}
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
	changes := gateChanges{}
	for _, unit := range units {
		if len(unit.Builds) != 0 {
			member, _ := branchMemberOf(unit)
			patch, err := branchMemberPatch(root, member)
			if err != nil {
				return err
			}
			mergeGateChanges(changes, patchGateChanges(patch))
			continue
		}
		patch, err := os.ReadFile(filepath.Join(root, "artifacts", "agents", "landing-batches", "chains", unit.Chain, "diff.patch"))
		if err != nil {
			return err
		}
		mergeGateChanges(changes, patchGateChanges(patch))
	}
	steps := []gateStep{{Name: "fast gate", Args: []string{"bash", "scripts/agents/go-gate.sh", "--fast"}}}
	packages, err := changedGoPackages(root, tree, changes)
	if err != nil {
		return err
	}
	for _, pkg := range packages {
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
	return Claim{Machine: file.Claimed.HandedOver.FromMachine, Lineage: file.Claimed.HandedOver.FromLineage, Epoch: file.Claimed.HandedOver.FromEpoch, Revision: file.Claimed.Revision, AccountingRevision: file.Claimed.AccountingRevision}, nil
}

// ReadClaimAt exposes the sealed-tree claim reader to the production owner.
func ReadClaimAt(root, tree, batchID, goalID string) (Claim, error) {
	return claimAt(root, tree, batchID, goalID)
}
