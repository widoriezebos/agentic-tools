package main

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/batch"
)

// The gate at publication (g1-s70 D2, S70-05): a batch publishes later and on
// retries, and its final authority check reads a fresh ledger; the landing
// gate is read there too, so a hold published while the batch proved stops
// the publication and its retry, and the word must still be at the tip the
// member joined at.
func TestBatchPublicationReadsTheLandingGateAgainstTheFreshLedger(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "metasystem.conf"), []byte("metasystem.runtimes=fake\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC().Truncate(time.Second)
	tip := strings.Repeat("2", 40)
	claim := batch.Claim{Machine: "source", Lineage: "lineage", Epoch: 1, Revision: 2, AccountingRevision: 2}
	unit := batch.Unit{GoalID: "goal-gated", State: batch.UnitJoined, Claim: claim, BranchTip: tip, Chain: tip}
	record := batch.Record{BatchID: "01j5x00000000000000000ba99", Seal: map[string]batch.Claim{"goal-gated": claim}}
	landingOpid := goal.Opid("01ARZ3NDEKTSV4RRFFQ69G5FL1", "source", "lineage")
	file := &goal.GoalFile{Id: "goal-gated", State: goal.StateClaimed, Revision: 2, Tier: 2,
		Budget: &goal.Budget{ElapsedLimit: "1h", AttemptLimit: 2, ReservedJobMinutesLimit: 20, ActiveJobLimit: 1},
		Claimed: &goal.ClaimRecord{At: now.Add(-2 * time.Hour).Format(time.RFC3339), Revision: 2, AccountingRevision: 2,
			HandedOver: goal.HandedOver{FromMachine: claim.Machine, FromLineage: claim.Lineage, FromEpoch: claim.Epoch, Batch: record.BatchID}},
		History: []goal.HistoryLine{{At: now.Add(-3 * time.Hour).Format(time.RFC3339), Verb: "open"}, {At: now.Add(-2 * time.Hour).Format(time.RFC3339), Verb: "claim"},
			{At: now.Add(-time.Hour).Format(time.RFC3339), Opid: landingOpid, Verb: "land-ready", Actor: "source+lineage"},
			{At: now.Add(-50 * time.Minute).Format(time.RFC3339), Opid: goal.Opid("01ARZ3NDEKTSV4RRFFQ69G5FL2", "mac-ui", "m9"), Verb: "review", Actor: "human:Wido",
				Reason: "reviewed verdict=clear-to-land tip=" + tip + " record=plans/reviews/review-of-goal-gated.md by=Wido"}},
		Landing: &goal.LandingRecord{At: now.Add(-time.Hour).Format(time.RFC3339), Opid: landingOpid},
	}
	projection := goal.Projection{Tree: &goal.TreeGoals{Live: map[string]*goal.GoalFile{unit.GoalID: file}}}
	if err := authorizeBatchMemberInProjection(root, now, record, unit, projection); err != nil {
		t.Fatalf("a member cleared at its tip was refused: %v", err)
	}

	// A hold published while the batch proved: the publication and its retry
	// both refuse, and the refusal takes the member out of the batch.
	file.History = append(file.History, goal.HistoryLine{At: now.Add(-time.Minute).Format(time.RFC3339), Verb: "review", Actor: "human:Wido",
		Reason: goal.SittingReason(true, "plans/reviews/review-of-goal-gated.md", "Wido")})
	for attempt := 1; attempt <= 2; attempt++ {
		var ejected *batch.PrefixRevisionRefusal
		err := authorizeBatchMemberInProjection(root, now, record, unit, projection)
		if !errors.As(err, &ejected) || !strings.Contains(err.Error(), goal.GateHeldBySitting) {
			t.Fatalf("attempt %d: a hold published while proving did not stop the publication: %T %v", attempt, err, err)
		}
	}

	// Released, but the member joined at another tip than the word's.
	file.History = append(file.History, goal.HistoryLine{At: now.Format(time.RFC3339), Verb: "review", Actor: "human:Wido",
		Reason: goal.SittingReason(false, "plans/reviews/review-of-goal-gated.md", "Wido")})
	if err := authorizeBatchMemberInProjection(root, now, record, unit, projection); err != nil {
		t.Fatalf("a released hold still stops the publication: %v", err)
	}
	moved := unit
	moved.BranchTip = strings.Repeat("3", 40)
	if err := authorizeBatchMemberInProjection(root, now, record, moved, projection); err == nil || !strings.Contains(err.Error(), goal.GateWaitsForHuman) {
		t.Fatalf("a member at another tip than the word passed: %v", err)
	}
	// A chain member's tip is the commit the chain publishes: a word at it
	// passes, and a member whose head could not be read has none to bind to.
	chain := unit
	chain.Chain = "impl1"
	if err := authorizeBatchMemberInProjection(root, now, record, chain, projection); err != nil {
		t.Fatalf("a chain member cleared at its head was refused: %v", err)
	}
	chain.BranchTip = ""
	if err := authorizeBatchMemberInProjection(root, now, record, chain, projection); err == nil || !strings.Contains(err.Error(), "names no branch tip") {
		t.Fatalf("a chain member with no head passed: %v", err)
	}
}
