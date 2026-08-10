package mission

import (
	"path/filepath"
	"strings"
	"testing"
)

const goodSHA = "abcdef0123456789abcdef0123456789abcdef01" // 40 hex

func TestInitParseAndRefuseOverwrite(t *testing.T) {
	file := filepath.Join(t.TempDir(), "ledger.md")
	if err := InitLedger(file, 5, 3); err != nil {
		t.Fatal(err)
	}
	cb, ngb, cycles, err := ParseLedger(file)
	if err != nil {
		t.Fatal(err)
	}
	if cb != 5 || ngb != 3 || len(cycles) != 0 {
		t.Fatalf("fresh ledger wrong: cb=%d ngb=%d cycles=%d", cb, ngb, len(cycles))
	}
	if err := InitLedger(file, 5, 3); err == nil {
		t.Fatal("init must refuse to overwrite an existing ledger")
	}
}

func TestInitRejectsNonPositiveBudgets(t *testing.T) {
	file := filepath.Join(t.TempDir(), "ledger.md")
	if err := InitLedger(file, 0, 3); err == nil {
		t.Fatal("non-positive budget must be refused")
	}
}

func TestAppendCyclesInOrder(t *testing.T) {
	file := filepath.Join(t.TempDir(), "ledger.md")
	if err := InitLedger(file, 5, 3); err != nil {
		t.Fatal(err)
	}
	if err := AppendCycle(file, 1, "contract-improved", goodSHA, "score=0.5"); err != nil {
		t.Fatal(err)
	}
	if err := AppendCycle(file, 2, "no-progress", goodSHA, "score=0.5"); err != nil {
		t.Fatal(err)
	}
	_, _, cycles, err := ParseLedger(file)
	if err != nil {
		t.Fatal(err)
	}
	if len(cycles) != 2 || cycles[0].Number != 1 || cycles[1].Number != 2 {
		t.Fatalf("expected two contiguous cycles: %+v", cycles)
	}
	// The next cycle must be 3.
	if err := AppendCycle(file, 2, "no-progress", goodSHA, "x"); err == nil ||
		!strings.Contains(err.Error(), "must be 3") {
		t.Fatalf("appending a wrong cycle number must be refused, got %v", err)
	}
}

func TestAppendValidatesShaAndClassification(t *testing.T) {
	file := filepath.Join(t.TempDir(), "ledger.md")
	_ = InitLedger(file, 5, 3)
	if err := AppendCycle(file, 1, "bogus-class", goodSHA, "x"); err == nil {
		t.Fatal("unknown classification must be refused")
	}
	if err := AppendCycle(file, 1, "contract-improved", "not-a-sha", "x"); err == nil {
		t.Fatal("a non-sha candidate must be refused")
	}
	if err := AppendCycle(file, 1, "contract-improved", goodSHA, ""); err == nil {
		t.Fatal("an empty observed measurement must be refused")
	}
}

func TestParseRejectsMalformed(t *testing.T) {
	file := filepath.Join(t.TempDir(), "ledger.md")
	if err := atomicWriteText(file, "# Mission Ledger\n\n- Cycle budget: 5\n"); err != nil {
		t.Fatal(err)
	}
	if _, _, _, err := ParseLedger(file); err == nil {
		t.Fatal("a ledger missing the no-gain budget must be rejected")
	}
}
