package goal

import (
	"fmt"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/readsubject"
)

func transferObligation() ReviewObligation {
	return ReviewObligation{OriginalEvidence: readsubject.Finding{ID: "read-1:1", Class: "regression", Severity: "high", Material: true, Where: "a.go", Claim: "Source requirement is incomplete", Evidence: "Required source behavior fails", Change: "Complete the source requirement"}, Finding: "read-1:1", Chain: "critic", Artifact: "a.go", Test: "destination must cover inherited finding", SourceUnit: "source", TargetUnit: "destination", OriginalRead: "read-1", OriginalFinding: "read-1:1", StopReference: "stop-1", SourceCommit: strings.Repeat("a", 40), TransferredOnce: true}
}

func TestTransferObligationPublicationAndCoverage(t *testing.T) {
	t.Parallel()
	endpoint := obligationAuthorityLocalEndpoint(t, "review-goal")
	req := verbReqFor(endpoint, "01J5X00000000000000000TR01", "mac-a")
	o := transferObligation()
	if result, err := DeferFindings(req, "review-goal", []ReviewObligation{o}); err != nil || result.Outcome != OutcomeConfirmed {
		t.Fatalf("publish: %+v %v", result, err)
	}
	req.Ulid = "01J5X00000000000000000TR02"
	if result, err := DeferFindings(req, "review-goal", []ReviewObligation{o}); err != nil || !result.Unchanged {
		t.Fatalf("replay: %+v %v", result, err)
	}
	projection, err := Project(endpoint, false, req.Now)
	if err != nil {
		t.Fatal(err)
	}
	got := projection.Tree.Live["review-goal"].ReviewObligations
	if len(got) != 1 || got[0].SourceCommit != o.SourceCommit || got[0].OriginalEvidence != o.OriginalEvidence || got[0].OriginalFinding != o.OriginalFinding || !got[0].TransferredOnce {
		t.Fatalf("transfer changed or duplicated: %+v", got)
	}
	req.Ulid = "01J5X00000000000000000TR03"
	if result, err := Done(req, "review-goal", "complete"); err != nil || result.Outcome != OutcomeRejected || !strings.Contains(result.Detail, "open review obligation") {
		t.Fatalf("premature done: %+v %v", result, err)
	}
	req.Ulid = "01J5X00000000000000000TR04"
	if result, err := DischargeReviewObligation(req, "review-goal", o.Finding, o.Chain, "mac-a", "clean but unrelated"); err != nil || result.Outcome != OutcomeRejected {
		t.Fatalf("unrelated clean discharged: %+v %v", result, err)
	}
	coverage := TransferCoverage{TargetUnit: o.TargetUnit, ReadID: "read-2", Commit: strings.Repeat("b", 40), Findings: []string{o.OriginalFinding}, SourceCommits: []string{o.SourceCommit}}
	req.Ulid = "01J5X00000000000000000TR05"
	if result, err := DischargeReviewObligation(req, "review-goal", o.Finding, o.Chain, "mac-a", "", DischargeEvidence{Transfer: &coverage}); err != nil || result.Outcome != OutcomeConfirmed {
		t.Fatalf("covered discharge: %+v %v", result, err)
	}
	projection, err = Project(endpoint, false, req.Now)
	if err != nil {
		t.Fatal(err)
	}
	got = projection.Tree.Live["review-goal"].ReviewObligations
	if got[0].State != "discharged" || got[0].CoverageRead != "read-2" || got[0].CoverageCommit != coverage.Commit || got[0].StopReference != o.StopReference {
		t.Fatalf("coverage not durable: %+v", got)
	}
}

func TestTransferRejectsCyclesAndSecondTransfer(t *testing.T) {
	t.Parallel()
	endpoint := obligationAuthorityLocalEndpoint(t, "review-goal")
	req := verbReqFor(endpoint, "01J5X00000000000000000TR11", "mac-a")
	o := transferObligation()
	if result, err := DeferFindings(req, "review-goal", []ReviewObligation{o}); err != nil || result.Outcome != OutcomeConfirmed {
		t.Fatalf("publish: %+v %v", result, err)
	}
	o.TargetUnit = "third"
	req.Ulid = "01J5X00000000000000000TR12"
	if result, err := DeferFindings(req, "review-goal", []ReviewObligation{o}); err != nil || result.Outcome != OutcomeRejected || !strings.Contains(result.Detail, "already been transferred") {
		t.Fatalf("second transfer: %+v %v", result, err)
	}
	o = transferObligation()
	o.Finding, o.OriginalFinding, o.OriginalRead, o.SourceUnit, o.TargetUnit = "read-2:1", "read-2:1", "read-2", "destination", "source"
	o.OriginalEvidence.ID = o.OriginalFinding
	req.Ulid = "01J5X00000000000000000TR13"
	if result, err := DeferFindings(req, "review-goal", []ReviewObligation{o}); err != nil || result.Outcome != OutcomeRejected || !strings.Contains(result.Detail, "cycle") {
		t.Fatalf("cyclic transfer: %+v %v", result, err)
	}
}

func TestTransferCoverageRequiresAllInheritedEvidence(t *testing.T) {
	t.Parallel()
	o := transferObligation()
	cases := []TransferCoverage{
		{TargetUnit: o.TargetUnit, ReadID: "read-2", Commit: "destination", Findings: []string{o.OriginalFinding}},
		{TargetUnit: o.TargetUnit, ReadID: "read-2", Commit: "destination", SourceCommits: []string{o.SourceCommit}},
		{TargetUnit: "other", ReadID: "read-2", Commit: "destination", Findings: []string{o.OriginalFinding}, SourceCommits: []string{o.SourceCommit}},
	}
	for _, coverage := range cases {
		if err := proveTransferCoverage(o, &coverage); err == nil {
			t.Fatalf("incomplete coverage accepted: %+v", coverage)
		}
	}
}

func TestCompleteTransfersPublishesAllCoverageAtomically(t *testing.T) {
	t.Parallel()
	endpoint := obligationAuthorityLocalEndpoint(t, "review-goal")
	req := verbReqFor(endpoint, "01J5X00000000000000000TR21", "mac-a")
	first := transferObligation()
	second := first
	second.Finding, second.OriginalFinding = "read-1:2", "read-1:2"
	second.OriginalEvidence.ID = second.OriginalFinding
	if result, err := DeferFindings(req, "review-goal", []ReviewObligation{first, second}); err != nil || result.Outcome != OutcomeConfirmed {
		t.Fatalf("publish transfer set: %+v %v", result, err)
	}
	coverage := TransferCoverage{TargetUnit: first.TargetUnit, ReadID: "read-2", Commit: strings.Repeat("b", 40), Findings: []string{first.OriginalFinding}, SourceCommits: []string{first.SourceCommit}}
	req.Ulid = "01J5X00000000000000000TR22"
	if result, err := CompleteTransfers(req, "review-goal", first.TargetUnit, coverage); err != nil || result.Outcome != OutcomeRejected {
		t.Fatalf("partial coverage published: %+v %v", result, err)
	}
	projection, err := Project(endpoint, false, req.Now)
	if err != nil {
		t.Fatal(err)
	}
	for _, obligation := range projection.Tree.Live["review-goal"].ReviewObligations {
		if obligation.State != "open" || obligation.CoverageRead != "" {
			t.Fatalf("partial completion changed a finding: %+v", obligation)
		}
	}
	coverage.Findings = append(coverage.Findings, second.OriginalFinding)
	req.Ulid = "01J5X00000000000000000TR23"
	if result, err := CompleteTransfers(req, "review-goal", first.TargetUnit, coverage); err != nil || result.Outcome != OutcomeConfirmed {
		t.Fatalf("complete coverage: %+v %v", result, err)
	}
	projection, err = Project(endpoint, false, req.Now)
	if err != nil {
		t.Fatal(err)
	}
	for _, obligation := range projection.Tree.Live["review-goal"].ReviewObligations {
		if obligation.State != "discharged" || obligation.CoverageRead != coverage.ReadID || obligation.CoverageCommit != coverage.Commit {
			t.Fatalf("coverage lost: %+v", obligation)
		}
	}
	req.Ulid = "01J5X00000000000000000TR24"
	if result, err := CompleteTransfers(req, "review-goal", first.TargetUnit, coverage); err != nil || !result.Unchanged {
		t.Fatalf("coverage replay performed another publication: %+v %v", result, err)
	}
}

func TestTransferredFindingRetainsCanonicalEvidence(t *testing.T) {
	t.Parallel()
	o := transferObligation()
	for _, mutation := range []func(*ReviewObligation){
		func(o *ReviewObligation) { o.OriginalEvidence.ID = "another-read:1" },
		func(o *ReviewObligation) { o.OriginalEvidence.Material = false },
		func(o *ReviewObligation) { o.OriginalEvidence.Evidence = "" },
		func(o *ReviewObligation) { o.OriginalEvidence.Where = "../outside.go" },
	} {
		invalid := o
		mutation(&invalid)
		if err := validateTransfer(invalid); err == nil {
			t.Fatalf("accepted evidence disconnected from the original read: %+v", invalid)
		}
	}
}

func TestTransferCannotRewriteOriginalEvidence(t *testing.T) {
	t.Parallel()
	endpoint := obligationAuthorityLocalEndpoint(t, "review-goal")
	req := verbReqFor(endpoint, "01J5X00000000000000000TR31", "mac-a")
	o := transferObligation()
	if result, err := DeferFindings(req, "review-goal", []ReviewObligation{o}); err != nil || result.Outcome != OutcomeConfirmed {
		t.Fatalf("publish: %+v %v", result, err)
	}
	for index, mutate := range []func(*ReviewObligation){
		func(o *ReviewObligation) { o.OriginalEvidence.Change = "A different correction" },
		func(o *ReviewObligation) { o.SourceCommit = strings.Repeat("b", 40) },
	} {
		incoming := o
		mutate(&incoming)
		req.Ulid = fmt.Sprintf("01J5X00000000000000000TR%d", 32+index)
		if result, err := DeferFindings(req, "review-goal", []ReviewObligation{incoming}); err != nil || result.Outcome != OutcomeRejected {
			t.Fatalf("rewritten original evidence accepted: %+v %v", result, err)
		}
	}
	projection, err := Project(endpoint, false, req.Now)
	if err != nil {
		t.Fatal(err)
	}
	got := projection.Tree.Live["review-goal"].ReviewObligations[0]
	if got.OriginalEvidence != o.OriginalEvidence || got.SourceCommit != o.SourceCommit {
		t.Fatalf("source provenance changed: %+v", got)
	}
}
