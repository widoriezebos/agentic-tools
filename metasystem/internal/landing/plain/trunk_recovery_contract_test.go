package plain

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testpolicy"
)

// This certifies the shipped contract and ledger directly; it does not freeze
// their current contents as fixtures for a behavior test.
func TestTrunkRecoveryRealContractLedgerAndPlan(t *testing.T) {
	t.Parallel()
	install, err := filepath.Abs("../../..")
	if err != nil {
		t.Fatal(err)
	}
	checkout := filepath.Dir(install)
	contract, err := testpolicy.Load(filepath.Join(install, "testing.json"))
	if err != nil {
		t.Fatal(err)
	}
	pages, err := filepath.Glob(filepath.Join(install, "plans/goals/*.md"))
	if err != nil {
		t.Fatal(err)
	}
	var file *goal.GoalFile
	var ledgerPath string
	for _, path := range pages {
		if filepath.Base(path) == "backlog.md" {
			continue
		}
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		parsed, problems := goal.ParseFile(data)
		if len(problems) != 0 {
			t.Fatalf("live goal %s: %v", path, problems)
		}
		if parsed.Risk != nil {
			file, ledgerPath = parsed, path
			break
		}
	}
	if file == nil {
		t.Fatal("live ledger has no risked goal")
	}
	ledgerPath, err = filepath.Rel(checkout, ledgerPath)
	if err != nil {
		t.Fatal(err)
	}
	ledgerPath = filepath.ToSlash(ledgerPath)
	code := "metasystem/internal/landing/plain/cause.go"
	plan, err := testpolicy.Select(contract, testpolicy.SelectionRequest{ChangedPaths: []string{code, ledgerPath}, RequestedMode: testpolicy.ModeAuto, Purpose: testpolicy.PurposeDelivery,
		GoalRisk: testpolicy.GoalRisk{Severity: int(file.Risk.Severity), Novelty: int(file.Risk.Novelty), Exposure: int(file.Risk.Exposure), Accumulation: int(file.Risk.Accumulation)}})
	if err != nil || len(plan.SelectedGroups) == 0 || len(plan.Stages) == 0 {
		t.Fatalf("live plan: %+v %v", plan, err)
	}
	output, err := json.Marshal(plan)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(output), `"selectedGroups"`) {
		t.Fatalf("plan output: %s", output)
	}
	for _, tc := range []struct {
		path string
		code bool
	}{{code, true}, {"metasystem/testing.json", true}, {ledgerPath, false}} {
		seams := ProveSeams{Git: func(string, ...string) (string, error) { return tc.path, nil }}
		moved, err := codeMoved(install, checkout, "A", "B", seams)
		if err != nil || moved != tc.code {
			t.Fatalf("live path %s code=%v want=%v: %v", tc.path, moved, tc.code, err)
		}
	}
	t.Logf("live contract %d groups; live goal %s; actual plan output: %s", len(contract.Groups), file.Id, output)
}
