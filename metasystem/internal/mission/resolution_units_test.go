package mission

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Blankness is category-complete (slice-6): whitespace, controls, and
// every Unicode format codepoint carry no attribution; any graphic
// non-space rune does.
func TestBlankStringCategories(t *testing.T) {
	blanks := []string{"", "   ", "\t\n", "\uFEFF", "\u200B\u200C\u200D", "\u2060", "\u2061", "\u2066", " \uFEFF\u2061 ", "\x00\x1f"}
	for _, s := range blanks {
		if !BlankString(s) {
			t.Fatalf("%q must be blank", s)
		}
	}
	content := []string{"Wido", " x ", "\uFEFFWido", "ĝ", "0", "-", "名"}
	for _, s := range content {
		if BlankString(s) {
			t.Fatalf("%q must count as content", s)
		}
	}
}

// The pending-block stamp is demanded byte-precisely: absent, cycle-
// mismatched, and byte-mismatched stamps all fail closed.
func TestPendingStampMatches(t *testing.T) {
	dir := t.TempDir()
	ledger := filepath.Join(dir, "ledger.md")
	content := "# Mission Ledger\n\n### Cycle 1\n- Classification: no-progress; candidate-sha=abc; observed=x\n"
	if pendingStampMatches(ledger, 1, content) {
		t.Fatal("an absent stamp must fail closed")
	}
	if err := stampLedgerPending(ledger, 1, content); err != nil {
		t.Fatal(err)
	}
	if !pendingStampMatches(ledger, 1, content) {
		t.Fatal("the exact stamped bytes must match")
	}
	if pendingStampMatches(ledger, 2, content) {
		t.Fatal("a cycle mismatch must fail closed")
	}
	if pendingStampMatches(ledger, 1, content+"- Note: appended later\n") {
		t.Fatal("any added byte must fail closed")
	}
}

// Only the heal's own lost-turn block shapes pass the no-marker gate.
func TestLostTurnBlockShapes(t *testing.T) {
	if !lostTurnBlock("### Cycle 3\n- Classification: no-progress; candidate-sha=abc; observed=unmeasurable:turn-lost\n") {
		t.Fatal("a turn-lost block is the heal's own shape")
	}
	if !lostTurnBlock("### Cycle 3\n- Classification: no-progress; candidate-sha=abc; observed=unmeasurable:drain-stalled\n- Drain: stalled:2\n") {
		t.Fatal("a drain-stalled block is the heal's own shape")
	}
	if lostTurnBlock("### Cycle 3\n- Classification: contract-improved; candidate-sha=abc; observed=score=1\n") {
		t.Fatal("a progress block is never a lost turn")
	}
	if lostTurnBlock("### Cycle 3\n- Classification: no-progress; candidate-sha=abc; observed=score=0\n") {
		t.Fatal("an ordinary no-progress block is not the heal's shape")
	}
}

// CurrentExpectedTree walks the E-sequence: empty before any E-event,
// the accepted post-tree after an acceptance, the resolution tree after
// a ruling.
func TestCurrentExpectedTree(t *testing.T) {
	tree1 := strings.Repeat("a", 40)
	tree2 := strings.Repeat("b", 40)
	state := map[string]any{"turnLog": []any{}, "workspaceTaint": map[string]any{"next": 1, "segment": 0, "entries": []any{}}}
	if got := CurrentExpectedTree(state); got != "" {
		t.Fatalf("no E-event yet: %q", got)
	}
	state["turnLog"] = []any{map[string]any{
		"turnId": "m-t1",
		"wall": map[string]any{"preTree": tree1, "postTree": tree1,
			"sequencePoint": map[string]any{"sequence": 1, "segment": 0}},
		"consumedAuthorizations": []any{},
	}}
	if got := CurrentExpectedTree(state); got != tree1 {
		t.Fatalf("after acceptance: %q", got)
	}
	state["workspaceTaint"] = map[string]any{"next": 2, "segment": 1, "entries": []any{
		map[string]any{"taintId": 1, "turnId": "m-t2", "reason": "drift",
			"setAt": "2026-08-19T00:00:00Z", "resolution": map[string]any{
				"variant": "restore", "treeId": tree2, "previousTree": tree1,
				"sequencePoint": map[string]any{"sequence": 2, "segment": 1},
				"resolvedAt":    "2026-08-19T00:00:00Z", "resolvedBy": "Wido", "reason": "ruled"}},
	}}
	if got := CurrentExpectedTree(state); got != tree2 {
		t.Fatalf("after resolution: %q", got)
	}
}

// Every ledger write leaves the byte-precise stamp behind — AppendCycle
// and the annotation append that mutates the final block.
func TestLedgerWritesStamp(t *testing.T) {
	dir := t.TempDir()
	ledger := filepath.Join(dir, "ledger.md")
	if err := InitLedger(ledger, 5, 3); err != nil {
		t.Fatal(err)
	}
	sha := strings.Repeat("c", 40)
	if err := AppendCycle(ledger, 1, "no-progress", sha, "score=0", ""); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(ledger)
	if err != nil {
		t.Fatal(err)
	}
	if !pendingStampMatches(ledger, 1, string(data)) {
		t.Fatal("AppendCycle must stamp its post-write bytes")
	}
	if err := AppendAnnotations(ledger, 1, CappedAnnotation); err != nil {
		t.Fatal(err)
	}
	mutated, err := os.ReadFile(ledger)
	if err != nil {
		t.Fatal(err)
	}
	if !pendingStampMatches(ledger, 1, string(mutated)) {
		t.Fatal("AppendAnnotations must re-stamp the mutated bytes")
	}
	if pendingStampMatches(ledger, 1, string(data)) {
		t.Fatal("the pre-annotation bytes must no longer match")
	}
}

// The other two ledger writers stamp as well: init and the vocal reset.
func TestInitAndResetWritesStamp(t *testing.T) {
	dir := t.TempDir()
	ledger := filepath.Join(dir, "ledger.md")
	if err := InitLedger(ledger, 5, 3); err != nil {
		t.Fatal(err)
	}
	initial, err := os.ReadFile(ledger)
	if err != nil {
		t.Fatal(err)
	}
	if !pendingStampMatches(ledger, 0, string(initial)) {
		t.Fatal("InitLedger must stamp its initial bytes")
	}
	if err := AppendCycle(ledger, 1, "no-progress", strings.Repeat("d", 40), "score=0", ""); err != nil {
		t.Fatal(err)
	}
	if err := AppendReset(ledger, "ask-1", "human reset for the stamp witness"); err != nil {
		t.Fatal(err)
	}
	afterReset, err := os.ReadFile(ledger)
	if err != nil {
		t.Fatal(err)
	}
	if !pendingStampMatches(ledger, 1, string(afterReset)) {
		t.Fatal("AppendReset must stamp its post-write bytes")
	}
}
