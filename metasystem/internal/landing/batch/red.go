package batch

import (
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/pathpattern"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testpolicy/adapter"
)

// DiagnosticRequest is one fresh, charged classification run. Tree is always
// derived from the durable batch record and NeverReuse must remain true.
type DiagnosticRequest struct {
	Tree, GoalID string
	Groups       []string
	Claim        Claim
	NeverReuse   bool
	// Fresh is the adapters' fresh-execution argv for a run whose purpose is
	// to execute, such as the second base run.
	Fresh []string
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
	// Adapter names a red group's language adapter and whether the group is a
	// package-selection expansion, whose manifest is the whole module; a nil
	// adapter means the group's language facts are unknown.
	Adapter func(RedGroup) (language adapter.Adapter, expansion bool)
}

// DiagnoseRed decides a red tip proof (D1): the failing groups run on the
// base; a base red runs once more, executed, and red twice is a trunk red
// that holds. On a green base every member named by evidence is ejected at
// once; when nobody is named every member returns with the log and the batch
// dissolves. A diagnostic that cannot run leaves the batch diagnosing for the
// next tick. It never changes the record's unit order.
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
	// runBase returns done when the diagnostic could not decide the red: the
	// fenced authority was ejected, or the batch stays diagnosing and err says
	// why when the runner failed outside an admission refusal.
	runBase := func(groups []RedGroup, fresh []string) (DiagnosticResult, bool, error) {
		result, runErr := seams.Run(DiagnosticRequest{Tree: record.BaseTree, GoalID: authority.GoalID, Groups: redGroupIDs(groups),
			Claim: authority.Claim, NeverReuse: true, Fresh: fresh})
		if runErr == nil {
			return result, false, nil
		}
		var refusal *DiagnosticRefusal
		status := runErr.Error()
		if errors.As(runErr, &refusal) {
			status, runErr = refusal.Status, nil
			if seams.ConfirmFenced != nil {
				reason, fenced, confirmErr := seams.ConfirmFenced(authority)
				if confirmErr == nil && fenced {
					return DiagnosticResult{}, true, ReassembleSurvivorsWithReturns(store, id, actor, at,
						[]ReturnDecision{{GoalID: authority.GoalID, Outcome: UnitEjected, Reason: reason}})
				}
				if confirmErr != nil {
					status += "; live fence verification: " + confirmErr.Error()
				}
			}
		}
		if seams.UpdateNext != nil {
			next := "diagnostic unavailable: " + status + "; the landing batch stays diagnosing and runs it again at its next tick"
			if editErr := seams.UpdateNext(authority.GoalID, next); editErr != nil {
				return DiagnosticResult{}, true, errors.Join(runErr, editErr)
			}
		}
		if runErr != nil {
			runErr = fmt.Errorf("batch %s stays diagnosing: base diagnostic unavailable: %w", id, runErr)
		}
		return DiagnosticResult{}, true, runErr
	}
	returnEveryMember := func(result DiagnosticResult) error {
		decisions := make([]ReturnDecision, 0, len(joined))
		for _, unit := range joined {
			decisions = append(decisions, ReturnDecision{GoalID: unit.GoalID, Outcome: UnitEjected, Reason: diagnosticFailure(result, failing)})
		}
		return ReassembleSurvivorsWithReturns(store, id, actor, at, decisions)
	}
	base, done, err := runBase(failing, nil)
	if done {
		return err
	}
	if !base.Green() {
		second, done, err := runBase(base.Groups, freshExecution(base.Groups, seams.Adapter))
		if done {
			return err
		}
		if !second.Green() {
			return holdAndRecordTrunkRed(store, record, actor, second, seams, at)
		}
		// Red then green on one base tree: main itself was intermittently red.
		// U1b-2 opens or promotes the known-flake entry here from the two base
		// attempts and decides step 3 with the identity known; until then
		// step 3 returns every member.
		return returnEveryMember(base)
	}
	named := namedDiagnosticUnits(joined, failing, seams.Adapter)
	if prefixGoal != "" {
		named = map[string]bool{prefixGoal: true}
	}
	if len(named) == 0 {
		// Step 3 until U1b-2 lands the register lookup: nobody named is a
		// return of every member with the log, never a hold.
		return returnEveryMember(base)
	}
	decisions := make([]ReturnDecision, 0, len(named))
	for _, unit := range joined {
		if named[unit.GoalID] {
			decisions = append(decisions, ReturnDecision{GoalID: unit.GoalID, Outcome: UnitEjected, Reason: diagnosticFailure(base, failing)})
		}
	}
	return ReassembleSurvivorsWithReturns(store, id, actor, at, decisions)
}

func redGroupIDs(groups []RedGroup) []string {
	ids := make([]string, 0, len(groups))
	for _, group := range groups {
		ids = append(ids, group.ID)
	}
	return ids
}

// freshExecution is the union of the red groups' adapters' fresh-execution
// argv, each argument once, in group order.
func freshExecution(groups []RedGroup, languageOf func(RedGroup) (adapter.Adapter, bool)) []string {
	var fresh []string
	for _, group := range groups {
		if languageOf == nil {
			break
		}
		if language, _ := languageOf(group); language != nil {
			for _, argument := range language.FreshExecution() {
				if !slices.Contains(fresh, argument) {
					fresh = append(fresh, argument)
				}
			}
		}
	}
	return fresh
}

func joinedUnits(units []Unit) []Unit {
	return slices.DeleteFunc(slices.Clone(units), func(unit Unit) bool { return unit.State != UnitJoined })
}

// namedDiagnosticUnits names a member for a red group (R2) when the adapter's
// owner unit of any failure is in the member's recorded closure, or, for a
// group that is not a package-selection expansion, when a changed path of the
// member matches the group's declared input manifest.
func namedDiagnosticUnits(units []Unit, failing []RedGroup, languageOf func(RedGroup) (adapter.Adapter, bool)) map[string]bool {
	named := map[string]bool{}
	for _, group := range failing {
		var language adapter.Adapter
		expansion := false
		if languageOf != nil {
			language, expansion = languageOf(group)
		}
		for _, unit := range units {
			if language != nil && unit.Closure != nil && ownsAFailure(language, group, *unit.Closure) {
				named[unit.GoalID] = true
				continue
			}
			if expansion {
				continue
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

func ownsAFailure(language adapter.Adapter, group RedGroup, closure adapter.Closure) bool {
	for _, failure := range group.Failures {
		owner, ok := language.OwnerUnit(adapter.Failure{Report: failure.Report, Classname: failure.Classname, Name: failure.Name,
			Status: failure.Status, Reason: failure.Reason}, closure)
		if ok && closure.Contains(owner) {
			return true
		}
	}
	return false
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

func diagnosticFailure(result DiagnosticResult, fallback []RedGroup) string {
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
	return fmt.Sprintf("attempt=%s groups=%s logs=%s failures=%s", result.AttemptID,
		strings.Join(ids, ","), strings.Join(logs, ","), strings.Join(failures, ","))
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
