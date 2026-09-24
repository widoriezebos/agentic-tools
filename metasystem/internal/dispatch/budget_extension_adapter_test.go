package dispatch

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/gittree"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
)

func TestLandingAdvancementGitAdapterReadsNestedReceipt(t *testing.T) {
	now := time.Date(2026, 8, 28, 10, 0, 0, 0, time.UTC)
	top := t.TempDir()
	installation := filepath.Join(top, "metasystem")
	for _, directory := range []string{filepath.Join(top, "development"), filepath.Join(installation, "memory"), filepath.Join(installation, "plans", "goals")} {
		if err := os.MkdirAll(directory, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(top, "development", "metasystem-design.md"), []byte("# template\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	line := shippedReceiptLine(now.Add(-time.Hour), "nested-goal", "implement", "shipped")
	if err := os.WriteFile(filepath.Join(installation, "memory", "receipts.log"), []byte(line+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(installation, "plans", "goals", "backlog.md"), []byte("# fixture ledger\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{
		{"init", "-q"},
		{"config", "user.email", "fixture@example.invalid"},
		{"config", "user.name", "fixture"},
		{"add", "development/metasystem-design.md", "metasystem/memory/receipts.log", "metasystem/plans/goals/backlog.md"},
		{"commit", "-q", "-m", "nested template receipt"},
		{"update-ref", goal.AcceptedRef, "HEAD"},
	} {
		if output, err := exec.Command("git", append([]string{"-C", top}, args...)...).CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v: %s", args, err, output)
		}
	}
	workspace := gittree.Workspace{Dir: installation}
	receiptPath, err := budgetExtensionReceiptPath(workspace, installation)
	if err != nil || receiptPath != "metasystem/memory/receipts.log" {
		t.Fatalf("nested receipt path = %q, %v", receiptPath, err)
	}
	tip, present, err := goal.AcceptedLedgerTip(installation)
	if err != nil || !present {
		t.Fatalf("nested accepted tip: %q present=%v err=%v", tip, present, err)
	}
	data, present, err := workspace.FileAt(tip, receiptPath)
	if err != nil || !present || string(data) != line+"\n" {
		t.Fatalf("nested receipt read: present=%v data=%q err=%v", present, data, err)
	}
	evidence, err := landingAdvancementEvidence(installation, "nested-goal", now)
	if err != nil || len(evidence) != 1 || evidence[0].kind != "landing" {
		t.Fatalf("nested template receipt was not selected: evidence=%+v err=%v", evidence, err)
	}
}
