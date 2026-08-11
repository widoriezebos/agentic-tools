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

// The strict one-classification-per-block rule counts only Classification
// lines: both annotation kinds (the rejected-return fault and the capped
// outcome) must pass through prompt assembly without becoming a second
// classification or leaking into the ledger tail rows.
func TestPromptLedgerRecordsSurviveAnnotationLines(t *testing.T) {
	dir := t.TempDir()
	ledger := filepath.Join(dir, "ledger.md")
	if err := InitLedger(ledger, 8, 6); err != nil {
		t.Fatal(err)
	}
	sha := "77b6f9ab2c13e302782555a4830ad9ce08d738eb"
	if err := AppendCycle(ledger, 1, "contract-improved", sha, "score=2", "yes",
		ReturnRejectedAnnotation("orchestrator return session identity matches neither the announced nor the observed session")); err != nil {
		t.Fatal(err)
	}
	if err := AppendCycle(ledger, 2, "unresolved", sha, "score=2", "no", CappedAnnotation); err != nil {
		t.Fatal(err)
	}
	records, err := promptLedgerRecords(ledger, 8)
	if err != nil {
		t.Fatalf("prompt assembly failed on annotated cycle blocks: %v", err)
	}
	if len(records) != 2 {
		t.Fatalf("want 2 cycle records, got %d", len(records))
	}
	if records[0][1] != "contract-improved" || records[0][3] != "score=2" || records[0][4] != "yes" {
		t.Fatalf("annotated row misread: %v", records[0])
	}
	if records[1][1] != "unresolved" {
		t.Fatalf("capped-annotation row misread: %v", records[1])
	}
}

func TestHealedTurnLostLineParsesAndSurvivesPromptAssembly(t *testing.T) {
	dir := t.TempDir()
	ledger := filepath.Join(dir, "ledger.md")
	if err := InitLedger(ledger, 8, 6); err != nil {
		t.Fatal(err)
	}
	sha := "77b6f9ab2c13e302782555a4830ad9ce08d738eb"
	if err := AppendCycle(ledger, 1, "unresolved", sha, "self-assessment=0", "no"); err != nil {
		t.Fatal(err)
	}
	// The resume heal records a reserved-but-never-appended cycle exactly
	// like this: a lost turn with no measurement.
	if err := AppendCycle(ledger, 2, "no-progress", sha, "unmeasurable:turn-lost", "no"); err != nil {
		t.Fatalf("healed lost-turn line refused: %v", err)
	}
	if _, _, cycles, err := ParseLedger(ledger); err != nil || len(cycles) != 2 {
		t.Fatalf("healed ledger must parse with 2 cycles: %v (%d)", err, len(cycles))
	}
	records, err := promptLedgerRecords(ledger, 8)
	if err != nil {
		t.Fatalf("prompt assembly failed on a healed ledger: %v", err)
	}
	if len(records) != 2 {
		t.Fatalf("want 2 cycle records, got %d", len(records))
	}
	row := records[1]
	if len(row) != 5 || row[0] != "2" || row[1] != "no-progress" || row[2] != sha ||
		row[3] != "unmeasurable:turn-lost" || row[4] != "no" {
		t.Fatalf("healed row misread by prompt assembly: %v", row)
	}
}
