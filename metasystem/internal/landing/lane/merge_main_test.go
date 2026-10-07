package lane

import (
	"errors"
	"slices"
	"strings"
	"testing"
)

func TestKeeperRechecksSelectionAndDrainBeforeLaunch(t *testing.T) {
	t.Parallel()
	for _, boundary := range []string{"selection", "drain"} {
		t.Run(boundary, func(t *testing.T) {
			t.Parallel()
			home, _, module := nestedLaneDirs(t)
			clock := laneNow
			agent := &fakeAgent{}
			scope := "selected-batch"
			draining := false
			keeper := agent.keeper(home, module, &clock, WakeSources{Reasons: func(string) ([]string, error) {
				if boundary == "selection" {
					scope = "another-batch"
				} else {
					draining = true
				}
				return []string{queuedReason}, nil
			}})
			keeper.Continuation = func(Record) string { return scope }
			keeper.AdmitWake = func(_ string, wake Wake) (Wake, error) {
				if draining {
					wake.Reasons = nil
				}
				return wake, nil
			}
			run := keeper.Run()
			want := AgentHeld
			if boundary == "drain" {
				want = AgentIdle
			}
			if run.Outcome != want || len(agent.starts) != 0 {
				t.Fatalf("%s changed during wake: %+v starts=%v", boundary, run, agent.starts)
			}
			state, err := ReadAgentState(home)
			if err != nil || state.StartingAt != "" || state.Launch != "" {
				t.Fatalf("refusal claimed a launch: %+v %v", state, err)
			}
		})
	}
}

func TestKeeperAdvisoryProgressPreservesSelection(t *testing.T) {
	t.Parallel()
	home, _, module := nestedLaneDirs(t)
	clock := laneNow
	agent := &fakeAgent{}
	reasons := []string{queuedReason}
	keeper := agent.keeper(home, module, &clock, wakeFor(&reasons))
	keeper.Observe = func(Record) error { return &ObservationError{Progress: errors.New("drain progress cannot be written")} }
	prepared, admitted := false, false
	keeper.Prepare = func(Record) error { prepared = true; return nil }
	keeper.Continuation = func(Record) string { return "selected-batch" }
	keeper.AdmitWake = func(_ string, wake Wake) (Wake, error) {
		admitted = true
		if !prepared || wake.BatchID != "selected-batch" {
			t.Errorf("launch lost preparation or selection: %+v", wake)
		}
		return wake, nil
	}
	run := keeper.Run()
	if run.Outcome != AgentStarted || !prepared || !admitted || len(agent.starts) != 1 || agent.starts[0].BatchID != "selected-batch" || !slices.Contains(run.Problems, "drain progress cannot be written") || !strings.Contains(run.Line, "drain progress cannot be written") {
		t.Fatalf("advisory progress blocked selection or lost its warning: %+v starts=%v", run, agent.starts)
	}
}

func TestKeeperExplicitRunNeedsPersonSelectionToBypassQuestionFailure(t *testing.T) {
	t.Parallel()
	home, _, module := nestedLaneDirs(t)
	clock := laneNow
	agent := &fakeAgent{}
	reasons := []string{queuedReason}
	keeper := agent.keeper(home, module, &clock, wakeFor(&reasons))
	keeper.Explicit = true
	keeper.Observe = func(Record) error { return &ObservationError{StopQuestion: errors.New("question cannot be read")} }
	if run := keeper.Run(); run.Outcome != AgentHeld || len(agent.starts) != 0 || !slices.Contains(run.Problems, "question cannot be read") {
		t.Fatalf("explicit flag supplied person authority: %+v starts=%v", run, agent.starts)
	}
	keeper.PersonSelection = func(Record) bool { return true }
	if run := keeper.Run(); run.Outcome != AgentStarted || len(agent.starts) != 1 || !slices.Contains(run.Problems, "question cannot be read") {
		t.Fatalf("recorded person's choice was blocked or lost warning: %+v starts=%v", run, agent.starts)
	}
}
