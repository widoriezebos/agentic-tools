package main

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/lane"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/plain"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/launch"
)

func recoveredTickBed(t *testing.T) (pendingLandingBed, lane.AgentKeeper, *plain.StewardEffects, plain.ProveSeams, time.Time) {
	t.Helper()
	b := newPendingLandingBed(t)
	b.interrupt(t)
	reservation, err := b.manager.Store.Read(firstReservationID)
	if err != nil {
		t.Fatal(err)
	}
	original, err := time.Parse(time.RFC3339, reservation.StartedAt)
	if err != nil {
		t.Fatal(err)
	}
	*b.clock = original.Add(26 * time.Minute)
	state, err := lane.ReadAgentState(b.home)
	if err != nil || state.StartingAt == "" {
		t.Fatalf("outstanding claim: %+v %v", state, err)
	}
	if _, starting, err := lane.AgentStarting(b.home, *b.clock); err != nil || starting {
		t.Fatalf("claim has not expired: %t %v", starting, err)
	}
	b.manager.Supervisor = recordingSupervisor{b.manager.Store}
	a := newTestLandingAgent(func(a *landingAgent) {
		a.manager = func() *launch.Manager { return b.manager }
		a.now = b.manager.Now
	})
	k := newLandingAgentKeeper(b.root, b.home, a)
	// Keep production recovery and probe wiring; isolate unrelated admission reads.
	k.Refresh, k.Observe, k.Prepare, k.Clock = nil, nil, nil, nil
	k.Holds, k.ProviderHold, k.Helmed, k.AdmitWake = nil, nil, nil, nil
	k.Fingerprint, k.BarrenStop, k.Waiting = nil, nil, nil
	k.Continuation, k.Sources = b.keeper.Continuation, b.keeper.Sources
	effects := landingAgentStewardEffects(b.home, a, k)
	effects.UsageStore = &b.manager.Store
	registration, _, err := lane.Read(b.home)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(registration.Install, "metasystem.conf"), []byte("metasystem.template=true\n"), 0600); err != nil {
		t.Fatal(err)
	}
	seams := plain.ProveSeams{
		Now:       b.manager.Now,
		Machine:   func(string) (string, error) { return "fixture", nil },
		Git:       func(string, ...string) (string, error) { return "main", nil },
		Incidents: func(string, string, string) ([]goal.TrunkRedEntry, error) { return nil, nil },
		Alive:     func(plain.Running) bool { return false },
		Steward:   effects,
	}
	return b, k, effects, seams, original
}

func TestRecoveredReservationSurvivesItsOwnTick(t *testing.T) {
	t.Parallel()
	for _, retained := range []bool{false, true} {
		t.Run(map[bool]string{false: "no-step", true: "dead-retained-step"}[retained], func(t *testing.T) {
			t.Parallel()
			b, k, effects, seams, original := recoveredTickBed(t)
			registration, _, _ := lane.Read(b.home)
			if retained {
				writeQuestionFixture(t, filepath.Join(plain.Dir(registration.Install), "agent-step.json"), plain.AgentStep{Verb: "prove", StartedAt: original.Add(-time.Hour).Format(time.RFC3339), PID: 4242})
			}
			var calls []string
			effects.Settle = func(lane.Record) (string, error) {
				calls = append(calls, "settle")
				_, err := b.manager.Store.Update(firstReservationID, func(r *launch.Record) error { r.State = launch.Cancelled; return nil })
				return "ended", err
			}
			effects.Wake = func(lane.Record) (string, error) { calls = append(calls, "wake"); return "keeper woken", nil }
			steps := 0
			tick := landingStewardTick(k.Run, func() (plain.StewardRun, error) {
				steps++
				return plain.StewardStep(b.home, *b.clock, seams)
			})
			recoveredAt := *b.clock
			run := tick()
			probe, err := effects.Owner(b.root)
			state, stateErr := lane.ReadAgentState(b.home)
			if err != nil || stateErr != nil || !probe.Alive || !probe.Since.Equal(recoveredAt) || state.Launch != firstReservationID || state.StartedAt != recoveredAt.Format(time.RFC3339) {
				t.Fatalf("recovery probe=%+v state=%+v errors=%v %v", probe, state, err, stateErr)
			}
			record, err := b.manager.Store.Read(firstReservationID)
			if err != nil || record.State != launch.Running || run.Outcome != lane.AgentStarted || run.Launch != firstReservationID || len(run.Problems) != 0 || steps != 1 || len(calls) != 0 || !strings.Contains(run.Line, "; steward: ") {
				t.Fatalf("recovery tick=%+v record=%+v steps=%d calls=%v err=%v", run, record, steps, calls, err)
			}
			*b.clock = recoveredAt.Add(21 * time.Minute)
			run = tick()
			probe, err = effects.Owner(b.root)
			if err != nil || probe.Alive || steps != 2 || len(run.Problems) != 0 || !reflect.DeepEqual(calls, []string{"settle", "wake"}) || !strings.Contains(run.Line, "silent-agent") {
				t.Fatalf("later tick=%+v probe=%+v steps=%d calls=%v err=%v", run, probe, steps, calls, err)
			}
		})
	}
}

func TestLandingStewardOldHeldReservationKeepsSilenceAge(t *testing.T) {
	t.Parallel()
	b, k, effects, seams, original := recoveredTickBed(t)
	if _, err := b.manager.Store.Update(firstReservationID, func(r *launch.Record) error { r.Reason = "cancel-requested"; return nil }); err != nil {
		t.Fatal(err)
	}
	var calls []string
	effects.Settle = func(lane.Record) (string, error) { calls = append(calls, "settle"); return "ended", nil }
	effects.Wake = func(lane.Record) (string, error) { calls = append(calls, "wake"); return "keeper woken", nil }
	run := landingStewardTick(k.Run, func() (plain.StewardRun, error) { return plain.StewardStep(b.home, *b.clock, seams) })()
	probe, err := effects.Owner(b.root)
	if err != nil || !probe.Since.Equal(original) || run.Outcome != lane.AgentHeld || len(run.Problems) != 0 || !reflect.DeepEqual(calls, []string{"settle", "wake"}) {
		t.Fatalf("tick=%+v probe=%+v calls=%v err=%v", run, probe, calls, err)
	}
}

func TestLandingAgentProbeSelectsMatchingLaterKeeperStart(t *testing.T) {
	t.Parallel()
	for _, scenario := range []string{"absent", "different-launch", "older", "malformed", "later"} {
		t.Run(scenario, func(t *testing.T) {
			t.Parallel()
			b, _, effects, _, original := recoveredTickBed(t)
			state := lane.AgentState{Launch: firstReservationID, StartedAt: original.Add(time.Minute).Format(time.RFC3339)}
			want := original
			switch scenario {
			case "absent":
				if err := os.Remove(lane.AgentStatePath(b.home)); err != nil {
					t.Fatal(err)
				}
			case "different-launch":
				state.Launch = "another-reservation"
			case "older":
				state.StartedAt = original.Add(-time.Minute).Format(time.RFC3339)
			case "malformed":
				state.StartedAt = "invalid-time"
			case "later":
				want = original.Add(time.Minute)
			}
			if scenario != "absent" {
				writeQuestionFixture(t, lane.AgentStatePath(b.home), state)
			}
			probe, err := effects.Owner(b.root)
			if err != nil || !probe.Alive || !probe.Since.Equal(want) {
				t.Fatalf("probe=%+v want=%s err=%v", probe, want, err)
			}
		})
	}
}
