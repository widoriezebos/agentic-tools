package batch

import (
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/pathpattern"
)

// DiagnosticRequest is one fresh, charged classification run. Tree is always
// derived from the durable batch record and NeverReuse must remain true.
type DiagnosticRequest struct {
	Tree, GoalID string
	Groups       []string
	Claim        Claim
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
	Run func(DiagnosticRequest) (DiagnosticResult, error)
	// ConfirmFenced re-reads the live accepted ledger and binds a stop fence to
	// this exact joined, handed-over claim. A runner refusal alone is not proof.
	ConfirmFenced func(Unit) (string, bool, error)
	MintOpid      func() (string, error)
	Ledger        LedgerOwner
	UpdateNext    func(goalID, status string) error
	BaseCommit    string
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
	authority := joined[len(joined)-1]
	groupIDs := make([]string, 0, len(failing))
	for _, group := range failing {
		groupIDs = append(groupIDs, group.ID)
	}
	run := func(tree string, unit Unit) (DiagnosticResult, bool, error) {
		result, runErr := seams.Run(DiagnosticRequest{Tree: tree, GoalID: unit.GoalID, Groups: slices.Clone(groupIDs), Claim: unit.Claim, NeverReuse: true})
		if runErr != nil {
			var refusal *DiagnosticRefusal
			if errors.As(runErr, &refusal) {
				status := refusal.Status
				if seams.ConfirmFenced != nil {
					reason, fenced, confirmErr := seams.ConfirmFenced(unit)
					if confirmErr == nil && fenced {
						return DiagnosticResult{}, true, ReassembleSurvivorsWithReturns(store, id, actor, at,
							[]ReturnDecision{{GoalID: unit.GoalID, Outcome: UnitEjected, Reason: reason}})
					}
					if confirmErr != nil {
						status += "; live fence verification: " + confirmErr.Error()
					}
				}
				next := "diagnostic refusal: " + status
				if seams.UpdateNext != nil {
					if editErr := seams.UpdateNext(authority.GoalID, next); editErr != nil {
						return DiagnosticResult{}, false, errors.Join(runErr, editErr)
					}
				}
				return DiagnosticResult{}, true, HoldUnclassified(store, id, actor, status, next, at)
			}
		}
		return result, false, runErr
	}
	hold := func(reason string) error {
		next := "inspect the diagnostic evidence and ordered member patch composition before returning a unit"
		if err := HoldUnclassified(store, id, actor, reason, next, at); err != nil {
			return err
		}
		if seams.UpdateNext != nil {
			return seams.UpdateNext(authority.GoalID, next)
		}
		return nil
	}
	base, held, err := run(record.BaseTree, authority)
	if err != nil {
		return hold("base diagnostic unavailable: " + err.Error())
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
		return hold("green base but no failed group can be attributed to a member")
	}
	decisions := make([]ReturnDecision, 0, len(named))
	if len(named) == 1 {
		for goalID := range named {
			decisions = append(decisions, ReturnDecision{GoalID: goalID, Outcome: UnitEjected, Reason: diagnosticFailure(base, failing, nil)})
		}
	} else {
		unnamed := slices.DeleteFunc(slices.Clone(joined), func(unit Unit) bool { return named[unit.GoalID] })
		survivors := slices.Clone(unnamed)
		if len(unnamed) > 0 {
			trees, assembleErr := store.reassembly.assemble(record.BaseTree, unnamed)
			if assembleErr != nil {
				return hold("unnamed members do not compose: " + assembleErr.Error())
			}
			result, held, runErr := run(trees[len(trees)-1], unnamed[len(unnamed)-1])
			if held {
				return nil
			}
			if runErr != nil {
				return hold("unnamed diagnostic unavailable: " + runErr.Error())
			}
			if !result.Green() {
				return hold("unnamed diagnostic red")
			}
		}
		for _, unit := range joined {
			if !named[unit.GoalID] {
				continue
			}
			included := map[string]bool{unit.GoalID: true}
			for _, survivor := range survivors {
				included[survivor.GoalID] = true
			}
			candidate := slices.DeleteFunc(slices.Clone(joined), func(member Unit) bool { return !included[member.GoalID] })
			trees, assembleErr := store.reassembly.assemble(record.BaseTree, candidate)
			if assembleErr != nil {
				var conflict *assemblyConflict
				if len(decisions) == 0 || !errors.As(assembleErr, &conflict) || conflict.GoalID != unit.GoalID {
					return hold("named diagnostic members do not compose: " + assembleErr.Error())
				}
				decisions = append(decisions, ReturnDecision{GoalID: unit.GoalID, Outcome: UnitEjected,
					Reason: "cannot apply after returning " + returnedGoalIDs(decisions) + ": " + assembleErr.Error()})
				continue
			}
			result, held, runErr := run(trees[len(trees)-1], unit)
			if runErr != nil {
				return hold("named diagnostic unavailable: " + runErr.Error())
			}
			if held {
				return nil
			}
			if result.Green() {
				survivors = candidate
				continue
			}
			decisions = append(decisions, ReturnDecision{GoalID: unit.GoalID, Outcome: UnitEjected,
				Reason: diagnosticFailure(result, failing, survivors)})
		}
	}
	return ReassembleSurvivorsWithReturns(store, id, actor, at, decisions)
}

func returnedGoalIDs(decisions []ReturnDecision) string {
	ids := make([]string, 0, len(decisions))
	for _, decision := range decisions {
		ids = append(ids, decision.GoalID)
	}
	return strings.Join(ids, ",")
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
	value, literal, err := pathpattern.ManifestEntry(pattern)
	if err != nil {
		return true
	}
	value, changed = strings.TrimPrefix(value, "metasystem/"), strings.TrimPrefix(changed, "metasystem/")
	if literal {
		return value == changed
	}
	matched, err := pathpattern.MatchManifestEntry(value, changed)
	if err != nil {
		return true
	}
	return matched
}

func diagnosticFailure(result DiagnosticResult, fallback []RedGroup, landedPartners []Unit) string {
	groups := result.Groups
	if len(groups) == 0 {
		groups = fallback
	}
	ids, logs, failures := make([]string, 0, len(groups)), []string{}, []string{}
	for _, group := range groups {
		ids = append(ids, group.ID)
		if group.LogPath != "" {
			logs = append(logs, group.LogPath)
		}
		for _, failure := range group.Failures {
			if failure.Name != "" {
				failures = append(failures, failure.Name)
			}
		}
	}
	partners := make([]string, 0, len(landedPartners))
	for _, unit := range landedPartners {
		partners = append(partners, unit.GoalID)
	}
	return fmt.Sprintf("attempt=%s groups=%s logs=%s failures=%s landed-partners=%s", result.AttemptID,
		strings.Join(ids, ","), strings.Join(logs, ","), strings.Join(failures, ","), strings.Join(partners, ","))
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
