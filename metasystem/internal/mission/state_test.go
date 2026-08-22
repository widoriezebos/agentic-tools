package mission

import (
	"bytes"
	"fmt"
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
	if err := InitStateWithBaseline(state, contract, ledger, "", "", strings.Repeat("b", 40), testAdmissionOrigins()); err != nil {
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
	if err := InitStateWithBaseline(state, contract, ledger, "", "", strings.Repeat("b", 40), testAdmissionOrigins()); err == nil {
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

// The PUBLIC state writer never lands a resolution:
// the custody check runs INSIDE WriteState on the
// single parsed proposal — no source swap between check and write — and
// only WriteStateResolution (resolve-taint's writer) may pass it.
func TestPublicWriterRefusesResolutionTransitions(t *testing.T) {
	_, state, _ := initMission(t)
	_, hash, _ := VerifyStateShape(state)
	source := state + ".src"

	// Booking an unresolved taint is not resolution-shaped: the public
	// writer allows it (the design's tamper-evident cooperative tier).
	doc, _ := readStateDoc(state)
	taint, _ := doc["workspaceTaint"].(map[string]any)
	taint["next"] = 2
	taint["entries"] = []any{map[string]any{
		"taintId": 1, "turnId": "m-t1", "reason": "drift",
		"setAt": "2026-08-18T00:00:00Z", "resolution": nil}}
	if err := atomicWriteJSON(source, doc); err != nil {
		t.Fatal(err)
	}
	if err := WriteState(state, source, hash); err != nil {
		t.Fatalf("an unresolved taint booking must pass the custody gate: %v", err)
	}
	_, hash, _ = VerifyStateShape(state)

	// A forged resolution refuses through the public writer...
	resolved, _ := readStateDoc(state)
	delete(resolved, "integrity")
	tree := strings.Repeat("a", 40)
	resolvedTaint, _ := resolved["workspaceTaint"].(map[string]any)
	resolvedTaint["segment"] = 1
	resolvedTaint["entries"] = []any{map[string]any{
		"taintId": 1, "turnId": "m-t1", "reason": "drift",
		"setAt": "2026-08-18T00:00:00Z", "resolution": map[string]any{
			"variant": "restore", "treeId": tree, "previousTree": tree,
			"sequencePoint": map[string]any{"sequence": 2, "segment": 1},
			"resolvedAt":    "2026-08-18T00:00:00Z", "resolvedBy": "impostor", "reason": "forged",
			"posture": testRecordedPosture()}}}
	if err := atomicWriteJSON(source, resolved); err != nil {
		t.Fatal(err)
	}
	if err := WriteState(state, source, hash); err == nil ||
		!strings.Contains(err.Error(), "resolutions land only through resolve-taint") {
		t.Fatalf("a resolution-shaped proposal must refuse, got %v", err)
	}
	// ...and the SAME proposal lands through the resolution writer.
	if err := WriteStateResolution(state, source, hash); err != nil {
		t.Fatalf("the resolution writer must land the lawful resolution: %v", err)
	}
	_, hash, _ = VerifyStateShape(state)

	// A bare segment move refuses too.
	moved, _ := readStateDoc(state)
	delete(moved, "integrity")
	movedTaint, _ := moved["workspaceTaint"].(map[string]any)
	movedTaint["segment"] = 2
	if err := atomicWriteJSON(source, moved); err != nil {
		t.Fatal(err)
	}
	if err := WriteState(state, source, hash); err == nil ||
		!strings.Contains(err.Error(), "segments move only through resolve-taint") {
		t.Fatalf("a segment move must refuse, got %v", err)
	}
}

// One write resolves at most one taint: two
// resolutions sharing a single occurrence would put two trees on one
// E-point.
func TestTransitionRefusesTwoResolutionsInOneWrite(t *testing.T) {
	_, state, _ := initMission(t)
	fresh, _ := readStateDoc(state)
	tree := strings.Repeat("d", 40)
	unresolvedPair := []any{
		map[string]any{"taintId": 1, "turnId": "m-t1", "reason": "a", "setAt": "2026-08-18T00:00:00Z", "resolution": nil},
		map[string]any{"taintId": 2, "turnId": "m-t2", "reason": "b", "setAt": "2026-08-18T00:00:00Z", "resolution": nil},
	}
	resolve := func(id int64) map[string]any {
		return map[string]any{"taintId": id, "turnId": fmt.Sprintf("m-t%d", id), "reason": string(rune('a' + id - 1)),
			"setAt": "2026-08-18T00:00:00Z", "resolution": map[string]any{
				"variant": "restore", "treeId": tree, "previousTree": tree,
				"sequencePoint": map[string]any{"sequence": 1, "segment": 2},
				"resolvedAt":    "2026-08-18T00:00:00Z", "resolvedBy": "Wido", "reason": "r"}}
	}
	before := map[string]any{}
	for k, v := range fresh {
		before[k] = v
	}
	before["workspaceTaint"] = map[string]any{"next": 3, "segment": 0, "entries": unresolvedPair}
	after := map[string]any{}
	for k, v := range fresh {
		after[k] = v
	}
	after["workspaceTaint"] = map[string]any{"next": 3, "segment": 2, "entries": []any{resolve(1), resolve(2)}}
	if err := validateTransition(before, after); err == nil ||
		!strings.Contains(err.Error(), "more than one expected-tree event") {
		t.Fatalf("two resolutions in one write must refuse, got %v", err)
	}

	// An acceptance ALONGSIDE a resolution shares the occurrence the
	// same way and refuses. The acceptance binds to
	// an open marker, so the beds open m-t3 first.
	openMarker := map[string]any{
		"turnId": "m-t3", "cycle": 1, "preTree": tree,
		"sequence": 0, "segment": 0, "openedAt": "2026-08-18T00:00:00Z",
		"headCommit": strings.Repeat("c", 40), "headTree": tree,
		"topTree": nil, "refMap": map[string]any{}, "topStaged": nil,
	}
	before["openTurn"] = openMarker
	acceptance := map[string]any{
		"turnId": "m-t3", "cycle": 1, "consumedAuthorizations": []any{},
		"wall": map[string]any{
			"verdict": "passed", "preTree": tree, "expectedTree": tree,
			"postTree": tree, "orderedDigests": []any{},
			"sequencePoint": map[string]any{"sequence": 1, "segment": 1},
		},
	}
	mixed := map[string]any{}
	for k, v := range fresh {
		mixed[k] = v
	}
	mixed["openTurn"] = openMarker
	mixed["turnLog"] = []any{acceptance}
	oneResolved := map[string]any{"taintId": 1, "turnId": "m-t1", "reason": "a",
		"setAt": "2026-08-18T00:00:00Z", "resolution": map[string]any{
			"variant": "restore", "treeId": tree, "previousTree": tree,
			"sequencePoint": map[string]any{"sequence": 1, "segment": 1},
			"resolvedAt":    "2026-08-18T00:00:00Z", "resolvedBy": "Wido", "reason": "r"}}
	mixed["workspaceTaint"] = map[string]any{"next": 3, "segment": 1, "entries": []any{
		oneResolved, unresolvedPair[1]}}
	if err := validateTransition(before, mixed); err == nil ||
		!strings.Contains(err.Error(), "more than one expected-tree event") {
		t.Fatalf("an acceptance beside a resolution must refuse, got %v", err)
	}

	// Two acceptances in one write refuse too — both can satisfy the
	// occurrence pin {prevSequence+1, segment}, which is exactly the
	// shared-occurrence hole the event count closes.
	segmentZero := func(entry map[string]any, turnID string) map[string]any {
		out := map[string]any{}
		for k, v := range entry {
			out[k] = v
		}
		wall := map[string]any{}
		for k, v := range out["wall"].(map[string]any) {
			wall[k] = v
		}
		wall["sequencePoint"] = map[string]any{"sequence": 1, "segment": 0}
		out["wall"] = wall
		out["turnId"] = turnID
		return out
	}
	firstAcceptance := segmentZero(acceptance, "m-t3")
	secondAcceptance := segmentZero(acceptance, "m-t3")
	double := map[string]any{}
	for k, v := range fresh {
		double[k] = v
	}
	double["openTurn"] = openMarker
	double["turnLog"] = []any{firstAcceptance, secondAcceptance}
	double["workspaceTaint"] = map[string]any{"next": 3, "segment": 0, "entries": unresolvedPair}
	if err := validateTransition(before, double); err == nil ||
		!strings.Contains(err.Error(), "more than one expected-tree event") {
		t.Fatalf("two acceptances in one write must refuse, got %v", err)
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
	if v, _ := intValue(doc["ledgerSemantics"]); v != 3 {
		t.Fatalf("init must pin ledgerSemantics 3, got %v", doc["ledgerSemantics"])
	}

	// The pin is immutable: a proposal that changes it is refused.
	_, hash, _ := VerifyStateShape(state)
	doc["ledgerSemantics"] = 2
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

// State v2 carries the wall: the exact legacy refusal precedes shape
// validation; the fresh state carries openTurn/workspaceTaint; acceptance
// payloads derive the consumption index and refuse duplicates; the taint
// ledger is monotonic across transitions.
func TestStateV2Wall(t *testing.T) {
	_, state, _ := initMission(t)
	doc, _ := readStateDoc(state)
	if v, _ := intValue(doc["schemaVersion"]); v != 4 {
		t.Fatalf("fresh state schemaVersion = %v, want 4 (the snapshot-scope schema)", doc["schemaVersion"])
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
			"sequencePoint":  map[string]any{"sequence": 1, "segment": 0},
			"headCommitPost": strings.Repeat("c", 40), "refMapPost": map[string]any{},
			"stagedTreePost": tree, "topTreePost": nil, "topStagedPost": nil,
			"worktreeCensusPost": []any{}, "capturedAt": "2026-01-01T00:00:00Z",
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

	// Entry-grain immutability: acceptance entries are
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
	resolution := map[string]any{"variant": "restore", "treeId": tree, "previousTree": tree,
		"sequencePoint": map[string]any{"sequence": 1, "segment": 1},
		"resolvedAt":    "2026-08-17T01:00:00Z", "resolvedBy": "Wido", "reason": "restored",
		"posture": testRecordedPosture()}
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
// through the REAL reconcile surface: no corrupt-state file, no recovery.
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

// Corrupt-state recovery never re-roots wall history: a
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
			"expectedTree": tree, "postTree": tree, "orderedDigests": []any{digest},
			"sequencePoint": map[string]any{"sequence": 1, "segment": 0}},
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

// The in-tree half of the downgrade claim: a
// schema-3 state carrying wall history that reaches a reconciler which
// cannot validate it is NEVER re-rooted — the corrupt-state recovery
// refusal holds for schema-invalid bytes exactly as for
// tampered ones, so state.json itself stays byte-identical. (The full
// cross-version proof needs a pre-semantics-3 binary and is recorded as
// the accepted residual: an old binary may write its runner record and a
// spurious corrupt-evidence copy, but can never mutate the state or
// write best markers.)
func TestReconcileRefusesRerootOnFutureSchema(t *testing.T) {
	root, state, ledger := initMission(t)
	doc, _ := readStateDoc(state)
	digest := strings.Repeat("d", 64)
	tree := strings.Repeat("e", 40)
	doc["schemaVersion"] = 99 // unvalidatable to EVERY binary
	doc["turnLog"] = []any{map[string]any{
		"turnId": "demo-t1", "consumedAuthorizations": []any{digest},
		"wall": map[string]any{"verdict": "passed", "preTree": tree,
			"expectedTree": tree, "postTree": tree, "orderedDigests": []any{digest},
			"sequencePoint": map[string]any{"sequence": 1, "segment": 0}},
	}}
	writeText(t, state, mustMarshalIndent(t, doc))
	before, _ := os.ReadFile(state)
	code, err := Reconcile(state, root, ledger)
	if code != 3 || err == nil || !strings.Contains(err.Error(), "automatic recovery refused") {
		t.Fatalf("future-schema wall history re-rooted: %d %v", code, err)
	}
	after, _ := os.ReadFile(state)
	if !bytes.Equal(before, after) {
		t.Fatal("reconciliation mutated a future-schema state")
	}
}

// The open-turn marker's transition discipline: a write may open
// a turn or conclude one, but never silently replace the turn in flight —
// that would mask a missed conclusion.
func TestOpenTurnImmutableWhileInFlight(t *testing.T) {
	_, state, _ := initMission(t)
	marker := func(turnID string) map[string]any {
		return map[string]any{
			"turnId": turnID, "cycle": 1,
			"preTree":  strings.Repeat("a", 40),
			"sequence": 0, "segment": 0,
			"openedAt":   "2026-08-18T00:00:00Z",
			"headCommit": strings.Repeat("c", 40),
			"headTree":   strings.Repeat("a", 40),
			"topTree":    nil, "refMap": map[string]any{}, "topStaged": nil,
		}
	}
	propose := func(mutate func(doc map[string]any)) error {
		_, hash, _ := VerifyStateShape(state)
		doc, _ := readStateDoc(state)
		mutate(doc)
		source := state + ".src"
		if err := atomicWriteJSON(source, doc); err != nil {
			t.Fatal(err)
		}
		return WriteState(state, source, hash)
	}
	if err := propose(func(doc map[string]any) { doc["openTurn"] = marker("turn-one") }); err != nil {
		t.Fatalf("opening a turn from null must be legal: %v", err)
	}
	err := propose(func(doc map[string]any) { doc["openTurn"] = marker("turn-two") })
	if err == nil || !strings.Contains(err.Error(), "openTurn is immutable") {
		t.Fatalf("replacing the turn in flight must be refused, got %v", err)
	}
	if err := propose(func(doc map[string]any) { doc["openTurn"] = nil }); err != nil {
		t.Fatalf("concluding the open turn must be legal: %v", err)
	}
}

// testAdmissionOrigins is a minimal valid admission-origins record for
// states born outside a live repository.
func testAdmissionOrigins() map[string]any {
	return map[string]any{
		"headCommit": strings.Repeat("c", 40), "topTree": nil, "topStaged": nil,
		"refMap": map[string]any{}, "worktreeCensus": []any{},
		"capturedAt": "2026-01-01T00:00:00Z",
	}
}

// testRecordedPosture is a minimal valid recorded carrier posture.
func testRecordedPosture() map[string]any {
	return map[string]any{
		"headCommitPost": strings.Repeat("c", 40), "refMapPost": map[string]any{},
		"stagedTreePost": strings.Repeat("a", 40), "topTreePost": nil, "topStagedPost": nil,
		"worktreeCensusPost": []any{}, "capturedAt": "2026-01-01T00:00:00Z",
	}
}

// A schema-2/3 state predates the snapshot-scope anchors and
// refuses resume with the named error — no migration machinery.
func TestPreSnapshotScopeStateRefusesByName(t *testing.T) {
	_, state, _ := initMission(t)
	doc, _ := readStateDoc(state)
	for _, version := range []int{2, 3} {
		doc["schemaVersion"] = version
		if err := validate(doc); err == nil || !strings.Contains(err.Error(), "predates the wall's snapshot scope") {
			t.Fatalf("schema %d must refuse by name, got %v", version, err)
		}
	}
}

// Two-phase enforcement: an acceptance write can neither
// surface success nor carry its own verification, and the admission
// origins are immutable identity.
func TestTwoPhaseAcceptanceSchema(t *testing.T) {
	_, state, _ := initMission(t)
	base, _ := readStateDoc(state)
	digest := strings.Repeat("c", 64)
	tree := strings.Repeat("d", 40)
	acceptance := map[string]any{
		"turnId": "demo-t1", "cycle": 1, "consumedAuthorizations": []any{digest},
		"wall": map[string]any{
			"verdict": "passed", "preTree": tree, "expectedTree": tree,
			"postTree": tree, "orderedDigests": []any{digest},
			"sequencePoint":  map[string]any{"sequence": 1, "segment": 0},
			"headCommitPost": strings.Repeat("c", 40), "refMapPost": map[string]any{},
			"stagedTreePost": tree, "topTreePost": nil, "topStagedPost": nil,
			"worktreeCensusPost": []any{}, "capturedAt": "2026-01-01T00:00:00Z"},
		"gatePassed": true,
	}
	marker := map[string]any{
		"turnId": "demo-t1", "cycle": 1, "preTree": tree,
		"sequence": 0, "segment": 0, "openedAt": "2026-01-01T00:00:00Z",
		"headCommit": strings.Repeat("c", 40), "headTree": tree,
		"topTree": nil, "refMap": map[string]any{}, "topStaged": nil,
	}
	verification := map[string]any{
		"turnId": "demo-t1", "kind": "wall-verification",
		"capturedAt": "2026-01-01T00:00:01Z", "verdict": "clean",
	}
	propose := func(mutate func(doc map[string]any)) error {
		doc, _ := deepCopyDoc(base).(map[string]any)
		mutate(doc)
		return validateTransition(base, doc)
	}
	// Success in the acceptance write refuses (previous already open,
	// so the binding rule is not what refuses here).
	openBase, _ := deepCopyDoc(base).(map[string]any)
	openBase["openTurn"] = marker
	successWrite, _ := deepCopyDoc(openBase).(map[string]any)
	successWrite["turnLog"] = []any{acceptance}
	successWrite["status"] = "completed"
	successWrite["gatePassed"] = true
	successWrite["parkReason"] = nil
	if err := validateTransition(openBase, successWrite); err == nil || !strings.Contains(err.Error(), "cannot surface success") {
		t.Fatalf("acceptance+completed in one write must refuse: %v", err)
	}
	// The acceptance BINDS to the already-open turn: no
	// marker, a foreign marker, and a cycle mismatch all refuse.
	if err := propose(func(doc map[string]any) {
		doc["openTurn"] = marker
		doc["turnLog"] = []any{acceptance}
	}); err == nil || !strings.Contains(err.Error(), "opened by an earlier write") {
		t.Fatalf("acceptance without a prior open turn must refuse: %v", err)
	}
	foreignBase, _ := deepCopyDoc(base).(map[string]any)
	foreignMarker, _ := deepCopyDoc(marker).(map[string]any)
	foreignMarker["turnId"] = "demo-t9"
	foreignBase["openTurn"] = foreignMarker
	foreignWrite, _ := deepCopyDoc(foreignBase).(map[string]any)
	foreignWrite["turnLog"] = []any{acceptance}
	if err := validateTransition(foreignBase, foreignWrite); err == nil || !strings.Contains(err.Error(), "must name the open turn") {
		t.Fatalf("acceptance naming a foreign turn must refuse: %v", err)
	}
	staleBase, _ := deepCopyDoc(base).(map[string]any)
	staleMarker, _ := deepCopyDoc(marker).(map[string]any)
	staleMarker["cycle"] = 2
	staleBase["openTurn"] = staleMarker
	staleWrite, _ := deepCopyDoc(staleBase).(map[string]any)
	staleWrite["turnLog"] = []any{acceptance}
	if err := validateTransition(staleBase, staleWrite); err == nil || !strings.Contains(err.Error(), "cycle must match") {
		t.Fatalf("acceptance with a stale cycle must refuse: %v", err)
	}
	// Acceptance and verification in ONE write refuses even with the
	// marker open and dying in the same write.
	if err := propose(func(doc map[string]any) {
		doc["turnLog"] = []any{acceptance, verification}
	}); err == nil {
		t.Fatal("same-write acceptance+verification must refuse")
	}
	withOpen, _ := deepCopyDoc(base).(map[string]any)
	withOpen["openTurn"] = marker
	sameWrite, _ := deepCopyDoc(withOpen).(map[string]any)
	sameWrite["openTurn"] = nil
	sameWrite["turnLog"] = []any{acceptance, verification}
	if err := validateTransition(withOpen, sameWrite); err == nil || !strings.Contains(err.Error(), "no committed acceptance") {
		t.Fatalf("same-write acceptance+verification must refuse by name: %v", err)
	}
	// A second acceptance for an already-accepted turn refuses.
	acceptedBase, _ := deepCopyDoc(base).(map[string]any)
	acceptedBase["openTurn"] = marker
	acceptedBase["turnLog"] = []any{acceptance}
	second, _ := deepCopyDoc(acceptedBase).(map[string]any)
	dup, _ := deepCopyDoc(acceptance).(map[string]any)
	dupWall, _ := dup["wall"].(map[string]any)
	dupWall["sequencePoint"] = map[string]any{"sequence": 1, "segment": 0}
	secondLog, _ := second["turnLog"].([]any)
	second["turnLog"] = append(secondLog, dup)
	if err := validateTransition(acceptedBase, second); err == nil || !strings.Contains(err.Error(), "already carries an acceptance") {
		t.Fatalf("a second acceptance for one turn must refuse: %v", err)
	}
	// Success without a verification write refuses even with no appended
	// acceptance: closing the marker and flipping completed in one bare
	// write is not a lawful transition.
	bare, _ := deepCopyDoc(acceptedBase).(map[string]any)
	bare["openTurn"] = nil
	bare["status"] = "completed"
	bare["gatePassed"] = true
	bare["parkReason"] = nil
	if err := validateTransition(acceptedBase, bare); err == nil || !strings.Contains(err.Error(), "post-verification write") {
		t.Fatalf("bare completion must refuse: %v", err)
	}
	// A verification of a gate-FAILED acceptance cannot carry completion.
	failedAcceptance, _ := deepCopyDoc(acceptance).(map[string]any)
	failedAcceptance["gatePassed"] = false
	withFailed, _ := deepCopyDoc(base).(map[string]any)
	withFailed["openTurn"] = marker
	withFailed["turnLog"] = []any{failedAcceptance}
	forged, _ := deepCopyDoc(withFailed).(map[string]any)
	forged["openTurn"] = nil
	forgedLog, _ := forged["turnLog"].([]any)
	forged["turnLog"] = append(forgedLog, verification)
	forged["status"] = "completed"
	forged["gatePassed"] = true
	forged["parkReason"] = nil
	if err := validateTransition(withFailed, forged); err == nil || !strings.Contains(err.Error(), "gate-failed acceptance") {
		t.Fatalf("completion over a gate-failed acceptance must refuse: %v", err)
	}
	// admissionOrigins is immutable identity.
	if err := propose(func(doc map[string]any) {
		origins, _ := doc["admissionOrigins"].(map[string]any)
		origins["headCommit"] = strings.Repeat("e", 40)
	}); err == nil || !strings.Contains(err.Error(), "immutable identity") {
		t.Fatalf("admissionOrigins rewrite must refuse: %v", err)
	}
}
