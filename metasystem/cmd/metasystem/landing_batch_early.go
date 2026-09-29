package main

import (
	"errors"
	"fmt"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/batch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/proofrun"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testpolicy/adapter"
)

// productionEarlySeams are the owner's early acts on the lane at root (D14,
// R27; U10b-3): the join's cheap phase on a waiting batch's recorded tip.
func productionEarlySeams(root string) batch.EarlySeams {
	return batch.EarlySeams{
		Cheap: func(record batch.Record) (batch.EarlyResult, error) {
			return earlyCheapPhase(root, record, runBatchAdmissionOnTree)
		},
		Adapter: func(group batch.RedGroup) (adapter.Adapter, bool) {
			if language := batchDiagnosisSeams.redLanguage(root); language != nil {
				return language(group)
			}
			return nil, false
		},
	}
}

type batchAdmissionRun func(root, batchID, baseTree, goalID string, claim batch.Claim, tree, label string,
	episode func(batch.JoinAdmission, int64) (batch.JoinAdmission, error)) (batch.JoinAdmission, proofrun.TestResult, bool, error)

// earlyCheapPhase runs the join's cheap phase once over the tip the joins
// recorded, which no join ran: each join proves its unit's tree alone. It is
// charged to the head member like the tip proof, and a fresh group gets an
// episode of its own that nothing retains.
func earlyCheapPhase(root string, record batch.Record, run batchAdmissionRun) (batch.EarlyResult, error) {
	var head batch.Unit
	for _, unit := range record.Units {
		if unit.State == batch.UnitJoined {
			head = unit
		}
	}
	if head.GoalID == "" {
		return batch.EarlyResult{}, fmt.Errorf("batch %s has no joined member", record.BatchID)
	}
	result, proof, _, err := run(root, record.BatchID, record.BaseTree, head.GoalID, head.Claim, record.TipTree, "early-cheap",
		func(decision batch.JoinAdmission, maxAgeMS int64) (batch.JoinAdmission, error) {
			token, err := newTestingFreshEpisode()
			if err != nil {
				return batch.JoinAdmission{}, err
			}
			now, err := goalCommandNow(batch.ModuleRoot(root))
			if err != nil {
				return batch.JoinAdmission{}, err
			}
			decision.FreshEpisode = token
			decision.FreshExpiresAt = now.Add(time.Duration(maxAgeMS) * time.Millisecond).UTC().Format(time.RFC3339Nano)
			return decision, nil
		})
	var red *batch.JoinAdmissionRed
	if errors.As(err, &red) {
		return batch.EarlyResult{Attempt: proof.AttemptID, Failing: batchRedAdapters(root, batch.ResultToRedGroups(proof))}, nil
	}
	if err != nil {
		return batch.EarlyResult{}, err
	}
	return batch.EarlyResult{Attempt: result.AttemptID}, nil
}
