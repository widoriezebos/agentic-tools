package mission

import (
	"bytes"
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

// State v2 (host-implementer wall): the exact legacy refusal precedes shape
// validation; the fresh state carries openTurn/workspaceTaint; acceptance
// payloads derive the consumption index and refuse duplicates; the taint
// ledger is monotonic across transitions.
func TestStateV2Wall(t *testing.T) {
	_, state, _ := initMission(t)
	doc, _ := readStateDoc(state)
	if v, _ := intValue(doc["schemaVersion"]); v != 2 {
		t.Fatalf("fresh state schemaVersion = %v, want 2", doc["schemaVersion"])
	}
	if doc["openTurn"] != nil {
		t.Fatalf("fresh state has an open turn: %v", doc["openTurn"])
	}

	// A version-1 state gets the exact legacy refusal, not a shape error.
	legacy := map[string]any{}
	for k, v := range doc {
		legacy[k] = v
	}
	legacy["schemaVersion"] = 1
	delete(legacy, "integrity")
	if err := validate(func() map[string]any {
		legacy["integrity"] = map[string]any{}
		return legacy
	}()); err == nil || !strings.Contains(err.Error(), "state predates the host-implementer wall; re-provision the mission") {
		t.Fatalf("legacy refusal wrong: %v", err)
	}

	// Acceptance payloads: consumption replays; a duplicate digest refuses.
	digest := strings.Repeat("c", 64)
	tree := strings.Repeat("d", 40)
	accepted := map[string]any{
		"turnId": "demo-t1", "consumedAuthorizations": []any{digest},
		"wall": map[string]any{
			"verdict": "passed", "preTree": tree, "expectedTree": tree,
			"postTree": tree, "orderedDigests": []any{digest},
		},
	}
	doc["turnLog"] = []any{accepted}
	index, err := ConsumedAuthorizations(doc)
	if err != nil || index[digest] != "demo-t1" {
		t.Fatalf("consumption index wrong: %v %v", index, err)
	}
	second := map[string]any{}
	for k, v := range accepted {
		second[k] = v
	}
	second["turnId"] = "demo-t2"
	doc["turnLog"] = []any{accepted, second}
	if _, err := ConsumedAuthorizations(doc); err == nil || !strings.Contains(err.Error(), "twice") {
		t.Fatalf("duplicate consumption not refused: %v", err)
	}
	// A wall payload without consumption (or the reverse) is corrupt.
	doc["turnLog"] = []any{map[string]any{"turnId": "demo-t3", "wall": accepted["wall"]}}
	if _, err := ConsumedAuthorizations(doc); err == nil {
		t.Fatal("wall-only entry accepted")
	}

	// Entry-grain immutability (slice-4 F-1/F-3): acceptance entries are
	// append-only; taint facts freeze; a resolution lands once and bumps
	// the segment by exactly one; appended taints start unresolved.
	fresh, _ := readStateDoc(state)
	withTaint := func(taint map[string]any, log []any) map[string]any {
		out := map[string]any{}
		for k, v := range fresh {
			out[k] = v
		}
		out["workspaceTaint"] = taint
		if log != nil {
			out["turnLog"] = log
		}
		return out
	}
	entry := map[string]any{"taintId": 1, "turnId": "demo-t1", "reason": "drift", "setAt": "2026-08-17T00:00:00Z", "resolution": nil}
	resolution := map[string]any{"variant": "restore", "treeId": tree, "resolvedAt": "2026-08-17T01:00:00Z", "resolvedBy": "Wido", "reason": "restored"}
	resolvedEntry := map[string]any{"taintId": 1, "turnId": "demo-t1", "reason": "drift", "setAt": "2026-08-17T00:00:00Z", "resolution": resolution}
	tainted := withTaint(map[string]any{"next": 2, "segment": 0, "entries": []any{entry}}, nil)
	resolved := withTaint(map[string]any{"next": 2, "segment": 1, "entries": []any{resolvedEntry}}, nil)
	if err := validateTransition(tainted, resolved); err != nil {
		t.Fatalf("lawful resolution refused: %v", err)
	}
	unbumped := withTaint(map[string]any{"next": 2, "segment": 0, "entries": []any{resolvedEntry}}, nil)
	if err := validateTransition(tainted, unbumped); err == nil {
		t.Fatal("resolution without a segment bump accepted")
	}
	reopened := withTaint(map[string]any{"next": 2, "segment": 1, "entries": []any{entry}}, nil)
	if err := validateTransition(resolved, reopened); err == nil {
		t.Fatal("reopened resolution accepted")
	}
	rewritten := map[string]any{"taintId": 1, "turnId": "demo-t1", "reason": "REWRITTEN", "setAt": "2026-08-17T00:00:00Z", "resolution": nil}
	if err := validateTransition(tainted, withTaint(map[string]any{"next": 2, "segment": 0, "entries": []any{rewritten}}, nil)); err == nil {
		t.Fatal("rewritten taint fact accepted")
	}
	preResolved := map[string]any{"taintId": 2, "turnId": "demo-t2", "reason": "x", "setAt": "2026-08-17T02:00:00Z", "resolution": resolution}
	if err := validateTransition(tainted, withTaint(map[string]any{"next": 3, "segment": 1, "entries": []any{entry, preResolved}}, nil)); err == nil {
		t.Fatal("entry appended already-resolved accepted")
	}

	// Turn-log entries are append-only and immutable in place.
	logged := withTaint(map[string]any{"next": 1, "segment": 0, "entries": []any{}}, []any{accepted})
	if err := validateTransition(logged, withTaint(map[string]any{"next": 1, "segment": 0, "entries": []any{}}, []any{})); err == nil {
		t.Fatal("turn-log erasure accepted")
	}
	mutated := map[string]any{}
	for k, v := range accepted {
		mutated[k] = v
	}
	mutated["consumedAuthorizations"] = []any{}
	if err := validateTransition(logged, withTaint(map[string]any{"next": 1, "segment": 0, "entries": []any{}}, []any{mutated})); err == nil {
		t.Fatal("turn-log rewrite accepted")
	}
}

// A preserved v1 state reaches the operator as the exact named refusal
// through the REAL reconcile surface: no corrupt-state file, no recovery
// (slice-4 critique F-2).
func TestReconcileRefusesLegacyStateVerbatim(t *testing.T) {
	root, state, ledger := initMission(t)
	doc, _ := readStateDoc(state)
	doc["schemaVersion"] = 1
	delete(doc, "openTurn")
	delete(doc, "workspaceTaint")
	delete(doc, "integrity")
	finalized := map[string]any{}
	for k, v := range doc {
		finalized[k] = v
	}
	finalized["integrity"] = map[string]any{}
	rebuilt, err := finalizeNext(finalized, nil, nil)
	if err == nil {
		// finalizeNext validates; a v1 body correctly refuses there, so
		// write the legacy bytes directly instead.
		_ = rebuilt
	}
	finalized["integrity"] = map[string]any{"sequence": 0, "previousHash": nil,
		"hash": strings.Repeat("0", 64), "history": []any{}, "recoveryOf": nil}
	writeText(t, state, mustMarshalIndent(t, finalized))

	code, rerr := Reconcile(state, root, ledger)
	if code != 3 || rerr == nil || rerr.Error() != "mission resume refused: state predates the host-implementer wall; re-provision the mission" {
		t.Fatalf("legacy reconcile = %d %v", code, rerr)
	}
	matches, _ := filepath.Glob(filepath.Join(filepath.Dir(state), "state.corrupt.*"))
	if len(matches) != 0 {
		t.Fatalf("legacy refusal wrote corrupt-state files: %v", matches)
	}
}

// Corrupt-state recovery never re-roots wall history (slice-4 R2-1): a
// tampered state carrying an acceptance entry refuses automatic recovery,
// preserves evidence, and leaves the corrupt bytes in place for the human.
func TestReconcileRefusesToRerootWallHistory(t *testing.T) {
	root, state, ledger := initMission(t)
	doc, _ := readStateDoc(state)
	digest := strings.Repeat("e", 64)
	tree := strings.Repeat("f", 40)
	doc["turnLog"] = []any{map[string]any{
		"turnId": "demo-t1", "consumedAuthorizations": []any{digest},
		"wall": map[string]any{"verdict": "passed", "preTree": tree,
			"expectedTree": tree, "postTree": tree, "orderedDigests": []any{digest}},
	}}
	// The hash is now stale: this is exactly a tampered/corrupt state.
	writeText(t, state, mustMarshalIndent(t, doc))
	before, err := os.ReadFile(state)
	if err != nil {
		t.Fatal(err)
	}
	code, rerr := Reconcile(state, root, ledger)
	if code != 3 || rerr == nil || !strings.Contains(rerr.Error(), "automatic recovery refused") {
		t.Fatalf("wall-history re-root not refused: %d %v", code, rerr)
	}
	after, err := os.ReadFile(state)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(before, after) {
		t.Fatal("recovery rewrote the corrupt state despite the refusal")
	}
	matches, _ := filepath.Glob(filepath.Join(filepath.Dir(state), "state.corrupt.*"))
	if len(matches) != 1 {
		t.Fatalf("evidence not preserved exactly once: %v", matches)
	}
}
