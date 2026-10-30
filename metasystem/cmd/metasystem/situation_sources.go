package main

import (
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/batchowner"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/plain"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/report"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/steward"
)

// situationSources fills the situation's reads the steward cannot reach:
// the clock, the host's launch store as the launch verbs count it, the lane
// status over that same observation, the claim's lane reader and the
// durable waiting lines of the checkout's installation. A source the caller gave is kept; the lane home,
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
