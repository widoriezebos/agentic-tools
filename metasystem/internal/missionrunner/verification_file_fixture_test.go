package missionrunner

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/gittree"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/mission"
)

type verificationFileBed struct {
	*recoveryFileBed
	facts       *verificationFacts
	anchorCalls int
	wantCycles  int
	wantLedger  []byte
}

type verificationFacts struct{ *recoveryFacts }

func newVerificationFileBed(t *testing.T) *verificationFileBed {
	t.Helper()
	r := newRecoveryFileBed(t)
	b := &verificationFileBed{recoveryFileBed: r, wantCycles: 0, wantLedger: append([]byte(nil), r.facts.original...)}
	f := &verificationFacts{r.facts}
	b.facts = f
	r.e.wallWorkspaceFactory = func(root string) wallWorkspace {
		if root != r.e.Root {
			t.Fatalf("workspace root %q, want %q", root, r.e.Root)
		}
		return f
	}
	r.e.wallReadFacts = f
	r.e.anchorFn = func(state, ledger, identity string) error {
		f.next("StateAnchor", state, ledger, identity)
		b.anchorCalls++
		if state != r.state || ledger != r.ledger || (identity != "acc" && identity != "alpha-t1-live") {
			t.Fatalf("anchor state/ledger/identity: %q %q %q", state, ledger, identity)
		}
		if _, _, err := mission.VerifyStateShape(state); err != nil {
			t.Fatalf("anchor state shape: %v", err)
		}
		_, _, cycles, err := mission.ParseLedger(ledger)
		if err != nil || len(cycles) != b.wantCycles {
			t.Fatalf("anchor ledger cycles = %d, want %d: %v", len(cycles), b.wantCycles, err)
		}
		current, err := os.ReadFile(ledger)
		if err != nil || !bytes.Equal(current, b.wantLedger) {
			t.Fatalf("anchor ledger bytes differ from expected bytes: %v", err)
		}
		return nil
	}
	return b
}

func (f *verificationFacts) HeadTree() (string, error) {
	r := f.next("HeadTree")
	return r.value.(string), r.err
}

func (f *verificationFacts) BlobOID(root string, content []byte) (string, error) {
	r := f.next("BlobOID", root, content)
	return r.value.(string), r.err
}

func (f *verificationFacts) LedgerTruth(root string, state map[string]any, path string) (string, string, error) {
	f.next("LedgerTruth", root, state, path)
	if root != f.bed.e.Root || path != f.bed.ledger || !reflect.DeepEqual(state, f.stateDoc()) {
		f.t.Fatalf("ledger truth requested for wrong root/state/path: %q %q", root, path)
	}
	current, err := os.ReadFile(path)
	if err != nil {
		return "", "", err
	}
	return string(f.original), string(current), nil
}

func (b *verificationFileBed) expectAnchor(identity string) {
	b.facts.add("StateAnchor", nil, b.state, b.ledger, identity)
}

func (b *verificationFileBed) checkAnchors(want int) {
	b.t.Helper()
	if b.anchorCalls != want {
		b.t.Fatalf("anchor calls = %d, want %d", b.anchorCalls, want)
	}
}

func (b *verificationFileBed) expectLedgerTruth() {
	b.facts.add("LedgerTruth", nil, b.e.Root, b.facts.stateDoc(), b.ledger)
}

func (b *verificationFileBed) expectCapture(head string) {
	f := b.facts
	ledger := []string{missionLedgerRel(b.e.Mission)}
	refs := f.refs()
	refs["refs/heads/main"] = head
	f.add("HistorySteeringFiles", []string{})
	f.add("HeadCommit", scopePolicyHead{oid: head})
	f.add("TreeOf", recoveryPre, head)
	f.add("FilterTree", recoveryPre, recoveryPre, ledger)
	f.add("SymbolicHead", scopePolicyBranch{ref: "refs/heads/main"})
	f.add("RefMap", refs)
	f.add("WorktreeCensus", []gittree.WorktreeRecord{{Path: b.e.Root, HeadOID: head, Branch: "refs/heads/main",
		PostureReadable: true, Staged: gittree.StagedPosture{Tree: recoveryPre}}})
	f.add("StagedTree", recoveryPre)
	f.add("FilterTree", recoveryPre, recoveryPre, ledger)
	f.add("Prefix", "")
	f.add("SnapshotSeeded", recoverySnapshot{post: recoveryPre}, head, recoveryPre, []string{})
	f.add("FilterTree", recoveryPre, recoveryPre, ledger)
}

func (b *verificationFileBed) accept() {
	b.t.Helper()
	b.expectCapture(recoveryHead)
	state := b.facts.stateDoc()
	open := state["openTurn"].(map[string]any)
	pre := open["preTree"].(string)
	capture, err := b.e.captureWallPosture(pre, nil)
	if err != nil {
		b.t.Fatal(err)
	}
	sequence, hash, err := mission.VerifyStateShape(b.state)
	if err != nil {
		b.t.Fatal(err)
	}
	wall := map[string]any{"verdict": "passed", "preTree": pre, "expectedTree": pre, "postTree": capture.Post,
		"orderedDigests": []any{}, "sequencePoint": map[string]any{"sequence": sequence + 1, "segment": 0}}
	for field, value := range capture.postureDoc(b.e.Mission) {
		wall[field] = value
	}
	proposed := b.facts.stateDoc()
	log, _ := proposed["turnLog"].([]any)
	proposed["turnLog"] = append(log, map[string]any{
		"turnId": "alpha-t1-live", "cycle": 1, "outcome": "completed", "detail": "host return accepted",
		"sessionId": nil, "measurement": nil, "accepted": []any{}, "rejected": []any{},
		"certified": []any{}, "factsForLedger": []any{}, "gaps": []any{},
		"wall": wall, "consumedAuthorizations": []any{}, "gatePassed": false,
	})
	if _, err := mission.AppendCycle(b.ledger, 1, "no-progress", strings.Repeat("a", 40), "observed=unmeasurable:test", ""); err != nil {
		b.t.Fatal(err)
	}
	b.wantCycles = 1
	proposed["ledger"].(map[string]any)["cycles"] = 1
	proposed["fences"].(map[string]any)["cycles"] = 1
	delete(proposed, "integrity")
	source := b.state + ".acc.src"
	if err := atomicWriteJSON(source, proposed); err != nil {
		b.t.Fatal(err)
	}
	if err := mission.WriteState(b.state, source, hash); err != nil {
		b.t.Fatalf("acceptance write: %v", err)
	}
	bytesAtAcceptance, err := os.ReadFile(b.ledger)
	if err != nil {
		b.t.Fatal(err)
	}
	b.facts.original = bytesAtAcceptance
	b.wantLedger = append([]byte(nil), bytesAtAcceptance...)
	b.expectAnchor("acc")
	if err := b.e.anchor(b.state, b.ledger, "acc"); err != nil {
		b.t.Fatal(err)
	}
	if pending := mission.UnverifiedAcceptance(b.facts.stateDoc()); pending != "alpha-t1-live" {
		b.t.Fatalf("expected pending acceptance, got %q", pending)
	}
	fences := readTestDoc(b.t, b.e.fencesPath())
	fences["cycles"] = 1
	writeJSONFile(b.t, b.e.fencesPath(), fences)
}

func (b *verificationFileBed) expectPark() {
	b.facts.add("Git", scopePolicyGit{stdout: recoveryHead + "\n"}, b.e.Root,
		[]string{"rev-parse", "--verify", "--quiet", "HEAD^{commit}"})
	b.facts.add("Git", scopePolicyGit{stdout: recoveryHead + "\n"}, b.e.Root,
		[]string{"-C", b.e.Root, "rev-parse", "main"})
	b.expectAnchor("alpha-t1-live")
}

func (b *verificationFileBed) expectLedgerCarrier() {
	b.facts.add("LedgerBlobOID", nil, b.e.Root, b.facts.stateDoc(), b.ledger)
	b.facts.add("Git", scopePolicyGit{code: 1}, b.e.Root, []string{"cat-file", "-e", recoveryPre})
	b.facts.add("AuthenticateLedger", nil, b.e.Root, b.facts.stateDoc(), b.ledger)
}

func (b *verificationFileBed) expectConclusion() {
	b.expectLedgerTruth()
	b.facts.add("Git", scopePolicyGit{}, b.e.Root,
		[]string{"update-ref", "-d", mission.MissionRefNamespace(b.e.Mission) + "turn-open-head"})
	b.expectAnchor("alpha-t1-live")
}

func (b *verificationFileBed) checkLedgerBytes() {
	b.t.Helper()
	current, err := os.ReadFile(b.ledger)
	if err != nil || !bytes.Equal(current, b.facts.original) {
		b.t.Fatalf("ledger bytes changed after acceptance: %v", err)
	}
}

var _ wallWorkspace = (*verificationFacts)(nil)
var _ wallReads = (*verificationFacts)(nil)

func (b *verificationFileBed) contractPath() string {
	return filepath.Join("plans", "mission-"+b.e.Mission+".contract.md")
}
