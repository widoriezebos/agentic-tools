package kernel

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/batch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/lane"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/proofrun"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testpolicy"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testrun"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/verbresult"
)

// K3, K4, K6 (integration of K-b and K-c): landing publish reads exactly
// what landing begin and landing prove recorded: the series B..candidate
// and its tree, and the batch's latest proof of subject batch, with the
// engines its retained result names; and it re-verifies that retained
// result for the proven tree, charged to the lane, before it publishes.
func TestPublishEvidenceReadsWhatBeginAndProveRecorded(t *testing.T) {
	t.Parallel()
	bed := newKernelBed(t)
	evidence := PublishEvidence{Layout: bed.layout}
	if _, err := evidence.Begin(bed.batchID); err == nil {
		t.Fatal("a batch with no begin read as begun")
	}
	outcome, err := bed.begin()
	if err != nil {
		t.Fatal(err)
	}
	opening := outcome.Opening
	if _, err := evidence.Proof(bed.batchID); err == nil {
		t.Fatal("a batch with no proof read as proven")
	}
	result := passed("feature-standard")
	result.PolicyEngineDigest, result.CandidateEngineDigest = "sha256:"+strings.Repeat("a", 64), "sha256:"+strings.Repeat("b", 64)
	executable, _ := bed.fakeChild("batch", result, verbresult.Result{Outcome: verbresult.Confirmed, Summary: "passed"}, 0)
	green, err := bed.prove(batch.SubjectBatch, executable)
	if err != nil || green.Status != batch.AttemptGreen {
		t.Fatalf("prove batch: %+v %v", green, err)
	}
	// A later triage of one member is not the batch's proof.
	executable, _ = bed.fakeChild("member", passed("feature-standard"), verbresult.Result{Outcome: verbresult.Confirmed, Summary: "passed"}, 0)
	if _, err := bed.prove(batch.SubjectMember+":"+bed.goalID, executable); err != nil {
		t.Fatal(err)
	}

	begun, err := evidence.Begin(bed.batchID)
	if err != nil {
		t.Fatal(err)
	}
	if begun.Batch != bed.batchID || begun.OpID != opening.OpID || begun.Base != opening.Base || begun.Head != opening.Candidate || begun.Tree != opening.Tree ||
		strings.Join(begun.Members, ",") != strings.Join(opening.Members, ",") {
		t.Fatalf("begin read %+v; want the recorded series %+v ending at its candidate", begun, opening)
	}
	proof, err := evidence.Proof(bed.batchID)
	if err != nil {
		t.Fatal(err)
	}
	want := lane.ProofAttempt{Batch: bed.batchID, Attempt: green.ID, Subject: lane.SubjectBatch, Outcome: lane.OutcomeGreen, Base: opening.Base,
		Commit: opening.Candidate, Tree: opening.Tree, PolicyEngineDigest: result.PolicyEngineDigest, CandidateEngineDigest: result.CandidateEngineDigest}
	if proof != want {
		t.Fatalf("proof read %+v; want %+v", proof, want)
	}

	var asked []testrun.SelectionRequest
	verdict := proofrun.TestResult{CandidateTree: opening.Tree}
	verdict.Delivery.Sufficient = true
	evidence.Verifier = func(request testrun.SelectionRequest) (proofrun.TestResult, error) {
		asked = append(asked, request)
		return verdict, nil
	}
	if err := evidence.Verify(proof); err != nil {
		t.Fatalf("a sufficient retained proof of the tree: %v", err)
	}
	request := asked[0]
	if request.Tree != opening.Tree || request.LaneID != lane.AccountID(string(bed.layout.Checkout)) || request.LaneCheckout != string(bed.layout.Checkout) ||
		request.ControlRoot != string(bed.layout.Install) || !request.BatchTipProof || request.Purpose != testpolicy.PurposeDelivery || request.Mode != testpolicy.ModeAuto {
		t.Fatalf("verify asked %+v; want the proven tree's delivery verification charged to the lane", request)
	}
	if filepath.Base(request.Root) != "metasystem" || strings.HasPrefix(request.Root, string(bed.layout.Checkout)+string(filepath.Separator)) {
		t.Fatalf("verify ran in %s; want the installation of its own projection of the tree", request.Root)
	}
	verdict.Delivery.Sufficient = false
	if err := evidence.Verify(proof); err == nil {
		t.Fatal("an insufficient retained proof verified")
	}
	verdict.Delivery.Sufficient, verdict.CandidateTree = true, bed.tree(bed.base)
	if err := evidence.Verify(proof); err == nil {
		t.Fatal("a retained proof of another tree verified")
	}
	evidence.Verifier = func(testrun.SelectionRequest) (proofrun.TestResult, error) {
		return proofrun.TestResult{}, errors.New("retained result digest moved")
	}
	if err := evidence.Verify(proof); err == nil {
		t.Fatal("a failed verification verified")
	}
	if err := (PublishEvidence{Layout: bed.layout}).Verify(proof); err == nil {
		t.Fatal("an evidence reader with no verifier verified")
	}
}
