package batchowner

import (
	"cmp"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/counselor"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal/branch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/batch"
)

type LedgerTrunkRedOwner struct {
	Endpoint               goal.Endpoint
	Actor                  goal.Actor
	Now                    func() time.Time
	GitRead                func(root string, args ...string) (string, error)
	BeforeClearTransaction func() error
}

func NewLedgerTrunkRedOwnerWithConfig(root, machine, lineage string, lookup func(string, string) (string, error)) (batch.LedgerOwner, error) {
	resolve := goal.ResolveEndpoint
	if lookup != nil {
		resolve = func(root string) (goal.Endpoint, error) { return goal.ResolveEndpointWithConfig(root, lookup) }
	}
	endpoint, err := resolve(root)
	if err != nil {
		return nil, err
	}
	return &LedgerTrunkRedOwner{Endpoint: endpoint, Actor: goal.Actor{Machine: machine, Lineage: lineage}, Now: time.Now}, nil
}

// productionBatchLedgerOwner uses the landing identity that mints trunk-red operation identifiers.
func productionBatchLedgerOwner(root string) (batch.LedgerOwner, error) {
	return ProductionBatchLedgerOwnerWithConfig(root, nil)
}

func ProductionBatchLedgerOwnerWithConfig(root string, lookup func(string, string) (string, error)) (batch.LedgerOwner, error) {
	resolve := goal.ResolveMachine
	if lookup != nil {
		resolve = func(root string) (string, error) { return goal.ResolveMachineWithConfig(root, lookup) }
	}
	machine, err := resolve(root)
	if err != nil {
		return nil, err
	}
	return NewLedgerTrunkRedOwnerWithConfig(root, machine, LandingOwnerLineage, lookup)
}

func (owner LedgerTrunkRedOwner) Request(opid string) (goal.VerbRequest, error) {
	if len(opid) < 26 || opid != goal.Opid(opid[:26], owner.Actor.Machine, owner.Actor.Lineage) {
		return goal.VerbRequest{}, fmt.Errorf("trunk-red opid %q does not derive from the landing owner", opid)
	}
	now := time.Now
	if owner.Now != nil {
		now = owner.Now
	}
	return goal.VerbRequest{Endpoint: owner.Endpoint, Actor: owner.Actor, Ulid: opid[:26], Now: now().UTC()}, nil
}

func (owner LedgerTrunkRedOwner) Record(opid string, red batch.TrunkRed) ([]batch.EntryRef, error) {
	request, err := owner.Request(opid)
	if err != nil {
		return nil, err
	}
	args := goal.TrunkRedRecordArgs{Batch: red.BatchID, Attempt: red.AttemptID, BaseCommit: red.BaseCommit,
		BaseTree: red.BaseTree, SeenAt: red.SeenAt.UTC().Truncate(time.Second).Format(time.RFC3339), OwnerMachine: red.OwnerMachine()}
	args.Groups = batch.RedGroupsToRecordGroups(red.Groups)
	result, err := owner.runJournaled(opid, func() (goal.PublishResult, error) { return goal.RecordTrunkRed(request, args) })
	if err != nil {
		return nil, err
	}
	projection, err := goal.ProjectAtEndpoint(owner.Endpoint, result.Tip, request.Now)
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

func (owner LedgerTrunkRedOwner) Clear(opid string, ref batch.EntryRef, green batch.Green) error {
	request, err := owner.Request(opid)
	if err != nil {
		return err
	}
	projection, err := goal.Project(owner.Endpoint, false, request.Now)
	if err != nil {
		return err
	}
	gitRead := owner.GitRead
	if gitRead == nil {
		gitRead = branch.ScrubbedGit
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
		_, objectErr := gitRead(owner.Endpoint.Root, "cat-file", "-e", entry.FixBranch.Commit)
		if objectErr != nil {
			var exit *exec.ExitError
			if errors.As(objectErr, &exit) && exit.ExitCode() == 1 {
				if _, historyErr := gitRead(owner.Endpoint.Root, "-c", "core.commitGraph=false", "rev-list", "--quiet", green.BaseCommit); historyErr != nil {
					return historyErr
				}
				break
			}
			return objectErr
		}
		_, ancestryErr := gitRead(owner.Endpoint.Root, "merge-base", "--is-ancestor", entry.FixBranch.Commit, green.BaseCommit)
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
	if owner.BeforeClearTransaction != nil {
		if err := owner.BeforeClearTransaction(); err != nil {
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

func (owner LedgerTrunkRedOwner) Open() ([]batch.OpenEntry, error) {
	now := time.Now
	if owner.Now != nil {
		now = owner.Now
	}
	projection, err := goal.Project(owner.Endpoint, false, now().UTC())
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
		allowance, _ := time.Parse(time.RFC3339, entry.AllowanceUntil)
		open = append(open, batch.OpenEntry{ID: entry.ID, Group: entry.Group, OwnerMachine: entry.Owner.Machine,
			FixGoal: entry.FixGoal, Holds: append([]string(nil), entry.Holds...), LastBaseCommit: lastBaseCommit,
			Class: entry.EntryClass(), AllowanceUntil: allowance, Identity: entry.Identity, Owner: cmp.Or(entry.Owner.By, entry.Owner.Machine)})
	}
	return open, nil
}

func (owner LedgerTrunkRedOwner) runJournaled(opid string, publish func() (goal.PublishResult, error)) (goal.PublishResult, error) {
	entry, err := goal.ReadEntry(owner.Endpoint.Root, opid)
	if err == nil {
		if entry.Phase == goal.PhaseTerminal {
			if entry.Outcome != goal.OutcomeConfirmed && entry.Outcome != goal.OutcomeConfirmedLate {
				return goal.PublishResult{}, trunkRedJournalFailure(entry)
			}
			return owner.publishAndClassify(opid, publish)
		}
		owner.Endpoint.ConfigureCarriedCounselorAppend(counselor.AppendCarriedRow)
		if _, recoverErr := goal.Recover(owner.Endpoint); recoverErr != nil {
			return goal.PublishResult{}, recoverErr
		}
		entry, err = goal.ReadEntry(owner.Endpoint.Root, opid)
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

func (owner LedgerTrunkRedOwner) publishAndClassify(opid string, publish func() (goal.PublishResult, error)) (goal.PublishResult, error) {
	result, err := publish()
	if err != nil {
		if entry, readErr := goal.ReadEntry(owner.Endpoint.Root, opid); readErr == nil {
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
		if entry, readErr := goal.ReadEntry(owner.Endpoint.Root, opid); readErr == nil && entry.Phase == goal.PhaseTerminal {
			return goal.PublishResult{}, trunkRedJournalFailure(entry)
		}
		return goal.PublishResult{}, batch.ErrTrunkRedRecordPending
	}
	return result, nil
}

func trunkRedJournalFailure(entry goal.Entry) error {
	return &batch.TrunkRedRecordFailed{Outcome: string(entry.Outcome), Evidence: entry.Evidence}
}
