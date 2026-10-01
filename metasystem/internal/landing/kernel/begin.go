package kernel

import (
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/gittree"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/batch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/batchowner"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/lane"
)

// BeginRequest is one landing begin: the batch's pinned members in series
// order, the base B and the agent's composed head C — or, with Conflict
// set, the agent's report that member Conflict failed to apply on Onto.
type BeginRequest struct {
	Home    string
	Layout  lane.Layout
	BatchID string
	Members []string
	Base    string
	Head    string
	// Actor is the lane's claim identity (machine+lineage).
	Actor    string
	Conflict string
	Onto     string
}

// BeginSeams are begin's clock, op-id minting and member authority.
type BeginSeams struct {
	Now   func() time.Time
	NewID func() (string, error)
	// Authorize reads a member's authority on the live ledger (its stop
	// fence, landing gate and budget): the join no longer does, so begin
	// does for every member before it records anything. It is handed the
	// lane's installation, where the ledger lives.
	Authorize func(install string, record batch.Record, unit batch.Unit) error
}

// ProductionBeginSeams are the production clock, ids and authority.
func ProductionBeginSeams() BeginSeams {
	return BeginSeams{Now: func() time.Time { return time.Now().UTC() }, NewID: newAttemptID, Authorize: authorizeMember}
}

// authorizeMember is a batch member's authority, read for a member
// the batch has not sealed through the claim it joined with.
func authorizeMember(install string, record batch.Record, unit batch.Unit) error {
	seal := map[string]batch.Claim{}
	for id, claim := range record.Seal {
		seal[id] = claim
	}
	if _, sealed := seal[unit.GoalID]; !sealed && !unit.IsChange() {
		seal[unit.GoalID] = unit.Claim
	}
	record.Seal = seal
	return batchowner.AuthorizeBatchMember(install, record, unit)
}

// BeginOutcome is what begin recorded: the opening (Changed false when the
// same series was already the batch's), or the composition evidence of a
// refusal.
type BeginOutcome struct {
	Opening  batch.Opening
	Changed  bool
	Evidence *batch.CompositionEvidence
}

// CandidateRef keeps a batch's canonical candidate reachable in the lane
// checkout until it is published.
func CandidateRef(batchID string) string { return "refs/metasystem/lane/" + batchID }

// Begin checks and writes the canonical series (K4) and durably records it
// under the pause (lane.Gate) before anything executes. A composition
// refusal (a member that does not apply on B, a series over the aggregate
// cap) is recorded as the evidence the members' return stands on and is
// returned as the *batch.CompositionRefusal; any other refusal records
// nothing.
func Begin(request BeginRequest, seams BeginSeams) (BeginOutcome, error) {
	checkout := string(request.Layout.Checkout)
	store := batch.NewStore(checkout, nil)
	record, err := store.Load(request.BatchID)
	if err != nil {
		return BeginOutcome{}, &Refusal{Code: batch.CodeBeginRefused, Reason: fmt.Sprintf("batch %s can't be read: %v", request.BatchID, err),
			Next: "run landing status to see the lane's batches"}
	}
	opID, err := seams.NewID()
	if err != nil {
		return BeginOutcome{}, err
	}
	at := seams.Now()
	if request.Conflict != "" {
		evidence, err := batch.CheckConflict(checkout, record, batch.ConflictRequest{BatchID: request.BatchID, Member: request.Conflict, Base: request.Base,
			Onto: request.Onto, Actor: request.Actor, OpID: opID, At: at})
		if err != nil {
			return BeginOutcome{}, err
		}
		if err := gated(request, at, func() error { return batch.RecordComposition(store, request.BatchID, evidence, at) }); err != nil {
			return BeginOutcome{}, err
		}
		return BeginOutcome{Evidence: &evidence}, nil
	}
	if seams.Authorize == nil {
		return BeginOutcome{}, errors.New("landing begin has no member authority to read")
	}
	for _, name := range request.Members {
		for _, unit := range record.Units {
			if unit.GoalID != name || unit.State != batch.UnitJoined {
				continue
			}
			if err := seams.Authorize(string(request.Layout.Install), record, unit); err != nil {
				return BeginOutcome{}, &Refusal{Code: batch.CodeBeginRefused, Reason: fmt.Sprintf("%s can't land now: %v", name, err),
					Next: "run landing status --verbose to see the member"}
			}
		}
	}
	opening, planErr := batch.PlanOpening(checkout, record, batch.BeginRequest{BatchID: request.BatchID, Members: request.Members, Base: request.Base,
		Head: request.Head, LedgerRoot: string(request.Layout.Install), Actor: request.Actor, At: at, OpID: opID})
	var composition *batch.CompositionRefusal
	if errors.As(planErr, &composition) {
		if err := gated(request, at, func() error { return batch.RecordComposition(store, request.BatchID, composition.Evidence, at) }); err != nil {
			return BeginOutcome{}, err
		}
		return BeginOutcome{Evidence: &composition.Evidence}, planErr
	}
	if planErr != nil {
		return BeginOutcome{}, planErr
	}
	outcome := BeginOutcome{}
	err = gated(request, at, func() error {
		if err := updateRef(checkout, CandidateRef(request.BatchID), opening.Candidate); err != nil {
			return err
		}
		recorded, changed, err := batch.RecordOpening(store, request.BatchID, opening, at)
		if err != nil {
			return err
		}
		current, _ := recorded.CurrentOpening()
		outcome = BeginOutcome{Opening: current, Changed: changed}
		return nil
	})
	return outcome, err
}

// gated runs one durable write of begin under the pause, in the lane it was
// prepared for.
func gated(request BeginRequest, at time.Time, write func() error) error {
	return lane.Gate(request.Home, lane.OpBegin, lane.AuthorityAgent, func(registered lane.Record) error {
		layout, err := registered.Layout()
		if err != nil || layout.Checkout != request.Layout.Checkout || layout.Install != request.Layout.Install {
			return &Refusal{Code: batch.CodeBeginRefused, Reason: "the landing lane moved while the series was checked, so nothing was recorded",
				Next: "run the same command again"}
		}
		// The session takes the batch up (K10): its allowance starts at the
		// session's clock, and one session takes up one batch.
		if err := lane.BindBatchHeld(request.Home, request.BatchID, at); err != nil {
			return err
		}
		return write()
	})
}

func updateRef(root, ref, commit string) error {
	command := exec.Command("git", "-C", root, "update-ref", ref, commit)
	command.Env = gittree.ScrubbedEnviron()
	if output, err := command.CombinedOutput(); err != nil {
		return fmt.Errorf("keep the candidate %s: %s: %w", commit, strings.TrimSpace(string(output)), err)
	}
	return nil
}
