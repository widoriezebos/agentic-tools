//go:build batchtest

package main

import (
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/batch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/proofrun"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testpolicy"
)

// TestBatchOwnerProofNamesTheEarlyRetryAndOwnsItsAttempt (R27, U3-03,
// U10b-3): a sealed batch whose tip is the tree its early proof failed
// launches its proof with the accountable retry decision naming the early
// attempt, and the proof it records is its own launch, never the early one;
// a batch whose tip moved past the early tree carries none.
func TestBatchOwnerProofNamesTheEarlyRetryAndOwnsItsAttempt(t *testing.T) {
	for _, sameTree := range []bool{true, false} {
		root, id, store, record := batchProofRefusalBed(t)
		earlyTree := record.TipTree
		if !sameTree {
			earlyTree = record.PrefixTrees[0]
		}
		if err := store.Update(id, func(current *batch.Record) error {
			current.Early = &batch.Early{Shape: []string{"goal-a", "goal-b"}, Tree: earlyTree, Cheap: "green", Proof: "red", Attempt: "proof-early-1",
				Finding: &batch.EarlyFinding{Group: "required", Attempt: "proof-early-1"}, Ended: "batch started"}
			return nil
		}); err != nil {
			t.Fatal(err)
		}
		var launched batchProofLaunch
		deps := batchProofDependencies{
			rearm: func(string, string) error { return nil },
			plan: func(string, string, string, testpolicy.Mode) (testpolicy.Plan, error) {
				return testpolicy.Plan{RequiredMode: testpolicy.ModeStandard, ExecutedMode: testpolicy.ModeStandard, SelectedGroups: []string{"required"}}, nil
			},
			launch: func(request batchProofLaunch) (proofrun.TestResult, error) {
				launched = request
				return proofrun.TestResult{AttemptID: "proof-tip-1", CandidateTree: request.Tree, Delivery: proofrun.DeliveryJudgment{Sufficient: true}}, nil
			},
		}
		if err := executeBatchProof(root, id, "owner", "full", "token", proofrun.LoadSample{}, time.Unix(10, 0), deps); err != nil {
			t.Fatal(err)
		}
		finished, err := store.Load(id)
		if err != nil {
			t.Fatal(err)
		}
		if finished.Proof == nil || finished.Proof.AttemptID != "proof-tip-1" || launched.Early {
			t.Fatalf("same tree %t: the batch's proof %+v, launched early %t", sameTree, finished.Proof, launched.Early)
		}
		if !sameTree {
			if launched.RetryDecision != "" {
				t.Fatalf("a batch past the early tree carried a retry decision: %s", launched.RetryDecision)
			}
			continue
		}
		var decision proofrun.RetryDecision
		data, err := os.ReadFile(launched.RetryDecision)
		if err != nil || json.Unmarshal(data, &decision) != nil || decision.PriorAttempt != "proof-early-1" || decision.SchemaVersion != 1 {
			t.Fatalf("retry decision %q: %+v %v", launched.RetryDecision, decision, err)
		}
	}
}
