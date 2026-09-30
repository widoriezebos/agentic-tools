package lane

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testgoal"
)

// TestValidationDueReadsTheNestedModuleLedger (A-a, critique F-5): the
// production due read, over a goal ledger at the nested module root of the
// lane checkout (never its top): no cadence status yet is due by the
// cadence's own rule; a forced window inside its interval is not; one past
// it is; and the read creates nothing in the installation.
func TestValidationDueReadsTheNestedModuleLedger(t *testing.T) {
	t.Parallel()
	_, checkout, module := nestedLaneDirs(t)
	now := laneNow
	root := goal.RenderRoot(&goal.RootRecord{Identity: "01ARZ3NDEKTSV4RRFFQ69G5FAV", FormatVersion: "1", SyncMode: goal.SyncLocal, Revision: 1})
	ledger := func(files map[string][]byte) func(string) (goal.Endpoint, error) {
		files["plans/goals/backlog.md"] = root
		repository := testgoal.New(files, now, "0000000000000000000000000000000000000001")
		return func(asked string) (goal.Endpoint, error) {
			if asked != module {
				t.Errorf("the due read resolved the ledger at %s, want the module root %s", asked, module)
			}
			return goal.Endpoint{Root: asked, Remote: goal.SyncLocal, Branch: goal.LocalLedgerBranch, Repository: repository}, nil
		}
	}
	before := entries(t, checkout)
	if due, err := validationDueWith(module, now, ledger(map[string][]byte{})); err != nil || !due {
		t.Fatalf("never validated: due=%t err=%v; want due", due, err)
	}
	status := &goal.CadenceStatus{TrunkCommit: strings.Repeat("a", 40), TrunkTree: strings.Repeat("b", 40), Trigger: goal.CadenceTriggerForcedWindow,
		RunID: "run-old", AttemptID: "attempt-old", StartedAt: now.Format(time.RFC3339), EndedAt: now.Format(time.RFC3339),
		ForcedWindowStart: now.Add(-time.Hour).Format(time.RFC3339), Opid: "01ARZ3NDEKTSV4RRFFQ69G5FAW-bed-m1-00000001", Groups: []goal.CadenceGroupStatus{{Group: "section/deep",
			ExecutionIdentity: strings.Repeat("1", 64), Status: "passed", EvidenceDigest: strings.Repeat("e", 64)}}}
	data, err := json.MarshalIndent(map[string]any{"schema": 1, "entries": []any{}, "cadence": status}, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	withStatus := ledger(map[string][]byte{"plans/goals/trunk-red.json": append(data, '\n')})
	if due, err := validationDueWith(module, now, withStatus); err != nil || due {
		t.Fatalf("inside the forced window: due=%t err=%v; want not due", due, err)
	}
	if due, err := validationDueWith(module, now.Add(6*time.Hour), withStatus); err != nil || !due {
		t.Fatalf("past the forced window: due=%t err=%v; want due", due, err)
	}
	if after := entries(t, checkout); after != before {
		t.Fatalf("the due read wrote into the lane checkout:\n%s\nwas\n%s", after, before)
	}
}

// entries lists every path under root.
func entries(t *testing.T, root string) string {
	t.Helper()
	var paths []string
	if err := filepath.Walk(root, func(path string, _ os.FileInfo, err error) error {
		paths = append(paths, path)
		return err
	}); err != nil {
		t.Fatal(err)
	}
	return strings.Join(paths, "\n")
}
