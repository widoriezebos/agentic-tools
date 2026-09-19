package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/batch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/trunkredmap"
)

type ledgerTrunkRedOwner struct {
	endpoint               goal.Endpoint
	actor                  goal.Actor
	now                    func() time.Time
	beforeClearTransaction func() error
}

func newLedgerTrunkRedOwner(root, machine, lineage string) (batch.LedgerOwner, error) {
	endpoint, err := goal.ResolveEndpoint(root)
	if err != nil {
		return nil, err
	}
	return &ledgerTrunkRedOwner{endpoint: endpoint, actor: goal.Actor{Machine: machine, Lineage: lineage}, now: time.Now}, nil
}

// productionBatchLedgerOwner uses the landing identity that mints trunk-red operation identifiers.
func productionBatchLedgerOwner(root string) (batch.LedgerOwner, error) {
	machine, err := goal.ResolveMachine(root)
	if err != nil {
		return nil, err
	}
	return newLedgerTrunkRedOwner(root, machine, landingOwnerLineage)
}

func isLedgerTrunkRedOwner(owner batch.LedgerOwner) bool {
	if _, ok := owner.(*ledgerTrunkRedOwner); ok {
		return true
	}
	_, ok := owner.(ledgerTrunkRedOwner)
	return ok
}

func (owner ledgerTrunkRedOwner) request(opid string) (goal.VerbRequest, error) {
	if len(opid) < 26 || opid != goal.Opid(opid[:26], owner.actor.Machine, owner.actor.Lineage) {
		return goal.VerbRequest{}, fmt.Errorf("trunk-red opid %q does not derive from the landing owner", opid)
	}
	now := time.Now
	if owner.now != nil {
		now = owner.now
	}
	return goal.VerbRequest{Endpoint: owner.endpoint, Actor: owner.actor, Ulid: opid[:26], Now: now().UTC()}, nil
}

func (owner ledgerTrunkRedOwner) Record(opid string, red batch.TrunkRed) ([]batch.EntryRef, error) {
	request, err := owner.request(opid)
	if err != nil {
		return nil, err
	}
	args := goal.TrunkRedRecordArgs{Batch: red.BatchID, Attempt: red.AttemptID, BaseCommit: red.BaseCommit,
		BaseTree: red.BaseTree, SeenAt: red.SeenAt.UTC().Truncate(time.Second).Format(time.RFC3339), OwnerMachine: red.OwnerMachine()}
	args.Groups = trunkredmap.RedGroupsToRecordGroups(red.Groups)
	result, err := owner.runJournaled(opid, func() (goal.PublishResult, error) { return goal.RecordTrunkRed(request, args) })
	if err != nil {
		return nil, err
	}
	projection, err := goal.ProjectAt(owner.endpoint.Root, result.Tip, request.Now)
	if err != nil {
		return nil, err
	}
	refs := goal.TrunkRedRefs(projection.Tree, opid)
	convertedRefs := make([]batch.EntryRef, len(refs))
	for index, ref := range refs {
		convertedRefs[index] = batch.EntryRef{ID: ref.ID, Group: ref.Group}
	}
	return convertedRefs, nil
}

func (owner ledgerTrunkRedOwner) Clear(opid string, ref batch.EntryRef, green batch.Green) error {
	request, err := owner.request(opid)
	if err != nil {
		return err
	}
	projection, err := goal.Project(owner.endpoint, false, request.Now)
	if err != nil {
		return err
	}
	branchMerged := false
	expectedEntry := goal.TrunkRedEntry{}
	for _, entry := range projection.Tree.TrunkRed {
		if entry.ID != ref.ID {
			continue
		}
		expectedEntry = entry
		if entry.FixBranch.Commit == "" {
			break
		}
		_, objectErr := goalBranchGit(owner.endpoint.Root, "cat-file", "-e", entry.FixBranch.Commit)
		if objectErr != nil {
			var exit *exec.ExitError
			if errors.As(objectErr, &exit) && exit.ExitCode() == 1 {
				if _, historyErr := goalBranchGit(owner.endpoint.Root, "-c", "core.commitGraph=false", "rev-list", "--quiet", green.BaseCommit); historyErr != nil {
					return historyErr
				}
				break
			}
			return objectErr
		}
		_, ancestryErr := goalBranchGit(owner.endpoint.Root, "merge-base", "--is-ancestor", entry.FixBranch.Commit, green.BaseCommit)
		if ancestryErr == nil {
			branchMerged = true
			break
		}
		var exit *exec.ExitError
		if !errors.As(ancestryErr, &exit) || exit.ExitCode() != 1 {
			return ancestryErr
		}
		break
	}
	if owner.beforeClearTransaction != nil {
		if err := owner.beforeClearTransaction(); err != nil {
			return err
		}
	}
	_, err = owner.runJournaled(opid, func() (goal.PublishResult, error) {
		return goal.ClearTrunkRed(request, goal.TrunkRedClearArgs{Entry: ref.ID, Attempt: green.AttemptID,
			BaseCommit: green.BaseCommit, BaseTree: green.BaseTree, Group: green.Group, BranchMerged: branchMerged,
			ExpectedEntry: expectedEntry})
	})
	return err
}

func (owner ledgerTrunkRedOwner) Open() ([]batch.OpenEntry, error) {
	now := time.Now
	if owner.now != nil {
		now = owner.now
	}
	projection, err := goal.Project(owner.endpoint, false, now().UTC())
	if err != nil {
		return nil, err
	}
	var open []batch.OpenEntry
	for _, entry := range projection.Tree.TrunkRed {
		if entry.Closed != nil {
			continue
		}
		lastBaseCommit := ""
		if len(entry.Sightings) > 0 {
			lastBaseCommit = entry.Sightings[len(entry.Sightings)-1].BaseCommit
		}
		open = append(open, batch.OpenEntry{ID: entry.ID, Group: entry.Group, OwnerMachine: entry.Owner.Machine,
			FixGoal: entry.FixGoal, Holds: append([]string(nil), entry.Holds...), LastBaseCommit: lastBaseCommit})
	}
	return open, nil
}

func (owner ledgerTrunkRedOwner) runJournaled(opid string, publish func() (goal.PublishResult, error)) (goal.PublishResult, error) {
	entry, err := goal.ReadEntry(owner.endpoint.Root, opid)
	if err == nil {
		if entry.Phase == goal.PhaseTerminal {
			if entry.Outcome != goal.OutcomeConfirmed && entry.Outcome != goal.OutcomeConfirmedLate {
				return goal.PublishResult{}, trunkRedJournalFailure(entry)
			}
			return owner.publishAndClassify(opid, publish)
		}
		configureCarriedCounselor(&owner.endpoint)
		if _, recoverErr := goal.Recover(owner.endpoint); recoverErr != nil {
			return goal.PublishResult{}, recoverErr
		}
		entry, err = goal.ReadEntry(owner.endpoint.Root, opid)
		if err != nil {
			return goal.PublishResult{}, err
		}
		if entry.Phase != goal.PhaseTerminal {
			return goal.PublishResult{}, batch.ErrTrunkRedRecordPending
		}
		if entry.Outcome != goal.OutcomeConfirmed && entry.Outcome != goal.OutcomeConfirmedLate {
			return goal.PublishResult{}, trunkRedJournalFailure(entry)
		}
		return owner.publishAndClassify(opid, publish)
	}
	if !errors.Is(err, os.ErrNotExist) {
		return goal.PublishResult{}, err
	}
	return owner.publishAndClassify(opid, publish)
}

func (owner ledgerTrunkRedOwner) publishAndClassify(opid string, publish func() (goal.PublishResult, error)) (goal.PublishResult, error) {
	result, err := publish()
	if err != nil {
		if entry, readErr := goal.ReadEntry(owner.endpoint.Root, opid); readErr == nil {
			if entry.Phase != goal.PhaseTerminal {
				return goal.PublishResult{}, batch.ErrTrunkRedRecordPending
			}
			if entry.Outcome != goal.OutcomeConfirmed && entry.Outcome != goal.OutcomeConfirmedLate {
				return goal.PublishResult{}, trunkRedJournalFailure(entry)
			}
		}
		return goal.PublishResult{}, err
	}
	if result.Outcome != goal.OutcomeConfirmed {
		if entry, readErr := goal.ReadEntry(owner.endpoint.Root, opid); readErr == nil && entry.Phase == goal.PhaseTerminal {
			return goal.PublishResult{}, trunkRedJournalFailure(entry)
		}
		return goal.PublishResult{}, batch.ErrTrunkRedRecordPending
	}
	return result, nil
}

func trunkRedJournalFailure(entry goal.Entry) error {
	return &batch.TrunkRedRecordFailed{Outcome: string(entry.Outcome), Evidence: entry.Evidence}
}
