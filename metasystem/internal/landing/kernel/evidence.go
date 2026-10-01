package kernel

import (
	"errors"
	"fmt"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/gittree"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/batch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/custody"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/custody/laneprobe"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/lane"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/proofrun"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/strictjson"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testpolicy"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testrun"
)

// PublishEvidence is landing publish's reader (lane.PublishEvidence) of the
// records landing begin and landing prove keep on the lane checkout's batch
// store (K3, K4, K6): what is proven is what is published.
type PublishEvidence struct {
	Layout lane.Layout
	// Home is the host lane state whose custody barrier (K9) the retained
	// verification passes and is recorded in.
	Home string
	// Custody is the barrier's reads; nil reads the production probes.
	Custody func(home, install string) custody.Probes
	// Verifier is test verify's retained verification of one selection;
	// nil verifies nothing, so nothing is published.
	Verifier func(testrun.SelectionRequest) (proofrun.TestResult, error)
}

var _ lane.PublishEvidence = PublishEvidence{}

func (e PublishEvidence) opening(id string) (batch.Record, batch.Opening, error) {
	record, err := batch.NewStore(string(e.Layout.Checkout), nil).Load(id)
	if err != nil {
		return batch.Record{}, batch.Opening{}, err
	}
	opening, ok := record.CurrentOpening()
	if !ok {
		return record, batch.Opening{}, fmt.Errorf("batch %s has no series: landing begin has not run for it", id)
	}
	return record, opening, nil
}

// Begin is the batch's current series: B..the canonical candidate, its one
// tree, and the lane's own commits in it, which held accepts.
func (e PublishEvidence) Begin(id string) (lane.BatchBegin, error) {
	_, opening, err := e.opening(id)
	if err != nil {
		return lane.BatchBegin{}, err
	}
	begun := lane.BatchBegin{Batch: id, OpID: opening.OpID, Members: append([]string(nil), opening.Members...),
		Base: opening.Base, Head: opening.Candidate, Tree: opening.Tree}
	for _, commit := range opening.Series {
		if commit.Kind == batch.SeriesResolved || commit.Kind == batch.SeriesIntegration {
			begun.LaneCommits = append(begun.LaneCommits, commit.Commit)
		}
	}
	return begun, nil
}

// Proof is the batch's latest attempt of subject batch on its current
// series, with the policy and candidate engines its retained result names.
func (e PublishEvidence) Proof(id string) (lane.ProofAttempt, error) {
	record, opening, err := e.opening(id)
	if err != nil {
		return lane.ProofAttempt{}, err
	}
	for index := len(record.Attempts) - 1; index >= 0; index-- {
		attempt := record.Attempts[index]
		if attempt.Subject != batch.SubjectBatch || attempt.OpID != opening.OpID {
			continue
		}
		proof := lane.ProofAttempt{Batch: id, Attempt: attempt.ID, Subject: attempt.Subject, Outcome: attempt.Status,
			Base: opening.Base, Commit: attempt.Commit, Tree: attempt.Tree}
		if attempt.Status == batch.AttemptGreen {
			var result proofrun.TestResult
			if err := strictjson.Read(attempt.ResultPath, &result); err != nil {
				return lane.ProofAttempt{}, fmt.Errorf("attempt %s's retained result can't be read: %w", attempt.ID, err)
			}
			proof.PolicyEngineDigest, proof.CandidateEngineDigest = result.PolicyEngineDigest, result.CandidateEngineDigest
		}
		return proof, nil
	}
	return lane.ProofAttempt{}, fmt.Errorf("batch %s has no test run of the whole batch on its series", id)
}

// Verify runs test verify's retained verification of the proven tree as
// the batch tip it was proven as, charged to the lane, in a projection of
// that tree: it must be sufficient for exactly that tree.
func (e PublishEvidence) Verify(proof lane.ProofAttempt) error {
	if e.Verifier == nil {
		return errors.New("this engine can't check the lane's kept test results again, so nothing is published")
	}
	if e.Home == "" {
		return errors.New("the lane's host state is not named, so the retained verification can't be custodied")
	}
	probes := laneprobe.Production(e.Home, string(e.Layout.Install), true)
	if e.Custody != nil {
		probes = e.Custody(e.Home, string(e.Layout.Install))
	}
	// The one custody barrier (K9): the verification is a kernel execution,
	// run in this process, custodied until it ends.
	if err := custody.Clear(e.Home, probes, false); err != nil {
		return err
	}
	record, err := custody.Open(e.Home, custody.KindVerify, "batch "+proof.Batch+" attempt "+proof.Attempt+" tree "+proof.Tree, time.Now())
	if err != nil {
		return err
	}
	if err := custody.BindSelf(e.Home, record.ID); err != nil {
		return errors.Join(err, custody.End(e.Home, record.ID))
	}
	err = e.verify(proof)
	return errors.Join(err, custody.End(e.Home, record.ID))
}

func (e PublishEvidence) verify(proof lane.ProofAttempt) error {
	checkout := string(e.Layout.Checkout)
	detached, err := (gittree.Workspace{Dir: checkout}).NewDetachedWorktree(proof.Tree)
	if err != nil {
		return fmt.Errorf("project the proven tree %s: %w", proof.Tree, err)
	}
	defer detached.Close()
	result, err := e.Verifier(testrun.SelectionRequest{Root: string(e.Layout.Execution(lane.CheckoutRoot(detached.Workspace().Dir))),
		ControlRoot: string(e.Layout.Install), LaneID: lane.AccountID(checkout), LaneCheckout: checkout, Tree: proof.Tree,
		Mode: testpolicy.ModeAuto, Purpose: testpolicy.PurposeDelivery, BatchTipProof: true})
	if err != nil {
		return err
	}
	if !result.Delivery.Sufficient {
		return fmt.Errorf("the kept test results do not cover tree %s; missing groups: %v", proof.Tree, result.Delivery.MissingGroups)
	}
	// The judged candidate is named as a tree or a commit of it.
	judged, err := (gittree.Workspace{Dir: checkout}).TreeOf(result.CandidateTree)
	if err != nil || judged != proof.Tree {
		return fmt.Errorf("the retained verification judged %s, not the proven tree %s", result.CandidateTree, proof.Tree)
	}
	return nil
}
