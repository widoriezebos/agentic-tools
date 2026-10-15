package main

import (
	"encoding/json"
	"errors"
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

func TestLandingStewardKeeperTicksEveryKeptOutcome(t *testing.T) {
	t.Parallel()
	for _, outcome := range []lane.AgentOutcome{lane.AgentIdle, lane.AgentHeld, lane.AgentPaused, lane.AgentRunning, lane.AgentStarted, lane.AgentFailed, lane.AgentNotKept} {
		t.Run(string(outcome), func(t *testing.T) {
			t.Parallel()
			calls := 0
			launch := ""
			if outcome == lane.AgentStarted || outcome == lane.AgentRunning {
				launch = "agent"
			}
			tick := landingStewardTick(func() lane.AgentRun {
				return lane.AgentRun{Outcome: outcome, Launch: launch, Line: "keeper", Problems: []string{"claim error"}}
			}, func() (plain.StewardRun, error) {
				calls++
				return plain.StewardRun{Line: "noticed"}, errors.New("steward error")
			})
			run := tick()
			wantCalls, wantLine := 1, "keeper; steward: noticed; steward failed: steward error"
			wantProblems := []string{"claim error", "steward error"}
			if outcome == lane.AgentNotKept {
				wantCalls, wantLine, wantProblems = 0, "keeper", []string{"claim error"}
			}
			if calls != wantCalls || run.Line != wantLine || !reflect.DeepEqual(run.Problems, wantProblems) {
				t.Fatalf("outcome=%s calls=%d line=%s", outcome, calls, run.Line)
			}
		})
	}
	for _, outcome := range []lane.AgentOutcome{lane.AgentPaused, lane.AgentHeld} {
		t.Run("cancelled-"+string(outcome), func(t *testing.T) {
			t.Parallel()
			calls := 0
			run := landingStewardTick(func() lane.AgentRun { return lane.AgentRun{Outcome: outcome, Launch: "just-launched", Line: "keeper"} }, func() (plain.StewardRun, error) {
				calls++
				return plain.StewardRun{Line: "noticed"}, nil
			})()
			if calls != 1 || run.Launch != "just-launched" || run.Line != "keeper; steward: noticed" {
				t.Fatalf("calls=%d run=%+v", calls, run)
			}
		})
	}
}

func TestLandingStewardHeldTickHealsSilentAgent(t *testing.T) {
	t.Parallel()
	for _, scenario := range []string{"silent", "person-stop", "person-proof"} {
		t.Run(scenario, func(t *testing.T) {
			t.Parallel()
			b := newPendingLandingBed(t)
			b.interrupt(t)
			if _, err := b.manager.Store.Update(firstReservationID, func(r *launch.Record) error { r.Reason = "cancel-requested"; return nil }); err != nil {
				t.Fatal(err)
			}
			now := *b.clock
			var calls []string
			seams := plain.ProveSeams{
				Now:       b.manager.Now,
				Machine:   func(string) (string, error) { return "fixture", nil },
				Git:       func(string, ...string) (string, error) { return "main", nil },
				Incidents: func(string, string, string) ([]goal.TrunkRedEntry, error) { return nil, nil },
				Alive:     func(plain.Running) bool { return true },
				Steward: &plain.StewardEffects{
					UsageStore: &b.manager.Store,
					Owner: func(string) (lane.OwnerProbe, error) {
						return lane.OwnerProbe{Alive: true, Since: now.Add(-21 * time.Minute)}, nil
					},
					Settle: func(lane.Record) (string, error) { calls = append(calls, "settle"); return "ended", nil },
					Wake:   func(lane.Record) (string, error) { calls = append(calls, "wake"); return "keeper woken", nil },
				},
			}
			record, _, err := lane.Read(b.home)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(record.Install, "metasystem.conf"), []byte("metasystem.template=true\n"), 0600); err != nil {
				t.Fatal(err)
			}
			if scenario == "person-stop" {
				if _, err := lane.SetPause(b.home, "fixture-person", now); err != nil {
					t.Fatal(err)
				}
			}
			if scenario == "person-proof" {
				writeQuestionFixture(t, filepath.Join(plain.Dir(record.Install), "running.json"), plain.Running{Attempt: "person", Person: &plain.ActProvenance{Person: "fixture-person"}, Since: now.Format(time.RFC3339), Pid: 42})
			}
			run := landingStewardTick(b.keeper.Run, func() (plain.StewardRun, error) { return plain.StewardStep(b.home, now, seams) })()
			if run.Outcome != lane.AgentHeld || len(run.Problems) != 0 || !strings.Contains(run.Line, "; steward: ") {
				t.Fatalf("tick: %+v", run)
			}
			data, err := os.ReadFile(filepath.Join(plain.Dir(record.Install), "interventions.jsonl"))
			if scenario != "silent" {
				if len(calls) != 0 || err != nil && !os.IsNotExist(err) || len(data) != 0 {
					t.Fatalf("person interrupted: calls=%v data=%s err=%v", calls, data, err)
				}
				return
			}
			var entry plain.Intervention
			if err != nil {
				t.Fatal(err)
			}
			if err := json.Unmarshal(data, &entry); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(calls, []string{"settle", "wake"}) || entry.Row != "silent-agent" || entry.Outcome != "keeper woken" || entry.How != "settle the landing agent; keeper wake" {
				t.Fatalf("calls=%v intervention=%+v", calls, entry)
			}
		})
	}
}

func TestLandingStewardKeeperReportsFailure(t *testing.T) {
	t.Parallel()
	run := landingStewardTick(func() lane.AgentRun { return lane.AgentRun{Outcome: lane.AgentHeld, Line: "barren"} }, func() (plain.StewardRun, error) { return plain.StewardRun{}, errors.New("unreadable intervention log") })()
	if len(run.Problems) != 1 || !strings.Contains(run.Line, "unreadable intervention log") {
		t.Fatal(run)
	}
}
