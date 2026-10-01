package batch

import (
	"os"
	"path/filepath"
	"slices"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/gittree"
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

// SealChangedDuringGateRefusal preserves the registered refusal code when
// composition or policy selection ran on a candidate that changed meanwhile.
type SealChangedDuringGateRefusal struct{ BatchID string }

func recordSelection(record *Record, plan testpolicy.Plan) {
	record.SelectedGroups = append(record.SelectedGroups, plan.SelectedGroups...)
	slices.Sort(record.SelectedGroups)
	record.SelectedGroups = slices.Compact(record.SelectedGroups)
	if plan.RequiredMode == testpolicy.ModeDeep {
		record.ClosedReason = "deep-ceiling"
	}
}

type committedGoalPrefixError struct{ error }

func readCommittedGoal(moduleRoot, tree, goalID string) ([]byte, bool, error) {
	return readCommittedGoalWithWorkspace(gittree.Workspace{Dir: moduleRoot}, tree, goalID)
}

func readCommittedGoalWithWorkspace(workspace gittree.Workspace, tree, goalID string) ([]byte, bool, error) {
	prefix, err := workspace.Prefix()
	if err != nil {
		return nil, false, &committedGoalPrefixError{err}
	}
	path := prefix + filepath.ToSlash(filepath.Join("plans", "goals", goalID+".md"))
	return workspace.FileAt(tree, path)
}
