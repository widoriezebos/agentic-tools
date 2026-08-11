package mission

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeText(t *testing.T, path, body string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

// initMission writes a two-stream contract and initializes its state.
func initMission(t *testing.T) (root, state, ledger string) {
	t.Helper()
	root = t.TempDir()
	contract := filepath.Join(root, "mission-demo.contract.md")
	writeText(t, contract, "```mission\ncandidate.branch=feature-x\nstream.alpha=Do alpha\nstream.beta=Do beta\n```\n")
	state = filepath.Join(root, "state.json")
	ledger = filepath.Join(root, "ledger.md")
	if err := InitState(state, contract, ledger, "", ""); err != nil {
		t.Fatalf("init: %v", err)
	}
	return root, state, ledger
}

func TestInitProducesValidChainedState(t *testing.T) {
	_, state, _ := initMission(t)
	seq, hash, err := VerifyStateShape(state)
	if err != nil {
		t.Fatalf("fresh state should validate: %v", err)
	}
	if seq != 0 || !hashRe.MatchString(hash) {
		t.Fatalf("genesis state wrong: seq=%d hash=%q", seq, hash)
	}
	doc, _ := readStateDoc(state)
	if id, _ := doc["missionId"].(string); id != "demo" {
		t.Fatalf("missionId should come from the filename: %q", id)
	}
	if b, _ := doc["branch"].(string); b != "feature-x" {
		t.Fatalf("branch should come from the sealed contract: %q", b)
	}
	streams, _ := doc["streams"].(map[string]any)
	if len(streams) != 2 {
		t.Fatalf("both streams should be declared: %v", streams)
	}
}

func TestInitRefusesExisting(t *testing.T) {
	root, state, ledger := initMission(t)
	contract := filepath.Join(root, "mission-demo.contract.md")
	if err := InitState(state, contract, ledger, "", ""); err == nil {
		t.Fatal("init must refuse to overwrite an existing state")
	}
}

func TestCompareAndWriteAdvancesTheChain(t *testing.T) {
	_, state, _ := initMission(t)
	_, hash, _ := VerifyStateShape(state)

	// A legal transition: bump the cycle fence.
	doc, _ := readStateDoc(state)
	doc["fences"].(map[string]any)["cycles"] = 1
	source := state + ".src"
	if err := atomicWriteJSON(source, doc); err != nil {
		t.Fatal(err)
	}
	if err := WriteState(state, source, hash); err != nil {
		t.Fatalf("a legal compare-and-write should succeed: %v", err)
	}
	seq, newHash, _ := VerifyStateShape(state)
	if seq != 1 {
		t.Fatalf("the chain should advance to sequence 1, got %d", seq)
	}
	if newHash == hash {
		t.Fatal("the hash must change when the state changes")
	}

	// The stale expect hash is now refused.
	if err := WriteState(state, source, hash); err == nil ||
		!strings.Contains(err.Error(), "compare-and-write hash mismatch") {
		t.Fatalf("a stale expect hash must be refused, got %v", err)
	}
}

func TestWriteRefusesIllegalTransition(t *testing.T) {
	_, state, _ := initMission(t)
	_, hash, _ := VerifyStateShape(state)

	doc, _ := readStateDoc(state)
	doc["streams"].(map[string]any)["alpha"].(map[string]any)["goal"] = "a different goal"
	source := state + ".src"
	_ = atomicWriteJSON(source, doc)
	if err := WriteState(state, source, hash); err == nil || !strings.Contains(err.Error(), "goal is immutable") {
		t.Fatalf("changing a stream goal must be refused, got %v", err)
	}
}

func TestWriteRefusesDecreasingCounter(t *testing.T) {
	_, state, _ := initMission(t)
	// First advance cycles to 2.
	_, hash, _ := VerifyStateShape(state)
	doc, _ := readStateDoc(state)
	doc["fences"].(map[string]any)["cycles"] = 2
	source := state + ".src"
	_ = atomicWriteJSON(source, doc)
	if err := WriteState(state, source, hash); err != nil {
		t.Fatal(err)
	}
	// Now try to drop it back to 1.
	_, hash2, _ := VerifyStateShape(state)
	doc2, _ := readStateDoc(state)
	doc2["fences"].(map[string]any)["cycles"] = 1
	_ = atomicWriteJSON(source, doc2)
	if err := WriteState(state, source, hash2); err == nil || !strings.Contains(err.Error(), "cannot decrease") {
		t.Fatalf("a decreasing fence counter must be refused, got %v", err)
	}
}

func TestLedgerSemanticsPinnedAtInit(t *testing.T) {
	_, state, _ := initMission(t)
	doc, _ := readStateDoc(state)
	if v, _ := intValue(doc["ledgerSemantics"]); v != 2 {
		t.Fatalf("init must pin ledgerSemantics 2, got %v", doc["ledgerSemantics"])
	}

	// The pin is immutable: a proposal that changes it is refused.
	_, hash, _ := VerifyStateShape(state)
	doc["ledgerSemantics"] = 3
	source := state + ".src"
	_ = atomicWriteJSON(source, doc)
	if err := WriteState(state, source, hash); err == nil ||
		!strings.Contains(err.Error(), "immutable identity") {
		t.Fatalf("changing ledgerSemantics must be refused, got %v", err)
	}
}

func TestLegacyStateWithoutLedgerSemanticsStillValidates(t *testing.T) {
	_, state, _ := initMission(t)
	// Rebuild the same state without the field, as a pre-replay binary wrote
	// it, and re-chain it so only the missing field is under test.
	doc, _ := readStateDoc(state)
	delete(doc, "ledgerSemantics")
	delete(doc, "integrity")
	finalized, err := finalizeNext(doc, nil, nil)
	if err != nil {
		t.Fatalf("a state without ledgerSemantics must stay valid: %v", err)
	}
	legacy := filepath.Join(t.TempDir(), "legacy-state.json")
	if err := atomicWriteJSON(legacy, finalized); err != nil {
		t.Fatal(err)
	}
	if _, _, err := VerifyStateShape(legacy); err != nil {
		t.Fatalf("legacy state rejected: %v", err)
	}

	// A non-integer pin is a shape error, and a legacy state cannot gain the
	// field mid-mission.
	bad, _ := readStateDoc(legacy)
	bad["ledgerSemantics"] = "two"
	if err := validateShape(bad); err == nil {
		t.Fatal("a non-integer ledgerSemantics must be rejected")
	}
	_, legacyHash, _ := VerifyStateShape(legacy)
	upgraded, _ := readStateDoc(legacy)
	upgraded["ledgerSemantics"] = 2
	source := legacy + ".src"
	_ = atomicWriteJSON(source, upgraded)
	if err := WriteState(legacy, source, legacyHash); err == nil ||
		!strings.Contains(err.Error(), "immutable identity") {
		t.Fatalf("adding ledgerSemantics mid-mission must be refused, got %v", err)
	}
}

func TestDrainStalledParkReasonAndLabelValidate(t *testing.T) {
	_, state, _ := initMission(t)
	source := state + ".src"

	// The drain-stalled park is an admitted reason.
	_, hash, _ := VerifyStateShape(state)
	doc, _ := readStateDoc(state)
	doc["status"] = "parked"
	doc["parkReason"] = "drain-stalled"
	_ = atomicWriteJSON(source, doc)
	if err := WriteState(state, source, hash); err != nil {
		t.Fatalf("drain-stalled must be an admitted park reason: %v", err)
	}

	// The unpark writes the optional label; the heal's conclude write clears
	// it again. Both transitions are legal.
	_, hash, _ = VerifyStateShape(state)
	doc, _ = readStateDoc(state)
	doc["status"] = "running"
	doc["parkReason"] = nil
	doc["lastDrainStall"] = map[string]any{"cycle": 3, "survivors": []any{"job-a", "job-b"}}
	_ = atomicWriteJSON(source, doc)
	if err := WriteState(state, source, hash); err != nil {
		t.Fatalf("lastDrainStall must be an admitted optional field: %v", err)
	}
	_, hash, _ = VerifyStateShape(state)
	doc, _ = readStateDoc(state)
	delete(doc, "lastDrainStall")
	_ = atomicWriteJSON(source, doc)
	if err := WriteState(state, source, hash); err != nil {
		t.Fatalf("clearing lastDrainStall must be legal: %v", err)
	}

	// The label's shape is strict: an object with exactly {cycle, survivors},
	// a positive cycle, and job-id survivors.
	for name, value := range map[string]any{
		"not an object":     "stalled",
		"missing survivors": map[string]any{"cycle": 3},
		"extra field":       map[string]any{"cycle": 3, "survivors": []any{}, "note": "x"},
		"non-positive":      map[string]any{"cycle": 0, "survivors": []any{}},
		"bad survivor id":   map[string]any{"cycle": 1, "survivors": []any{"NOT A JOB ID"}},
		"non-array":         map[string]any{"cycle": 1, "survivors": "job-a"},
	} {
		bad, _ := readStateDoc(state)
		bad["lastDrainStall"] = value
		if err := validateShape(bad); err == nil {
			t.Fatalf("%s must be rejected", name)
		}
	}
}

func TestVerifyRejectsMalformedState(t *testing.T) {
	root := t.TempDir()
	bad := filepath.Join(root, "bad.json")
	writeText(t, bad, `{"schemaVersion":1,"missionId":"x"}`)
	if _, _, err := VerifyStateShape(bad); err == nil {
		t.Fatal("a state missing most fields must be rejected")
	}
}

func TestChainDetectsTamper(t *testing.T) {
	_, state, _ := initMission(t)
	// Tamper: flip the branch on disk without recomputing the hash.
	doc, _ := readStateDoc(state)
	doc["branch"] = "tampered"
	writeText(t, state, mustMarshalIndent(t, doc))
	if _, _, err := VerifyStateShape(state); err == nil ||
		!strings.Contains(err.Error(), "hash does not match") {
		t.Fatalf("a tampered state must fail the hash check, got %v", err)
	}
}

func mustMarshalIndent(t *testing.T, doc map[string]any) string {
	t.Helper()
	var b strings.Builder
	// Reuse the package writer's format by writing to a temp file would be
	// heavier; a plain indented marshal is enough to prove tamper detection.
	tmp := filepath.Join(t.TempDir(), "x.json")
	if err := atomicWriteJSON(tmp, doc); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(tmp)
	b.Write(data)
	return b.String()
}
