//go:build batchtest

package main

import (
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/batch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/batchowner"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/proofrun"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testpolicy"
)

// TestBatchOwnerProofNamesTheEarlyRetryAndOwnsItsAttempt (R27, U3-03,
// U10b-3, fix round F-1): a sealed batch whose tip is a tree an earlier
// attempt of its head member failed (its early proof) launches its proof
// with the accountable retry decision naming that attempt, read from the
// retained proof store whatever the record kept, and the proof it records is
// its own launch, never the early one; a batch whose early red was on
// another tree carries none.
func TestBatchOwnerProofNamesTheEarlyRetryAndOwnsItsAttempt(t *testing.T) {
	previous := batchowner.BatchTipRetryAttempts
	t.Cleanup(func() { batchowner.BatchTipRetryAttempts = previous })
	for _, sameTree := range []bool{true, false} {
		root, id, _, record := batchProofRefusalBed(t)
		store := batch.NewStore(root, nil)
		earlyTree := record.TipTree
		if !sameTree {
			earlyTree = record.PrefixTrees[0]
		}
		batchowner.BatchTipRetryAttempts = func(string) ([]proofrun.Attempt, error) {
			return []proofrun.Attempt{{AttemptID: "proof-early-1", SchemaVersion: proofrun.CandidateAttemptSchemaVersion, CandidateGoalID: "goal-b",
				CandidateTree: earlyTree, TestAdmission: 3, Terminal: &proofrun.AttemptTerminal{Result: proofrun.TerminalFailed},
				TestResult: &proofrun.TestResult{Groups: []proofrun.GroupResult{{ID: "required", Status: "failed"}}}}}, nil
		}
		var launched batchowner.BatchProofLaunch
		deps := batchowner.BatchProofDependencies{
			Rearm: func(string, string) error { return nil },
			Plan: func(string, string, string, testpolicy.Mode) (testpolicy.Plan, error) {
				return testpolicy.Plan{RequiredMode: testpolicy.ModeStandard, ExecutedMode: testpolicy.ModeStandard, SelectedGroups: []string{"required"}}, nil
			},
			Launch: func(request batchowner.BatchProofLaunch) (proofrun.TestResult, error) {
				launched = request
				return proofrun.TestResult{AttemptID: "proof-tip-1", CandidateTree: request.Tree, Delivery: proofrun.DeliveryJudgment{Sufficient: true}}, nil
			},
		}
		if err := batchowner.ExecuteBatchProof(root, id, "owner", "full", "token", proofrun.LoadSample{}, time.Unix(10, 0), deps); err != nil {
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
