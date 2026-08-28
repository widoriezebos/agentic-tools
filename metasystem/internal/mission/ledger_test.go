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
	if _, err := AppendCycle(file, 1, "contract-improved", goodSHA, "score=0.5", ""); err != nil {
		t.Fatal(err)
	}
	if _, err := AppendCycle(file, 2, "no-progress", goodSHA, "score=0.5", ""); err != nil {
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
	if _, err := AppendCycle(file, 2, "no-progress", goodSHA, "x", ""); err == nil ||
		!strings.Contains(err.Error(), "must be 3") {
		t.Fatalf("appending a wrong cycle number must be refused, got %v", err)
	}
}

func TestAppendValidatesShaAndClassification(t *testing.T) {
	file := filepath.Join(t.TempDir(), "ledger.md")
	_ = InitLedger(file, 5, 3)
	if _, err := AppendCycle(file, 1, "bogus-class", goodSHA, "x", ""); err == nil {
		t.Fatal("unknown classification must be refused")
	}
	if _, err := AppendCycle(file, 1, "contract-improved", "not-a-sha", "x", ""); err == nil {
		t.Fatal("a non-sha candidate must be refused")
	}
	if _, err := AppendCycle(file, 1, "contract-improved", goodSHA, "", ""); err == nil {
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
	if _, err := AppendCycle(file, 1, "contract-improved", goodSHA, "score=0.6", "yes"); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "observed=score=0.6; best=yes\n") {
		t.Fatalf("best marker missing from the measurement line:\n%s", data)
	}
	if _, err := AppendCycle(file, 2, "unresolved", goodSHA, "score=0.6", "maybe"); err == nil {
		t.Fatal("an unknown best marker must be refused")
	}
}

func TestAppendResetGrammarAndSanitization(t *testing.T) {
	file := filepath.Join(t.TempDir(), "ledger.md")
	if err := InitLedger(file, 5, 3); err != nil {
		t.Fatal(err)
	}
	if _, err := AppendCycle(file, 1, "no-progress", goodSHA, "score=0.5", "no"); err != nil {
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

func TestAppendCycleAnnotations(t *testing.T) {
	file := filepath.Join(t.TempDir(), "ledger.md")
	if err := InitLedger(file, 5, 3); err != nil {
		t.Fatal(err)
	}
	if _, err := AppendCycle(file, 1, "contract-improved", goodSHA, "score=1", "yes",
		"Return: rejected:orchestrator return session identity mismatch", CappedAnnotation); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	// The classification line is untouched; the annotations are their own
	// lines inside the same cycle block.
	want := "- Classification: contract-improved; candidate-sha=" + goodSHA + "; observed=score=1; best=yes\n" +
		"- Return: rejected:orchestrator return session identity mismatch\n" +
		"- Outcome: capped\n"
	if !strings.Contains(string(data), want) {
		t.Fatalf("annotation lines misshapen:\n%s", data)
	}
	// Every parser tolerates and exposes them.
	_, _, cycles, err := ParseLedger(file)
	if err != nil || len(cycles) != 1 {
		t.Fatalf("annotated ledger must parse: %v (%d)", err, len(cycles))
	}
	wantAnnotations := []string{"Return: rejected:orchestrator return session identity mismatch", "Outcome: capped"}
	if len(cycles[0].Annotations) != 2 || cycles[0].Annotations[0] != wantAnnotations[0] || cycles[0].Annotations[1] != wantAnnotations[1] {
		t.Fatalf("annotations not exposed: %v", cycles[0].Annotations)
	}
	_, _, events, err := ParseLedgerEvents(file)
	if err != nil || len(events) != 1 {
		t.Fatalf("annotated events must parse: %v", err)
	}
	if len(events[0].Annotations) != 2 || events[0].Classification != "contract-improved" || events[0].Best != "yes" {
		t.Fatalf("event misread beside annotations: %+v", events[0])
	}
	// The next append stays contiguous: annotations never count as cycles.
	if _, err := AppendCycle(file, 2, "unresolved", goodSHA, "score=1", "no"); err != nil {
		t.Fatalf("append after annotations: %v", err)
	}
	// Only the two named annotation kinds may be written.
	if _, err := AppendCycle(file, 3, "unresolved", goodSHA, "score=1", "no", "Note: freeform"); err == nil {
		t.Fatal("an unknown annotation kind must be refused")
	}
	if _, err := AppendCycle(file, 3, "unresolved", goodSHA, "score=1", "no", "Return: rejected:a\nb"); err == nil {
		t.Fatal("a multi-line annotation must be refused")
	}
}

// AppendAnnotations is terminal delivery's write path: annotation lines land
// in the FINAL cycle's block only, under the strict write grammar, and every
// parser keeps reading around them.
func TestAppendAnnotationsToFinalBlock(t *testing.T) {
	file := filepath.Join(t.TempDir(), "ledger.md")
	if err := InitLedger(file, 5, 3); err != nil {
		t.Fatal(err)
	}
	if _, err := AppendCycle(file, 1, "unresolved", goodSHA, "score=1", "no"); err != nil {
		t.Fatal(err)
	}
	if _, err := AppendCycle(file, 2, "contract-improved", goodSHA, "score=2", "yes"); err != nil {
		t.Fatal(err)
	}
	rows := []string{
		LandedUnconsumedAnnotation("chain-a", "2", "artifacts/agents/chain-a/rounds/2/return.json"),
		LandedUnconsumedAnnotation("chain-b", "invalid", "artifacts/agents/chain-b/rounds/1/return.json"),
		LandedUnconsumedAnnotation("chain-c", "unreadable", "none"),
		LandedUnconsumedAnnotation("overflow", "3", "none"),
	}
	if _, err := AppendAnnotations(file, 2, "", rows...); err != nil {
		t.Fatalf("terminal delivery append refused: %v", err)
	}
	// Only the final block accepts the append; unknown kinds are refused.
	if _, err := AppendAnnotations(file, 1, "", rows[0]); err == nil {
		t.Fatal("an append to a closed history block must be refused")
	}
	if _, err := AppendAnnotations(file, 2, "", "Landed unconsumed: chain=UPPER round=x path="); err == nil {
		t.Fatal("an annotation outside the write grammar must be refused")
	}
	_, _, cycles, err := ParseLedger(file)
	if err != nil {
		t.Fatalf("annotated ledger must parse: %v", err)
	}
	if len(cycles) != 2 || len(cycles[1].Annotations) != 4 {
		t.Fatalf("the four landed rows must be exposed on cycle 2: %+v", cycles)
	}
	if cycles[1].Annotations[0] != "Landed unconsumed: chain=chain-a round=2 path=artifacts/agents/chain-a/rounds/2/return.json" {
		t.Fatalf("annotation misread: %q", cycles[1].Annotations[0])
	}
	// Replay parsing and later appends keep working around the annotations.
	_, _, events, err := ParseLedgerEvents(file)
	if err != nil || len(events) != 2 || len(events[1].Annotations) != 4 {
		t.Fatalf("events misread beside landed annotations: %v %+v", err, events)
	}
	if _, err := AppendCycle(file, 3, "no-progress", goodSHA, "score=2", "no"); err != nil {
		t.Fatalf("append after landed annotations: %v", err)
	}
}

// Terminal delivery is idempotent at line grain and pinned to the bytes
// the caller verified: a re-delivered annotation must
// not double, and an append over bytes that moved past the expectSHA
// proof must refuse under the lock.
func TestAppendAnnotationsIdempotentAndPinned(t *testing.T) {
	file := filepath.Join(t.TempDir(), "ledger.md")
	if err := InitLedger(file, 5, 3); err != nil {
		t.Fatal(err)
	}
	if _, err := AppendCycle(file, 1, "contract-improved", goodSHA, "score=1", "yes"); err != nil {
		t.Fatal(err)
	}
	row := LandedUnconsumedAnnotation("chain-a", "1", "artifacts/agents/chain-a/rounds/1/return.json")
	first, err := AppendAnnotations(file, 1, "", row)
	if err != nil {
		t.Fatalf("first delivery refused: %v", err)
	}
	again, err := AppendAnnotations(file, 1, first, row)
	if err != nil {
		t.Fatalf("re-delivery must be idempotent, got %v", err)
	}
	if again != first {
		t.Fatal("an already-present annotation must not change the ledger bytes")
	}
	if _, _, cycles, err := ParseLedger(file); err != nil || len(cycles[0].Annotations) != 1 {
		t.Fatalf("the annotation must appear exactly once: %v %+v", err, cycles)
	}
	// A stale pin — bytes moved past the caller's proof — refuses.
	fresh := LandedUnconsumedAnnotation("chain-b", "1", "none")
	if _, err := AppendAnnotations(file, 1, strings.Repeat("0", 64), fresh); err == nil ||
		!strings.Contains(err.Error(), "moved past the verified bytes") {
		t.Fatalf("a stale expectSHA must refuse: %v", err)
	}
}

func TestDrainStalledAnnotationGrammar(t *testing.T) {
	file := filepath.Join(t.TempDir(), "ledger.md")
	if err := InitLedger(file, 5, 3); err != nil {
		t.Fatal(err)
	}
	if _, err := AppendCycle(file, 1, "no-progress", goodSHA, DrainStalledObserved, "no", DrainStalledAnnotation(3)); err != nil {
		t.Fatalf("the drain-stalled annotation must be writable: %v", err)
	}
	_, _, cycles, err := ParseLedger(file)
	if err != nil || len(cycles) != 1 {
		t.Fatalf("annotated ledger must parse: %v (%d)", err, len(cycles))
	}
	if len(cycles[0].Annotations) != 1 || cycles[0].Annotations[0] != "Drain: stalled:3" {
		t.Fatalf("the survivor count must be exposed: %v", cycles[0].Annotations)
	}
	// The strict write grammar admits only a whole survivor count.
	for _, bad := range []string{"Drain: stalled:", "Drain: stalled:x", "Drain: stalled:-1", "Drain: stalled:03", "Drain: stalled"} {
		if _, err := AppendCycle(file, 2, "no-progress", goodSHA, DrainStalledObserved, "no", bad); err == nil {
			t.Fatalf("%q must be refused", bad)
		}
	}
	// The composer never emits a negative count.
	if got := DrainStalledAnnotation(-2); got != "Drain: stalled:0" {
		t.Fatalf("a negative count must clamp to zero: %q", got)
	}
}

func TestReturnRejectedAnnotationBounds(t *testing.T) {
	if got := ReturnRejectedAnnotation("line one\nline two"); got != "Return: rejected:line one line two" {
		t.Fatalf("newlines must flatten: %q", got)
	}
	if got := ReturnRejectedAnnotation("  \n "); got != "Return: rejected:unspecified" {
		t.Fatalf("an empty reason must not produce an empty annotation: %q", got)
	}
	long := ReturnRejectedAnnotation(strings.Repeat("x", 1000))
	if len(long) > len("Return: rejected:")+rejectedReasonMaxLen {
		t.Fatalf("over-long reasons must truncate: %d bytes", len(long))
	}
}

func TestParseLedgerToleratesHandWrittenAnnotationLines(t *testing.T) {
	// The tolerance is grammatical, not writer-specific: a block whose
	// annotation lines arrived from an older or foreign writer still parses
	// with exactly one classification per block.
	file := filepath.Join(t.TempDir(), "ledger.md")
	text := "# Mission Ledger\n\n- Cycle budget: 5\n- No-gain budget: 3\n\n" +
		"### Cycle 1\n- Classification: no-progress; candidate-sha=" + goodSHA + "; observed=score=1; best=no\n" +
		"- Return: rejected:host result has missing or unexpected fields\n" +
		"- Outcome: capped\n"
	if err := atomicWriteText(file, text); err != nil {
		t.Fatal(err)
	}
	_, _, cycles, err := ParseLedger(file)
	if err != nil || len(cycles) != 1 || len(cycles[0].Annotations) != 2 {
		t.Fatalf("hand-written annotations: %v (%+v)", err, cycles)
	}
}

func TestParseLedgerEventsOrderAndTokens(t *testing.T) {
	file := filepath.Join(t.TempDir(), "ledger.md")
	if err := InitLedger(file, 5, 3); err != nil {
		t.Fatal(err)
	}
	if _, err := AppendCycle(file, 1, "no-progress", goodSHA, "score=0.5", "no"); err != nil {
		t.Fatal(err)
	}
	if err := AppendReset(file, "stop-loss", "keep going"); err != nil {
		t.Fatal(err)
	}
	if _, err := AppendCycle(file, 2, "contract-improved", goodSHA, "score=0.9", "yes"); err != nil {
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

// The four Patience annotation forms round-trip through write and parse
// (records/patience/patience-satellite-4.md): the chain kind
// lives in the durable bytes, so the prompt projection needs no second
// source.
func TestPatienceAnnotationsRoundTrip(t *testing.T) {
	file := filepath.Join(t.TempDir(), "ledger.md")
	if err := InitLedger(file, 5, 3); err != nil {
		t.Fatal(err)
	}
	forms := []string{
		PatienceChainAnnotation("chain-a", 4, 2),
		PatienceOrphanAnnotation("orphan-b", 1),
		PatienceExcludedAnnotation(3),
		PatienceOverflowAnnotation(7),
	}
	if _, err := AppendCycle(file, 1, "no-progress", goodSHA, "score=0", "", forms...); err != nil {
		t.Fatalf("patience annotations refused by the write grammar: %v", err)
	}
	_, _, cycles, err := ParseLedger(file)
	if err != nil || len(cycles) != 1 {
		t.Fatalf("patience-annotated ledger must parse: %v", err)
	}
	if len(cycles[0].Annotations) != len(forms) {
		t.Fatalf("patience annotations lost in parse: %v", cycles[0].Annotations)
	}
	for i, want := range forms {
		if cycles[0].Annotations[i] != want {
			t.Fatalf("annotation %d: got %q want %q", i, cycles[0].Annotations[i], want)
		}
	}
	// The fuse replay never reads them: the replay must still see one plain
	// no-progress cycle and nothing else.
	if _, err := AppendCycle(file, 2, "contract-improved", goodSHA, "score=1", "yes"); err != nil {
		t.Fatalf("append after patience annotations: %v", err)
	}
	// Malformed patience lines are refused: zero rounds, orphan with a floor,
	// invalid identifiers.
	for _, bad := range []string{
		"Patience: chain=chain-a rounds=0 floor=2",
		"Patience: orphan=orphan-b rounds=1 floor=2",
		"Patience: chain=Bad_Chain rounds=1 floor=2",
		"Patience: excluded=0",
		"Patience overflow: chains=0",
	} {
		if _, err := AppendCycle(file, 3, "unresolved", goodSHA, "score=1", "no", bad); err == nil {
			t.Fatalf("malformed patience annotation accepted: %q", bad)
		}
	}
}
