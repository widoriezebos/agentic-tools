package main

import (
	"errors"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/lane"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/plain"
)

func TestLandingStewardKeeperTicksOnlyWithoutLaunch(t *testing.T) {
	t.Parallel()
	for _, outcome := range []lane.AgentOutcome{lane.AgentIdle, lane.AgentHeld, lane.AgentPaused, lane.AgentRunning, lane.AgentStarted, lane.AgentFailed, lane.AgentNotKept} {
		t.Run(string(outcome), func(t *testing.T) {
			t.Parallel()
			calls := 0
			launch := ""
			if outcome == lane.AgentStarted || outcome == lane.AgentRunning {
				launch = "agent"
			}
			tick := landingStewardTick(func() lane.AgentRun { return lane.AgentRun{Outcome: outcome, Launch: launch, Line: "keeper"} }, func() (plain.StewardRun, error) { calls++; return plain.StewardRun{Line: "noticed"}, nil })
			run := tick()
			want := outcome != lane.AgentStarted && outcome != lane.AgentFailed && outcome != lane.AgentNotKept
			if (calls == 1) != want || strings.Contains(run.Line, "steward: noticed") != want {
				t.Fatalf("outcome=%s calls=%d line=%s", outcome, calls, run.Line)
			}
		})
	}
	for _, outcome := range []lane.AgentOutcome{lane.AgentPaused, lane.AgentHeld} {
		run := landingStewardTick(func() lane.AgentRun { return lane.AgentRun{Outcome: outcome, Launch: "just-launched"} }, func() (plain.StewardRun, error) {
			t.Fatal("steward ran after a cancelled launch")
			return plain.StewardRun{}, nil
		})()
		if run.Launch != "just-launched" {
			t.Fatal(run)
		}
	}
}

func TestLandingStewardKeeperReportsFailure(t *testing.T) {
	t.Parallel()
	run := landingStewardTick(func() lane.AgentRun { return lane.AgentRun{Outcome: lane.AgentHeld, Line: "barren"} }, func() (plain.StewardRun, error) { return plain.StewardRun{}, errors.New("unreadable intervention log") })()
	if len(run.Problems) != 1 || !strings.Contains(run.Line, "unreadable intervention log") {
		t.Fatal(run)
	}
}
