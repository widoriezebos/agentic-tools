package batchowner

// The lane's returns (lane design r10 K7, K8): each member goes back to its
// seat, or is released, under the lane's claim identity, with its goal
// writes through the lane's publication boundary (K3).

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/gittree"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal/ownercall"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/batch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/lane"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/lease"
)

func goalFilesAt(root, tree string) ([]*goal.GoalFile, error) {
	command := exec.Command("git", "-C", root, "ls-tree", "-r", "--name-only", tree, "--", "plans/goals")
	command.Env = []string{"PATH=" + os.Getenv("PATH"), "LC_ALL=C"}
	out, err := command.Output()
	if err != nil {
		return nil, err
	}
	var files []*goal.GoalFile
	for _, name := range strings.Fields(string(out)) {
		if !strings.HasSuffix(name, ".md") || strings.HasSuffix(name, "/backlog.md") {
			continue
		}
		prefix, prefixErr := (gittree.Workspace{Dir: root}).Prefix()
		if prefixErr != nil {
			return nil, prefixErr
		}
		data, present, readErr := (gittree.Workspace{Dir: root}).FileAt(tree, prefix+name)
		if readErr != nil || !present {
			return nil, errors.Join(readErr, fmt.Errorf("goal file %s disappeared from %s", name, tree))
		}
		file, problems := goal.ParseFile(data)
		if len(problems) != 0 {
			return nil, fmt.Errorf("parse %s at %s: %v", name, tree, problems)
		}
		files = append(files, file)
	}
	return files, nil
}

var BatchReturnTargetSeams = struct {
	Goals         func(string, string) ([]*goal.GoalFile, error)
	Holder        func(string) (lease.CurrentHolderView, error)
	Announcements func(string, string) []lease.Announcement
}{goalFilesAt, lease.CurrentHolder, lease.AnnouncementsForOwnerLineage}

var BatchReturnLedgerGoal = batch.ReadReturnLedgerGoal

func ProductionReturnTarget(landingRoot, tree string, prober identity.Prober, unit batch.Unit) batch.ReturnTarget {
	files, err := BatchReturnTargetSeams.Goals(landingRoot, tree)
	if err != nil {
		return batch.ReturnTarget{State: batch.ReturnTargetUnknown, Reason: err.Error()}
	}
	for _, file := range files {
		if file.Id != unit.GoalID && file.Claimed != nil && file.Claimed.Machine == unit.Claim.Machine {
			return batch.ReturnTarget{State: batch.ReturnTargetOccupied, Reason: "source machine holds " + file.Id}
		}
	}
	if unit.SeatRoot == "" {
		return batch.ReturnTarget{State: batch.ReturnTargetUnknown, Reason: "source checkout root is absent"}
	}
	holder, holderErr := BatchReturnTargetSeams.Holder(unit.SeatRoot)
	announcements := BatchReturnTargetSeams.Announcements(unit.SeatRoot, unit.Claim.Lineage)
	seenDead := false
	for _, announcement := range announcements {
		ref := identity.Ref{Pid: announcement.Pid, StartedAtSec: announcement.PidStartedAt,
			StartTicks: announcement.PidStartTicks, BootID: announcement.BootID}
		switch identity.AliveRef(prober, ref) {
		case identity.Alive:
			if holderErr == nil && holder.MainId == announcement.MainId && holder.OwnerLineage == unit.Claim.Lineage && holder.ClaimEpoch > 0 {
				return batch.ReturnTarget{State: batch.ReturnTargetLive, Epoch: uint64(holder.ClaimEpoch)}
			}
			return batch.ReturnTarget{State: batch.ReturnTargetRestarted}
		case identity.Dead:
			seenDead = true
		}
	}
	if holderErr == nil && holder.OwnerLineage != "" && holder.OwnerLineage != unit.Claim.Lineage {
		return batch.ReturnTarget{State: batch.ReturnTargetRestarted}
	}
	if seenDead {
		return batch.ReturnTarget{State: batch.ReturnTargetDead}
	}
	return batch.ReturnTarget{State: batch.ReturnTargetUnknown, Reason: "source identity is not provable"}
}

// LaneCallSet is the ledger and landing owners the landing path calls
// in its own process, each under an explicit invocation context (design 6.2):
// the process making the call is the supplied identity, and the lineage is
// named, never inherited. The engine sets the production set at start; tests
// replace it.
type LaneCallSet struct {
	Handover func(ownercall.Invocation, ownercall.HandoverRequest) error
	EditNext func(invocation ownercall.Invocation, root, goalID, next string) error
	Release  func(invocation ownercall.Invocation, root, goalID string) error
	Held     func(root, base, commit, remote, ref string) error
}

var LaneCalls LaneCallSet

// laneLedger routes goal writes through the lane's publication boundary
// under the home that names the lane, as op for authority. A host with no
// home keeps no host state and so no lane (as LandingLaneSeams.Resolve
// reads it): its writes are not the lane's, and publish as before.
func laneLedger(home func() (string, error), op lane.Operation, authority lane.Authority) func(goal.Endpoint) goal.Endpoint {
	return func(endpoint goal.Endpoint) goal.Endpoint {
		at, err := home()
		if err != nil {
			return endpoint
		}
		return lane.LedgerEndpoint(at, endpoint, op, authority)
	}
}

// returnSeamsAt are the return seams of the lane whose checkout is root and
// whose installation (ledger) is controlRoot, publishing through calls, read
// when each is made. Each goal goes back under the lane's claim identity,
// read from the host record under home (claimAuthority); boundary is the
// publication boundary every goal write of the return goes through (K3):
// the agent's, or a person's cleanup.
func returnSeamsAt(root, controlRoot string, tree func() string, calls *LaneCallSet, boundary func(goal.Endpoint) goal.Endpoint, home func() (string, error)) batch.ReturnSeams {
	return batch.ReturnSeams{
		Read: func(_ string, tree, goalID string) (batch.ReturnLedgerGoal, error) {
			return BatchReturnLedgerGoal(controlRoot, tree, goalID)
		},
		Target: func(unit batch.Unit) batch.ReturnTarget {
			return ProductionReturnTarget(controlRoot, tree(), identity.KernelProber{}, unit)
		},
		HandBack: func(goalID string, source batch.Claim, epoch uint64) error {
			record, err := BatchReturnLedgerGoal(controlRoot, tree(), goalID)
			if err != nil {
				return err
			}
			loaded, err := findBatchUnit(root, record.Batch, goalID)
			if err != nil {
				return err
			}
			invocation, err := claimAuthority(controlRoot, goalID, record, calls, home, boundary)
			if err != nil {
				return err
			}
			return calls.Handover(invocation, ownercall.HandoverRequest{Root: controlRoot, GoalID: goalID,
				TargetMachine: source.Machine, TargetLineage: source.Lineage, TargetEpoch: int64(epoch),
				Batch: record.Batch, TargetRoot: loaded.SeatRoot})
		},
		Release: func(goalID, next string) error {
			record, err := BatchReturnLedgerGoal(controlRoot, tree(), goalID)
			if err != nil {
				return err
			}
			invocation, err := claimAuthority(controlRoot, goalID, record, calls, home, boundary)
			if err != nil {
				return err
			}
			if err := calls.EditNext(invocation, controlRoot, goalID, next); err != nil {
				return err
			}
			return calls.Release(invocation, controlRoot, goalID)
		},
	}
}

func findBatchUnit(root, batchID, goalID string) (batch.Unit, error) {
	record, err := batch.NewStore(root, nil).Load(batchID)
	if err != nil {
		return batch.Unit{}, err
	}
	for _, unit := range record.Units {
		if unit.GoalID == goalID && unit.State == batch.UnitReturnPending {
			return unit, nil
		}
	}
	return batch.Unit{}, fmt.Errorf("batch unit %s is absent", goalID)
}
