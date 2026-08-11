package mission

import (
	"path/filepath"
	"testing"
)

func TestPromptLedgerRecordsSurviveTrailingResetLine(t *testing.T) {
	dir := t.TempDir()
	ledger := filepath.Join(dir, "ledger.md")
	if err := InitLedger(ledger, 8, 6); err != nil {
		t.Fatal(err)
	}
	sha := "77b6f9ab2c13e302782555a4830ad9ce08d738eb"
	if err := AppendCycle(ledger, 1, "unresolved", sha, "self-assessment=0", "no"); err != nil {
		t.Fatal(err)
	}
	if err := AppendCycle(ledger, 2, "no-progress", sha, "unmeasurable:capped", "no"); err != nil {
		t.Fatal(err)
	}
	if err := AppendReset(ledger, "stop-loss", "human judged the tail worth more of the sealed fences"); err != nil {
		t.Fatalf("append reset: %v", err)
	}
	records, err := promptLedgerRecords(ledger, 8)
	if err != nil {
		t.Fatalf("prompt assembly failed after a trailing reset line: %v", err)
	}
	if len(records) != 2 {
		t.Fatalf("want 2 cycle records, got %d", len(records))
	}
}
