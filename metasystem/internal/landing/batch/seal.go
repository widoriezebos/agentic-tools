package batch

import (
	"errors"
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

// ModuleRoot returns the nested MetaSystem module when a checkout contains one.
func ModuleRoot(checkout string) string {
	nested := filepath.Join(checkout, "metasystem")
	if info, err := os.Stat(filepath.Join(nested, "go.mod")); err == nil && !info.IsDir() {
		return nested
	}
	return checkout
}

// Seal re-derives the batch at the fetched base, dry-runs every prefix
// boundary, and freezes member selections and revisions before proof admission.
func Seal(store Store, id, baseTree, owner string, at time.Time, plan func(string, string, string) (testpolicy.Plan, error)) error {
	return SealWithForecast(store, id, baseTree, owner, at, plan, nil)
}

// SealWithForecast computes the bounded cost view on the exact prepared
// series outside the store lock. The same revision guard protects both the
// selection and its historical forecast before either is published.
func SealWithForecast(store Store, id, baseTree, owner string, at time.Time, plan func(string, string, string) (testpolicy.Plan, error),
	forecast func(Record) (CostForecast, error)) error {
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
	_, err := prepareSealCandidate(store.root, baseTree, &candidate, func(_ string, base string, units []Unit) ([]string, error) {
		return store.reassembly.assemble(base, units)
	})
	if err != nil {
		return err
	}
	selectionErr := selectSealCandidateWithReader(store.root, &candidate, plan, store.committedGoal)
	if selectionErr == nil && forecast != nil {
		var cost CostForecast
		cost, selectionErr = forecast(candidate)
		if selectionErr == nil {
			candidate.CostForecast = &cost
		}
	}
	return store.Update(id, func(current *Record) error {
		if !reflect.DeepEqual(revision, sealRevisionOf(*current)) {
			return &SealChangedDuringGateRefusal{BatchID: id}
		}
		if selectionErr != nil {
			return selectionErr
		}
		current.BaseTree = candidate.BaseTree
		current.PrefixTrees = slices.Clone(candidate.PrefixTrees)
		current.TipTree = candidate.TipTree
		current.SelectedGroups = slices.Clone(candidate.SelectedGroups)
		current.ClosedReason = candidate.ClosedReason
		current.Seal = candidate.Seal
		current.CostForecast = candidate.CostForecast
		current.Transition(StateSealed, at, "seal", owner, "")
		return nil
	})
}

// SealChangedDuringGateRefusal preserves the registered refusal code when
// composition or policy selection ran on a candidate that changed meanwhile.
type SealChangedDuringGateRefusal struct{ BatchID string }

func (refusal *SealChangedDuringGateRefusal) Error() string {
	return fmt.Sprintf("BATCH_SEAL_CHANGED_DURING_GATE: batch %s changed during seal preparation", refusal.BatchID)
}

type sealMemberRevision struct {
	GoalID, Chain, State, Tree string
	Claim                      Claim
	SelectedGroups             []string
}

type sealRevision struct {
	BaseTree, ClosedReason string
	SelectedGroups         []string
	Members                []sealMemberRevision
	PrefixEpisodes         []CostForecastEpisode
}

func sealRevisionOf(record Record) sealRevision {
	revision := sealRevision{BaseTree: record.BaseTree, ClosedReason: record.ClosedReason,
		SelectedGroups: slices.Clone(record.SelectedGroups)}
	prefix := 0
	for _, unit := range record.Units {
		tree := ""
		if unit.State == UnitJoining || unit.State == UnitJoined {
			if prefix < len(record.PrefixTrees) {
				tree = record.PrefixTrees[prefix]
			}
			prefix++
		}
		revision.Members = append(revision.Members, sealMemberRevision{GoalID: unit.GoalID, Chain: unit.Chain, State: unit.State,
			Tree: tree, Claim: unit.Claim, SelectedGroups: slices.Clone(unit.SelectedGroups)})
	}
	keys := make([]string, 0, len(record.PrefixEpisodes))
	for goalID := range record.PrefixEpisodes {
		keys = append(keys, goalID)
	}
	slices.Sort(keys)
	for _, goalID := range keys {
		revision.PrefixEpisodes = append(revision.PrefixEpisodes, costForecastEpisode(record, goalID))
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
	return selectSealCandidateWithReader(root, candidate, plan, readCommittedGoal)
}

func selectSealCandidateWithReader(root string, candidate *Record, plan func(string, string, string) (testpolicy.Plan, error), read func(string, string, string) ([]byte, bool, error)) error {
	candidate.SelectedGroups = nil
	candidate.Seal = map[string]Claim{}
	units := joinedUnits(candidate.Units)
	for _, unit := range units {
		selection, planErr := plan(root, unit.GoalID, candidate.TipTree)
		if planErr != nil {
			return planErr
		}
		recordSelection(candidate, selection)
		claim, claimErr := claimAtWithReader(ModuleRoot(root), candidate.TipTree, candidate.BatchID, unit.GoalID, read)
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

func claimAt(root, tree, batchID, goalID string) (Claim, error) {
	return claimAtWithReader(ModuleRoot(root), tree, batchID, goalID, readCommittedGoal)
}

type committedGoalPrefixError struct{ error }

func readCommittedGoal(moduleRoot, tree, goalID string) ([]byte, bool, error) {
	workspace := gittree.Workspace{Dir: moduleRoot}
	prefix, err := workspace.Prefix()
	if err != nil {
		return nil, false, &committedGoalPrefixError{err}
	}
	path := prefix + filepath.ToSlash(filepath.Join("plans", "goals", goalID+".md"))
	return workspace.FileAt(tree, path)
}

func claimAtWithReader(moduleRoot, tree, batchID, goalID string, read func(string, string, string) ([]byte, bool, error)) (Claim, error) {
	data, present, err := read(moduleRoot, tree, goalID)
	var prefixErr *committedGoalPrefixError
	if errors.As(err, &prefixErr) {
		return Claim{}, prefixErr.error
	}
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
