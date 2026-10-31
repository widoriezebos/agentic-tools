package main

import (
	"errors"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/batchowner"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/plain"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/launch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/lease"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/report"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/steward"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/textui"
)

// situationSources fills the situation's reads the steward cannot reach:
// the clock, the host's launch store as the launch verbs count it, the lane
// status over that same observation, the claim's lane reader, the units over
// the read's projection, the durable waiting lines of the checkout's
// installation, the owners' launches as launch list sees them and each
// seat's lease holder, and goal next's wording of an owned red. A source
// the caller gave is kept; the lane and board homes, the ledger, the
// machine and the host take the steward's own readers.
func situationSources(owners intentOwners, src steward.SituationSources) steward.SituationSources {
	if src.Now == nil {
		src.Now = time.Now
	}
	if src.HostWork == nil {
		src.HostWork = func() (int, int, error) { return launchManager().HostWork() }
	}
	if src.LaneStatus == nil {
		hostWork := src.HostWork
		src.LaneStatus = func(home string, now time.Time) plain.Status {
			return batchowner.LandingLaneStatus(func() (string, error) { return home, nil }, now, hostWork)
		}
	}
	if src.ClaimLaneState == nil {
		src.ClaimLaneState = stewardClaimLaneState(owners)
	}
	if src.Units == nil {
		src.Units = situationUnitStages(owners)
	}
	if src.Launches == nil {
		manager := launchManager
		if owners.processes.launches != nil {
			manager = owners.processes.launches
		}
		src.Launches = func() ([]launch.Record, error) { return manager().EffectiveList() }
	}
	if src.Holder == nil {
		src.Holder = situationHolder
	}
	if src.RedLine == nil {
		src.RedLine = func(entry goal.TrunkRedEntry, now time.Time) string {
			return trunkRedOwnedLine(textui.Env{Now: now, Zone: now.Location()}, entry)
		}
	}
	if src.Waiting == nil {
		// Waiting rows are run state, kept under the checkout's installation.
		src.Waiting = func(root string) ([]string, error) {
			layout, err := owners.resolver.ResolveLayout(root)
			if err != nil {
				return nil, err
			}
			return report.CurrentWaitingLines(layout.InstallationRoot.Path())
		}
	}
	return src
}

// situationUnitStages is the seat's unit reader over the situation's one
// projection and clock: the invocation projects nothing of its own.
func situationUnitStages(owners intentOwners) func(string, string, goal.Projection, time.Time) ([]steward.UnitStage, error) {
	return func(root, id string, projection goal.Projection, now time.Time) ([]steward.UnitStage, error) {
		inv := &intentInvocation{cwd: root, owners: owners, givenProjection: &projection, givenNow: now}
		if problem := inv.selectRoot(); problem != nil {
			return nil, errors.New(problem.Summary)
		}
		_, _, _, _, units, problem := inv.goalUnitStages(id)
		if problem != nil {
			return nil, errors.New(problem.Summary)
		}
		return units, nil
	}
}

// situationHolder is an armed checkout's lease holder; no lease is no holder.
func situationHolder(checkout string) (steward.SeatHolder, error) {
	view, err := lease.CurrentHolder(checkout)
	if errors.Is(err, lease.ErrLeaseAbsent) {
		return steward.SeatHolder{}, nil
	}
	return steward.SeatHolder{MainId: view.MainId, SessionId: view.SessionId, Lineage: view.OwnerLineage, Pid: view.Pid}, err
}
