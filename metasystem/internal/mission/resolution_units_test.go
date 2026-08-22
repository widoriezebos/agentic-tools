package mission

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Blankness is category-complete: whitespace, controls, and
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

// CurrentExpectedTree walks the E-sequence: the admitted baseline before
// any E-event, the accepted post-tree after an acceptance, the
// resolution tree after a ruling. A state with neither baseline nor
// events — the shape validation refuses and reservation fails closed
// on — yields empty.
func TestCurrentExpectedTree(t *testing.T) {
	tree0 := strings.Repeat("0", 40)
	tree1 := strings.Repeat("a", 40)
	tree2 := strings.Repeat("b", 40)
	state := map[string]any{"turnLog": []any{}, "workspaceTaint": map[string]any{"next": 1, "segment": 0, "entries": []any{}}}
	if got := CurrentExpectedTree(state); got != "" {
		t.Fatalf("a baseline-less shape must yield empty: %q", got)
	}
	state["initialBaseline"] = tree0
	if got := CurrentExpectedTree(state); got != tree0 {
		t.Fatalf("before any E-event the admitted baseline is expected: %q", got)
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
	if _, err := AppendCycle(ledger, 1, "no-progress", sha, "score=0", ""); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(ledger)
	if err != nil {
		t.Fatal(err)
	}
	if !pendingStampMatches(ledger, 1, string(data)) {
		t.Fatal("AppendCycle must stamp its post-write bytes")
	}
	if _, err := AppendAnnotations(ledger, 1, "", CappedAnnotation); err != nil {
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

// The ADMITTED initial baseline is E0 from mission birth:
// with no acceptance and no resolution, the expected tree is the
// recorded baseline — never empty — and no later write may change it.
func TestInitialBaselineIsE0(t *testing.T) {
	root := t.TempDir()
	contract := filepath.Join(root, "mission-demo.contract.md")
	writeText(t, contract, "```mission\ncandidate.branch=feature-x\nstream.alpha=Do alpha\n```\n")
	state := filepath.Join(root, "state.json")
	ledger := filepath.Join(root, "ledger.md")
	baseline := strings.Repeat("f", 40)
	if err := InitStateWithBaseline(state, contract, ledger, "", "", baseline, testAdmissionOrigins()); err != nil {
		t.Fatalf("init: %v", err)
	}
	doc, _ := readStateDoc(state)
	if got := CurrentExpectedTree(doc); got != baseline {
		t.Fatalf("the admitted baseline must be the expected tree from birth: %q", got)
	}
	points := ExpectedTreePoints(doc)
	if len(points) != 1 || points[0].Tree != baseline || points[0].Sequence != 0 || points[0].Segment != 0 {
		t.Fatalf("E0 must be the recorded baseline at {0,0}: %+v", points)
	}
	mutated, _ := readStateDoc(state)
	mutated["initialBaseline"] = strings.Repeat("a", 40)
	source := state + ".src"
	if err := atomicWriteJSON(source, mutated); err != nil {
		t.Fatal(err)
	}
	_, hash, _ := VerifyStateShape(state)
	if err := WriteState(state, source, hash); err == nil ||
		!strings.Contains(err.Error(), "immutable identity") {
		t.Fatalf("the baseline must be immutable, got %v", err)
	}
}

// A state that predates baseline recording reaches reconciliation with
// its NAMED diagnosis and never enters corrupt-state recovery.
func TestReconcileNamesThePreBaselineState(t *testing.T) {
	root := t.TempDir()
	contract := filepath.Join(root, "mission-demo.contract.md")
	writeText(t, contract, "```mission\ncandidate.branch=feature-x\nstream.alpha=Do alpha\n```\n")
	state := filepath.Join(root, "state.json")
	ledger := filepath.Join(root, "ledger.md")
	if err := InitStateWithBaseline(state, contract, ledger, "", "", strings.Repeat("f", 40), testAdmissionOrigins()); err != nil {
		t.Fatalf("init: %v", err)
	}
	if err := InitLedger(ledger, 5, 3); err != nil {
		t.Fatal(err)
	}
	// Strip the baseline and rewrite the document, leaving the integrity
	// hash stale on purpose: the missing-baseline refusal is checked
	// before any integrity validation, so this state and a genuine
	// baseline-less state (whose chain never covered the field) are
	// diagnosed identically.
	doc, _ := readStateDoc(state)
	delete(doc, "initialBaseline")
	if err := atomicWriteJSON(state, doc); err != nil {
		t.Fatal(err)
	}
	code, err := Reconcile(state, root, ledger)
	if code != 3 || err == nil || !errors.Is(err, ErrPreWallBaseline) {
		t.Fatalf("reconcile must pass the named refusal verbatim: code=%d err=%v", code, err)
	}
	corpses, _ := filepath.Glob(filepath.Join(root, "state.corrupt.*"))
	if len(corpses) != 0 {
		t.Fatalf("a baseline-less state must never enter corrupt-state recovery: %v", corpses)
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
	if _, err := AppendCycle(ledger, 1, "no-progress", strings.Repeat("d", 40), "score=0", ""); err != nil {
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
