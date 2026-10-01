package main

import (
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/batchowner"
)

// The landing path (internal/landing/batchowner) calls back into the engine
// for what needs the engine's own configuration: the retained verification
// and cost forecast of a testing selection, a goal branch's claim check, and
// the goal ledger's owner functions; and it reads the landing agent from
// the launch store the work verbs use.
func init() {
	batchowner.Engine = batchowner.EngineCalls{
		VerifyRetainedTesting:    verifyRetainedTesting,
		ForecastTestingSelection: forecastTestingSelection,
		BranchClaimCheck:         goalBranchClaimCheck,
	}
	batchowner.LaneCalls = batchowner.LaneCallSet{Handover: goalHandoverOwner, EditNext: goalEditNextOwner, Release: goalReleaseOwner, Held: landingHeld}
	agent := newLandingAgent()
	batchowner.LandingAgentLive = agent.liveOrStarting(batchowner.LandingLaneHome)
	batchowner.LandingAgentProbe = agent.probe
}
