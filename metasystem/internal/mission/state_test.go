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
