package main

import (
	"errors"
	"slices"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/batch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/proofrun"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testpolicy/adapter"
)

// quietEarlySeams are early acts that find nothing: a bed about something
// else runs them without effect.
func quietEarlySeams() batch.EarlySeams {
	return batch.EarlySeams{
		Cheap:   func(batch.Record) (batch.EarlyResult, error) { return batch.EarlyResult{}, nil },
		Adapter: func(batch.RedGroup) (adapter.Adapter, bool) { return nil, false },
	}
}

// earlyRecord is a waiting batch: goal-a and goal-b joined, goal-c
// withdrawn, on base "base" with the recorded tip "tip-ab".
func earlyRecord() batch.Record {
	claim := func(revision uint64) batch.Claim {
		return batch.Claim{Machine: "landing", Lineage: "owner", Epoch: 1, Revision: revision, AccountingRevision: revision + 1}
	}
	record := batch.Record{Schema: 1, BatchID: "01j5x00000000000000000ea01", State: batch.StateOpen, TipTree: "tip-ab",
		Units: []batch.Unit{{GoalID: "goal-a", State: batch.UnitJoined, Claim: claim(2)}, {GoalID: "goal-b", State: batch.UnitJoined, Claim: claim(4)},
			{GoalID: "goal-c", State: batch.UnitWithdrawn, Claim: claim(6)}}}
	record.BaseTree = "base"
	return record
}

// TestBatchEarlyCheapPhaseRunsTheJoinsChecksOnTheRecordedTip (R27, U10b-3):
// the production cheap phase is the join's own admission run, once, on the
// tip the joins recorded, charged to the head member; a fresh group gets an
// episode nothing retains; a red returns its groups and attempt, and any
// other failure is an error.
func TestBatchEarlyCheapPhaseRunsTheJoinsChecksOnTheRecordedTip(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	var calls []string
	var episode batch.JoinAdmission
	outcome := func(result batch.JoinAdmission, proof proofrun.TestResult, err error) batchAdmissionRun {
		return func(gotRoot, batchID, baseTree, goalID string, claim batch.Claim, tree, label string,
			fresh func(batch.JoinAdmission, int64) (batch.JoinAdmission, error)) (batch.JoinAdmission, proofrun.TestResult, bool, error) {
			calls = append(calls, gotRoot+"|"+batchID+"|"+baseTree+"|"+goalID+"|"+tree+"|"+label)
			if claim.Revision != 4 || claim.AccountingRevision != 5 {
				t.Fatalf("charged to the wrong claim: %+v", claim)
			}
			var freshErr error
			episode, freshErr = fresh(batch.JoinAdmission{Tree: tree, Status: "pending"}, 60_000)
			if freshErr != nil {
				t.Fatal(freshErr)
			}
			return result, proof, true, err
		}
	}
	result, err := earlyCheapPhase(root, earlyRecord(), outcome(batch.JoinAdmission{Status: "verified", AttemptID: "cheap-1"}, proofrun.TestResult{}, nil))
	if err != nil || result.Attempt != "cheap-1" || len(result.Failing) != 0 ||
		!slices.Equal(calls, []string{root + "|01j5x00000000000000000ea01|base|goal-b|tip-ab|early-cheap"}) ||
		len(episode.FreshEpisode) != 64 || episode.FreshExpiresAt == "" {
		t.Fatalf("green: result %+v err %v calls %v episode %+v", result, err, calls, episode)
	}

	red := proofrun.TestResult{AttemptID: "cheap-2", Groups: []proofrun.GroupResult{{ID: "go-unit", Status: "failed", LogPath: "/logs/unit.log"},
		{ID: "go-lint", Status: "passed"}}}
	result, err = earlyCheapPhase(root, earlyRecord(), outcome(batch.JoinAdmission{}, red, &batch.JoinAdmissionRed{Reason: "BATCH_JOIN_ADMISSION_RED: go-unit"}))
	if err != nil || result.Attempt != "cheap-2" || len(result.Failing) != 1 || result.Failing[0].ID != "go-unit" || result.Failing[0].LogPath != "/logs/unit.log" {
		t.Fatalf("red: result %+v err %v", result, err)
	}

	if _, err = earlyCheapPhase(root, earlyRecord(), outcome(batch.JoinAdmission{}, proofrun.TestResult{}, errors.New("BATCH_JOIN_TEST_DROPPED: x"))); err == nil {
		t.Fatal("a dropped run was not an error")
	}
}
