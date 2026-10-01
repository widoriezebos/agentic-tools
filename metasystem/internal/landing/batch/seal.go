package batch

import (
	"os"
	"path/filepath"
	"slices"

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

func recordSelection(record *Record, plan testpolicy.Plan) {
	record.SelectedGroups = append(record.SelectedGroups, plan.SelectedGroups...)
	slices.Sort(record.SelectedGroups)
	record.SelectedGroups = slices.Compact(record.SelectedGroups)
	if plan.RequiredMode == testpolicy.ModeDeep {
		record.ClosedReason = "deep-ceiling"
	}
}
