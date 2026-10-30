package main

import (
	"errors"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/batchowner"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/plain"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/report"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/steward"
)

// situationSources fills the situation's reads the steward cannot reach:
// the clock, the host's launch store as the launch verbs count it, the lane
// status over that same observation, the claim's lane reader, the units over
// the read's projection and the durable waiting lines of the checkout's
// installation. A source the caller gave is kept; the lane and board homes,
// the ledger and the machine take the steward's own readers.
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
