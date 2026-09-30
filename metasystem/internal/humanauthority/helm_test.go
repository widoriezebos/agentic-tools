package humanauthority

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

// agentShellReader is the enrolled terminal (10 leader, 20 shell) with an
// agent beside the person: codex 70 under the leader and bash 80 under it,
// the person() shape of the helm command tests.
func agentShellReader() *treeReader {
	reader := enrolledReader()
	reader.snapshots[70] = []Snapshot{authoritySnapshot(70, 10, []string{"/opt/homebrew/bin/codex", "exec"}, "tty-1")}
	reader.snapshots[80] = []Snapshot{authoritySnapshot(80, 70, []string{"bash", "-c"}, "tty-1")}
	return reader
}

// prove is Prove with the helm seam supplied rather than read from AtHelm.
func prove(root string, invokerPID int64, reader Reader, now time.Time, atHelm func(string, int64) (HelmGrant, bool)) (Proof, error) {
	enrollment, err := ReadEnrollment(root)
	if err != nil {
		return Prove(root, invokerPID, reader, now)
	}
	return proveEnrolled(root, invokerPID, reader, now, enrollment, atHelm, nil)
}

func TestProveYieldsToTheHelmForTheAgentBesideThePerson(t *testing.T) {
	t.Parallel()
	root := authorityRoot(t)
	reader := agentShellReader()
	enrollTestTerminal(t, root, reader)
	enrollment, err := ReadEnrollment(root)
	if err != nil {
		t.Fatal(err)
	}
	var asked []int64
	grant := HelmGrant{By: "wido", Since: "2026-09-29T09:00:00Z", Class: "DELEGATE", Checkout: "/seat/.git"}
	atHelm := func(gotRoot string, pid int64) (HelmGrant, bool) {
		if gotRoot != root {
			t.Errorf("the helm was asked about %s, want %s", gotRoot, root)
		}
		asked = append(asked, pid)
		return grant, true
	}
	proof, err := prove(root, 80, reader, time.Unix(1100, 0), atHelm)
	if err != nil {
		t.Fatalf("the helm did not admit the agent beside the person: %v", err)
	}
	if !reflect.DeepEqual(asked, []int64{80}) {
		t.Fatalf("the helm was asked %v, want once for 80", asked)
	}
	if proof.Outcome != OutcomeProven || proof.Grade != GradeEnrolled || proof.Helm == nil || *proof.Helm != grant {
		t.Fatalf("helm proof = %+v", proof)
	}
	agentSeen := false
	for _, node := range proof.Nodes {
		agentSeen = agentSeen || node.AgentRuntime != nil
	}
	if len(proof.Nodes) != 2 || !agentSeen || proof.Nodes[0].Ref.PID != 80 {
		t.Fatalf("the helm proof does not keep the nodes as walked: %+v", proof.Nodes)
	}
	if !proof.Valid() || !proof.ValidFor(root) || !proof.TerminalValidFor(root) || !proof.EnrolledTerminalFor(root) || proof.ValidFor(t.TempDir()) {
		t.Fatal("the helm proof is not valid for its root, or valid for another")
	}
	if proof.ObservedTerminalID() != enrollment.TerminalID || proof.TerminalRef != enrollment.TerminalRef || proof.TerminalGeneration != enrollment.Generation {
		t.Fatalf("the helm proof does not carry the enrolled terminal: %+v %q", proof, proof.ObservedTerminalID())
	}
	if err := RecordProof(root, "op-helm", "goal approve", proof); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(root, "artifacts", "agents", "authority", "proofs", "op-helm.json"))
	if err != nil || !strings.Contains(string(data), `"helm": {`) || !strings.Contains(string(data), `"by": "wido"`) || !strings.Contains(string(data), `"class": "DELEGATE"`) {
		t.Fatalf("the proof record lacks the helm block: %v\n%s", err, data)
	}
	var parsed struct{ Proof Proof }
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatal(err)
	}
	if parsed.Proof.Helm == nil || *parsed.Proof.Helm != grant || parsed.Proof.Valid() {
		t.Fatalf("a parsed helm proof became authority: %+v", parsed.Proof)
	}

	// The walk that proves never asks the helm.
	reader.reads = map[int64]int{}
	asked = nil
	if proof, err := prove(root, 30, reader, time.Unix(1200, 0), atHelm); err != nil || proof.Helm != nil || len(asked) != 0 {
		t.Fatalf("a proven walk consulted the helm: %+v %v %v", proof, err, asked)
	}
}

func TestProveWithoutTheHelmRefusesAsToday(t *testing.T) {
	t.Parallel()
	root := authorityRoot(t)
	reader := agentShellReader()
	enrollTestTerminal(t, root, reader)
	walk := func(atHelm func(string, int64) (HelmGrant, bool)) (Proof, string) {
		reader.reads = map[int64]int{}
		proof, err := prove(root, 80, reader, time.Unix(1100, 0), atHelm)
		if err == nil {
			t.Fatalf("the agent shell was proven: %+v", proof)
		}
		return proof, err.Error()
	}
	today, todayErr := walk(nil)
	if today.Outcome != OutcomeAgent || today.Helm != nil {
		t.Fatalf("today's refusal changed: %+v", today)
	}
	declined, declinedErr := walk(func(string, int64) (HelmGrant, bool) { return HelmGrant{By: "wido"}, false })
	if !reflect.DeepEqual(today, declined) || todayErr != declinedErr {
		t.Fatalf("a helm that declines changed the refusal: %+v %q", declined, declinedErr)
	}
	reader.reads = map[int64]int{}
	exported, exportedErr := Prove(root, 80, reader, time.Unix(1100, 0))
	if AtHelm != nil || exportedErr == nil || !reflect.DeepEqual(today, exported) || exportedErr.Error() != todayErr {
		t.Fatalf("Prove with the library's nil seam is not today's refusal: %+v %v", exported, exportedErr)
	}

}

func TestHelmProofNeedsAHolderAndATime(t *testing.T) {
	t.Parallel()
	root := authorityRoot(t)
	now := time.Unix(1100, 0)
	if _, err := HelmProof(root, HelmGrant{By: " "}, now); err == nil {
		t.Fatal("a helm proof without a holder was minted")
	}
	if _, err := HelmProof(root, HelmGrant{By: "wido"}, time.Time{}); err == nil {
		t.Fatal("a helm proof without a time was minted")
	}
	// Without a readable enrollment the proof still names the holder, and
	// carries no terminal it did not observe.
	proof, err := HelmProof(root, HelmGrant{By: "wido", Class: "UNTRUSTED"}, now)
	if err != nil || !proof.ValidFor(root) || proof.ObservedTerminalID() != "" || proof.TerminalRef != (ProcessRef{}) {
		t.Fatalf("helm proof without an enrollment: %+v %v", proof, err)
	}
	proof.Helm.By = ""
	if proof.Valid() {
		t.Fatal("a helm proof with an empty holder is valid")
	}
}
