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

// Terminal delivery's landed-unconsumed lines are a cycle-block annotation
// kind like every other: appended to the final block, they must pass the
// strict one-classification rule and never leak into the ledger tail rows.
func TestPromptLedgerRecordsSurviveLandedUnconsumedAnnotations(t *testing.T) {
	dir := t.TempDir()
	ledger := filepath.Join(dir, "ledger.md")
	if err := InitLedger(ledger, 8, 6); err != nil {
		t.Fatal(err)
	}
	sha := "77b6f9ab2c13e302782555a4830ad9ce08d738eb"
	if err := AppendCycle(ledger, 1, "contract-improved", sha, "score=3", "yes"); err != nil {
		t.Fatal(err)
	}
	if err := AppendAnnotations(ledger, 1,
		LandedUnconsumedAnnotation("chain-a", "2", "artifacts/agents/chain-a/rounds/2/return.json"),
		LandedUnconsumedAnnotation("overflow", "4", "none")); err != nil {
		t.Fatalf("terminal delivery append refused: %v", err)
	}
	_, _, cycles, err := ParseLedger(ledger)
	if err != nil || len(cycles) != 1 || len(cycles[0].Annotations) != 2 {
		t.Fatalf("landed annotations must parse and be exposed: %v (%+v)", err, cycles)
	}
	records, err := promptLedgerRecords(ledger, 8)
	if err != nil {
		t.Fatalf("prompt assembly failed on a landed-annotated block: %v", err)
	}
	if len(records) != 1 || records[0][1] != "contract-improved" || records[0][4] != "yes" {
		t.Fatalf("annotated row misread: %v", records)
	}
}

// The healed drain-stalled line and its survivor-count annotation must parse
// everywhere the ledger is read: the strict one-classification rule, the
// annotation surface, and the prompt's ledger tail.
func TestHealedDrainStalledLineParsesAndSurvivesPromptAssembly(t *testing.T) {
	dir := t.TempDir()
	ledger := filepath.Join(dir, "ledger.md")
	if err := InitLedger(ledger, 8, 6); err != nil {
		t.Fatal(err)
	}
	sha := "77b6f9ab2c13e302782555a4830ad9ce08d738eb"
	if err := AppendCycle(ledger, 1, "unresolved", sha, "score=1", "no"); err != nil {
		t.Fatal(err)
	}
	if err := AppendCycle(ledger, 2, "no-progress", sha, DrainStalledObserved, "no", DrainStalledAnnotation(2)); err != nil {
		t.Fatalf("healed drain-stalled line refused: %v", err)
	}
	_, _, cycles, err := ParseLedger(ledger)
	if err != nil || len(cycles) != 2 {
		t.Fatalf("healed ledger must parse with 2 cycles: %v (%d)", err, len(cycles))
	}
	if len(cycles[1].Annotations) != 1 || cycles[1].Annotations[0] != "Drain: stalled:2" {
		t.Fatalf("the survivor-count annotation must be exposed: %v", cycles[1].Annotations)
	}
	records, err := promptLedgerRecords(ledger, 8)
	if err != nil {
		t.Fatalf("prompt assembly failed on a drain-stalled cycle block: %v", err)
	}
	if len(records) != 2 {
		t.Fatalf("want 2 cycle records, got %d", len(records))
	}
	row := records[1]
	if len(row) != 5 || row[1] != "no-progress" || row[3] != DrainStalledObserved || row[4] != "no" {
		t.Fatalf("drain-stalled row misread by prompt assembly: %v", row)
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
