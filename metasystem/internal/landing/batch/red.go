package batch

import (
	"errors"
	"fmt"
	"path"
	"slices"
	"strings"
	"time"
)

// DiagnosticRequest is one fresh, charged classification run. Tree is always
// derived from the durable batch record and NeverReuse must remain true.
type DiagnosticRequest struct {
	Tree, GoalID string
	Groups       []string
	NeverReuse   bool
}

// DiagnosticResult is the evidence needed to classify one diagnostic tree.
type DiagnosticResult struct {
	AttemptID string
	Groups    []RedGroup
}

func (result DiagnosticResult) Green() bool { return len(result.Groups) == 0 }

// DiagnosticRefusal identifies a refusal before a diagnostic runner started.
type DiagnosticRefusal struct{ Status string }

func (refusal *DiagnosticRefusal) Error() string { return refusal.Status }

// RedSeams names the effects outside classification and tree derivation.
type RedSeams struct {
	Run        func(DiagnosticRequest) (DiagnosticResult, error)
	MintOpid   func() (string, error)
	Ledger     LedgerOwner
	UpdateNext func(goalID, status string) error
	BaseCommit string
}

// DiagnoseRed runs the failed groups on the base first, then follows the
// bounded named-unit schedule. It never changes the record's unit order.
func DiagnoseRed(store Store, id, actor string, failing []RedGroup, prefixGoal string, at time.Time, seams RedSeams) error {
	record, err := store.Load(id)
	if err != nil {
		return err
	}
	if record.State != StateDiagnosing || record.Proof == nil {
		return fmt.Errorf("batch %s is not diagnosing a recorded proof", id)
	}
	joined := joinedUnits(record.Units)
	if len(joined) == 0 || seams.Run == nil {
		return fmt.Errorf("batch %s has no diagnostic runner or joined units", id)
	}
	authority := joined[len(joined)-1].GoalID
	groupIDs := make([]string, 0, len(failing))
	for _, group := range failing {
		groupIDs = append(groupIDs, group.ID)
	}
	run := func(tree string) (DiagnosticResult, bool, error) {
		result, runErr := seams.Run(DiagnosticRequest{Tree: tree, GoalID: authority, Groups: slices.Clone(groupIDs), NeverReuse: true})
		if runErr != nil {
			var refusal *DiagnosticRefusal
			if errors.As(runErr, &refusal) {
				next := "diagnostic refusal: " + refusal.Status
				if seams.UpdateNext != nil {
					if editErr := seams.UpdateNext(authority, next); editErr != nil {
						return DiagnosticResult{}, false, errors.Join(runErr, editErr)
					}
				}
				return DiagnosticResult{}, true, HoldUnclassified(store, id, actor, refusal.Status, next, at)
			}
		}
		return result, false, runErr
	}
	base, held, err := run(record.BaseTree)
	if err != nil {
		return err
	}
	if held {
		return nil
	}
	if !base.Green() {
		return holdAndRecordTrunkRed(store, record, actor, base, seams, at)
	}
	named := namedDiagnosticUnits(joined, failing)
	if prefixGoal != "" {
		named = map[string]bool{prefixGoal: true}
	}
	if len(named) == 0 {
		return holdAndRecordTrunkRed(store, record, actor, DiagnosticResult{AttemptID: record.Proof.AttemptID, Groups: failing}, seams, at)
	}
	if len(named) == 1 {
		for goalID := range named {
			if err := RequestReturn(store, id, goalID, UnitEjected, diagnosticFailure(base, failing), actor, at); err != nil {
				return err
			}
		}
		return ReassembleSurvivors(store, id, actor, at)
	}
	unnamed := slices.DeleteFunc(slices.Clone(joined), func(unit Unit) bool { return named[unit.GoalID] })
	survivors := slices.Clone(unnamed)
	if len(unnamed) > 0 {
		trees, assembleErr := assembleUnits(store.root, record.BaseTree, unnamed)
		if assembleErr != nil {
			return assembleErr
		}
		result, held, runErr := run(trees[len(trees)-1])
		if held {
			return nil
		}
		if runErr != nil || !result.Green() {
			if runErr != nil {
				return runErr
			}
			return HoldUnclassified(store, id, actor, "unnamed diagnostic red", "Gk Next: inspect unnamed diagnostic", at)
		}
	}
	for _, unit := range joined {
		if !named[unit.GoalID] {
			continue
		}
		candidate := append(slices.Clone(survivors), unit)
		trees, assembleErr := assembleUnits(store.root, record.BaseTree, candidate)
		if assembleErr != nil {
			return assembleErr
		}
		result, held, runErr := run(trees[len(trees)-1])
		if runErr != nil {
			return runErr
		}
		if held {
			return nil
		}
		if result.Green() {
			survivors = candidate
			continue
		}
		if err := RequestReturn(store, id, unit.GoalID, UnitEjected, diagnosticFailure(result, failing), actor, at); err != nil {
			return err
		}
	}
	return ReassembleSurvivors(store, id, actor, at)
}

func joinedUnits(units []Unit) []Unit {
	return slices.DeleteFunc(slices.Clone(units), func(unit Unit) bool { return unit.State != UnitJoined })
}

func namedDiagnosticUnits(units []Unit, failing []RedGroup) map[string]bool {
	named := map[string]bool{}
	for _, unit := range units {
		for _, group := range failing {
			if slices.Contains(unit.SelectedGroups, group.ID) {
				named[unit.GoalID] = true
			}
			for _, changed := range unit.ChangedPaths {
				for _, input := range group.InputManifest {
					if diagnosticPathMatches(input, changed) {
						named[unit.GoalID] = true
					}
				}
			}
		}
	}
	return named
}

func diagnosticPathMatches(pattern, changed string) bool {
	pattern, changed = strings.TrimPrefix(pattern, "metasystem/"), strings.TrimPrefix(changed, "metasystem/")
	if prefix, ok := strings.CutSuffix(pattern, "/**"); ok {
		return changed == prefix || strings.HasPrefix(changed, prefix+"/")
	}
	matched, _ := path.Match(pattern, changed)
	return matched
}

func diagnosticFailure(result DiagnosticResult, fallback []RedGroup) string {
	groups := result.Groups
	if len(groups) == 0 {
		groups = fallback
	}
	ids := make([]string, 0, len(groups))
	for _, group := range groups {
		ids = append(ids, group.ID)
	}
	return fmt.Sprintf("attempt=%s groups=%s", result.AttemptID, strings.Join(ids, ","))
}

func holdAndRecordTrunkRed(store Store, record Record, actor string, result DiagnosticResult, seams RedSeams, at time.Time) error {
	if seams.MintOpid == nil || seams.Ledger == nil {
		return errLedgerOwnerUnbound
	}
	opid, err := seams.MintOpid()
	if err != nil {
		return err
	}
	red := TrunkRed{AttemptID: result.AttemptID, BaseCommit: seams.BaseCommit, BaseTree: record.BaseTree, Groups: result.Groups}
	if err := store.HoldTrunkRed(record.BatchID, red, opid, at, actor); err != nil {
		return err
	}
	_, err = store.WithLedgerOwner(seams.Ledger).EnsureTrunkRedRecorded(record.BatchID, seams.MintOpid, at, actor)
	return err
}
