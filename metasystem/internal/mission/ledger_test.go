package mission

import (
	"os"
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
	if err := AppendCycle(file, 1, "contract-improved", goodSHA, "score=0.5", ""); err != nil {
		t.Fatal(err)
	}
	if err := AppendCycle(file, 2, "no-progress", goodSHA, "score=0.5", ""); err != nil {
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
	if err := AppendCycle(file, 2, "no-progress", goodSHA, "x", ""); err == nil ||
		!strings.Contains(err.Error(), "must be 3") {
		t.Fatalf("appending a wrong cycle number must be refused, got %v", err)
	}
}

func TestAppendValidatesShaAndClassification(t *testing.T) {
	file := filepath.Join(t.TempDir(), "ledger.md")
	_ = InitLedger(file, 5, 3)
	if err := AppendCycle(file, 1, "bogus-class", goodSHA, "x", ""); err == nil {
		t.Fatal("unknown classification must be refused")
	}
	if err := AppendCycle(file, 1, "contract-improved", "not-a-sha", "x", ""); err == nil {
		t.Fatal("a non-sha candidate must be refused")
	}
	if err := AppendCycle(file, 1, "contract-improved", goodSHA, "", ""); err == nil {
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

func TestAppendCycleBestMarker(t *testing.T) {
	file := filepath.Join(t.TempDir(), "ledger.md")
	if err := InitLedger(file, 5, 3); err != nil {
		t.Fatal(err)
	}
	if err := AppendCycle(file, 1, "contract-improved", goodSHA, "score=0.6", "yes"); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "observed=score=0.6; best=yes\n") {
		t.Fatalf("best marker missing from the measurement line:\n%s", data)
	}
	if err := AppendCycle(file, 2, "unresolved", goodSHA, "score=0.6", "maybe"); err == nil {
		t.Fatal("an unknown best marker must be refused")
	}
}

func TestAppendResetGrammarAndSanitization(t *testing.T) {
	file := filepath.Join(t.TempDir(), "ledger.md")
	if err := InitLedger(file, 5, 3); err != nil {
		t.Fatal(err)
	}
	if err := AppendCycle(file, 1, "no-progress", goodSHA, "score=0.5", "no"); err != nil {
		t.Fatal(err)
	}
	if err := AppendReset(file, "stop-loss", "tail work justifies more of the sealed fences"); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "\nStop-loss reset: ask=stop-loss; reason=tail work justifies more of the sealed fences\n") {
		t.Fatalf("reset line missing:\n%s", data)
	}
	// The refused reasons: newline-bearing (never mangled), empty, over-cap,
	// and a malformed ask id. None of them may touch the ledger.
	before, _ := os.ReadFile(file)
	for name, attempt := range map[string]func() error{
		"newline reason": func() error { return AppendReset(file, "stop-loss", "line one\nline two") },
		"empty reason":   func() error { return AppendReset(file, "stop-loss", "   ") },
		"over-cap":       func() error { return AppendReset(file, "stop-loss", strings.Repeat("x", resetReasonMaxLen+1)) },
		"bad ask id":     func() error { return AppendReset(file, "Not An Id", "reason") },
	} {
		if err := attempt(); err == nil {
			t.Fatalf("%s must be refused", name)
		}
	}
	after, _ := os.ReadFile(file)
	if string(before) != string(after) {
		t.Fatal("a refused reset must not touch the ledger")
	}
	// The ledger still parses, and the reset does not count as a cycle.
	_, _, cycles, err := ParseLedger(file)
	if err != nil || len(cycles) != 1 {
		t.Fatalf("ledger after reset: cycles=%d err=%v", len(cycles), err)
	}
}

func TestParseLedgerEventsOrderAndTokens(t *testing.T) {
	file := filepath.Join(t.TempDir(), "ledger.md")
	if err := InitLedger(file, 5, 3); err != nil {
		t.Fatal(err)
	}
	if err := AppendCycle(file, 1, "no-progress", goodSHA, "score=0.5", "no"); err != nil {
		t.Fatal(err)
	}
	if err := AppendReset(file, "stop-loss", "keep going"); err != nil {
		t.Fatal(err)
	}
	if err := AppendCycle(file, 2, "contract-improved", goodSHA, "score=0.9", "yes"); err != nil {
		t.Fatal(err)
	}
	cycleBudget, noGainBudget, events, err := ParseLedgerEvents(file)
	if err != nil {
		t.Fatal(err)
	}
	if cycleBudget != 5 || noGainBudget != 3 {
		t.Fatalf("budgets: %d/%d", cycleBudget, noGainBudget)
	}
	if len(events) != 3 {
		t.Fatalf("events: %+v", events)
	}
	if events[0].Reset || events[0].Classification != "no-progress" || events[0].Observed != "score=0.5" || events[0].Best != "no" {
		t.Fatalf("first event: %+v", events[0])
	}
	if !events[1].Reset || events[1].AskID != "stop-loss" || events[1].Reason != "keep going" {
		t.Fatalf("reset event out of order: %+v", events[1])
	}
	if events[2].Best != "yes" || events[2].Observed != "score=0.9" {
		t.Fatalf("marker parse: %+v", events[2])
	}
}

func TestParseLedgerEventsMarkerlessAndBareLines(t *testing.T) {
	file := filepath.Join(t.TempDir(), "ledger.md")
	text := "# Mission Ledger\n\n- Cycle budget: 5\n- No-gain budget: 3\n\n" +
		"### Cycle 1\n- Classification: no-progress; candidate-sha=" + goodSHA + "; observed=score=1\n\n" +
		"### Cycle 2\n- Classification: unresolved\n"
	if err := atomicWriteText(file, text); err != nil {
		t.Fatal(err)
	}
	_, _, events, err := ParseLedgerEvents(file)
	if err != nil {
		t.Fatal(err)
	}
	if events[0].Best != "" || events[0].Observed != "score=1" {
		t.Fatalf("marker-less line: %+v", events[0])
	}
	// A bare classification line older tooling wrote degrades conservatively:
	// the verdict word stands and the observed value folds as baseline.
	if events[1].Classification != "unresolved" || events[1].Observed != "" {
		t.Fatalf("bare line: %+v", events[1])
	}
}
