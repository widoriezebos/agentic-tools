package stoptransition

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/missionrunner"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/stopfence"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/supervise"
)

// These tests port the report lines of the mission-stop scenario in
// scripts/agents/mission-fixtures.sh: what `metasystem system stop` says
// about a cooperating runner, a TERM-ignoring host its runner killed, and
// the ownerless watcher and reaper the mission bed armed.

func TestMissionBedStopLinesNameWhoStoppedWhat(t *testing.T) {
	t.Parallel()
	runner := missionrunner.Item{Kind: missionrunner.ItemRunner, MissionID: "ignores-term", Pid: 41, Pgid: 41, Tag: "runner-tag"}
	turn := missionrunner.Item{Kind: missionrunner.ItemTurn, MissionID: "ignores-term", TurnID: "ignores-term-t1", Pid: 42, Pgid: 42}
	for _, test := range []struct {
		name     string
		item     missionrunner.Item
		outcome  missionrunner.StopOutcome
		line     string
		complete bool
	}{
		{name: "cooperating runner", item: runner,
			outcome:  missionrunner.StopOutcome{Result: "stopped", Signal: missionrunner.TerminationTerm, Reason: "runner-concluded"},
			line:     "mission ignores-term runner pid 41 pgid 41 tag runner-tag: stopped (TERM, runner concluded)",
			complete: true},
		{name: "runner that ignored TERM", item: runner,
			outcome:  missionrunner.StopOutcome{Result: "stopped", Signal: missionrunner.TerminationKill, Reason: "TERM ignored"},
			line:     "mission ignores-term runner pid 41 pgid 41 tag runner-tag: killed (TERM ignored; turn and lease closed by stop)",
			complete: true},
		{name: "host the runner killed", item: turn,
			outcome:  missionrunner.StopOutcome{Result: "stopped", Signal: missionrunner.TerminationKill, ByRunner: true},
			line:     "mission ignores-term turn ignores-term-t1 host pid 42 pgid 42: killed (by the runner, TERM ignored)",
			complete: true},
		{name: "host the runner stopped", item: turn,
			outcome:  missionrunner.StopOutcome{Result: "stopped", Signal: missionrunner.TerminationTerm, ByRunner: true},
			line:     "mission ignores-term turn ignores-term-t1 host pid 42 pgid 42: stopped (by the runner, TERM)",
			complete: true},
		{name: "orphan host", item: turn,
			outcome:  missionrunner.StopOutcome{Result: "stopped", Signal: missionrunner.TerminationTerm},
			line:     "mission ignores-term turn ignores-term-t1 host pid 42 pgid 42: stopped (TERM)",
			complete: true},
		{name: "surviving host", item: turn,
			outcome: missionrunner.StopOutcome{Result: "not-stopped", Signal: missionrunner.TerminationKill, Reason: "host group death is unproven"},
			line:    "NOT STOPPED mission ignores-term turn ignores-term-t1 host pid 42 pgid 42: host group death is unproven; did: left the turn record"},
	} {
		t.Run(test.name, func(t *testing.T) {
			line, complete := missionOutcomeLine(test.item, test.outcome)
			if line != test.line || complete != test.complete {
				t.Fatalf("line = %q complete=%v, want %q complete=%v", line, complete, test.line, test.complete)
			}
		})
	}
}

// ignores-term through the family: after the runner concluded, the host
// turn it killed is reported as the runner's kill, and the family sends
// nothing to a host whose conclusion is already recorded.
func TestMissionBedFamilyReportsTheRunnersKillOfItsHost(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	recordPath := filepath.Join(root, "artifacts", "agents", "missions", "ignores-term", "turns", "ignores-term-t1", "turn.json")
	if err := os.MkdirAll(filepath.Dir(recordPath), 0o755); err != nil {
		t.Fatal(err)
	}
	record, err := json.Marshal(map[string]any{
		"missionId": "ignores-term", "turnId": "ignores-term-t1", "runtime": "fake",
		"status": "failed", "outcome": "failed", "error": "turn-lost", "hostTermination": missionrunner.TerminationKill,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(recordPath, record, 0o644); err != nil {
		t.Fatal(err)
	}
	family := newMissionFamily(root)
	key := "mission:ignores-term:" + missionrunner.ItemTurn + ":ignores-term-t1"
	family.items[key] = missionrunner.Item{Kind: missionrunner.ItemTurn, Root: root, MissionID: "ignores-term", TurnID: "ignores-term-t1",
		Runtime: "fake", RecordPath: recordPath, Pid: 999950, PidStartedAt: 1, Pgid: 999950, Tag: "fixture-host", Liveness: identity.Alive.String()}
	family.runnerConcluded["ignores-term"] = true
	outcome, err := family.Stop(Item{Key: key, Survivor: stopfence.Survivor{Component: "mission-turn", ID: "ignores-term/ignores-term-t1"}})
	want := "mission ignores-term turn ignores-term-t1 host pid 999950 pgid 999950: killed (by the runner, TERM ignored)"
	if err != nil || !outcome.Complete || outcome.Line != want {
		t.Fatalf("family outcome = %+v, %v; want %q", outcome, err, want)
	}
}

// cooperating-host supervision: the stop reports the bed's ownerless
// watcher and reaper by name as stopped with TERM.
func TestMissionBedStopReportsOwnerlessWatcherAndReaper(t *testing.T) {
	t.Parallel()
	family := newSupervisionFamily(LocalConfig{Root: "/state", Installation: "/engine", Checkout: "/checkout"}, &processSnapshot{})
	components := []supervise.InventoryItem{
		{Component: "repo-watcher", Identity: identity.Ref{Pid: 71, StartedAtSec: 200}, Tag: "mission-watcher-tag"},
		{Component: "job-reaper", Identity: identity.Ref{Pid: 72, StartedAtSec: 201}, Tag: "mission-reaper-tag"},
	}
	keys := make([]string, len(components))
	for index, component := range components {
		keys[index] = "supervision:" + supervisionItemKey(component)
		family.items[keys[index]] = component
	}
	shutdowns := 0
	family.shutdown = func(string, string, string, string, int) (supervise.ShutdownReport, error) {
		shutdowns++
		report := supervise.ShutdownReport{}
		for _, component := range components {
			report.Outcomes = append(report.Outcomes, supervise.ComponentOutcome{
				Component: component.Component, Identity: component.Identity, Tag: component.Tag,
				Signal: supervise.ShutdownSignalTerm, Result: supervise.ShutdownStopped,
			})
		}
		return report, nil
	}
	for index, want := range []string{"repo-watcher pid 71: stopped (TERM)", "job-reaper pid 72: stopped (TERM)"} {
		component := components[index]
		outcome, err := family.Stop(Item{Key: keys[index], StatusLine: want[:len(want)-len(": stopped (TERM)")] + ": running",
			Survivor: stopfence.Survivor{Component: component.Component, Pid: component.Identity.Pid, PidStartedAt: component.Identity.StartedAtSec}})
		if err != nil || !outcome.Complete || outcome.Line != want {
			t.Fatalf("ownerless %s outcome = %+v, %v; want %q", component.Component, outcome, err, want)
		}
	}
	if shutdowns != 1 {
		t.Fatalf("supervision shutdowns = %d, want one for the whole set", shutdowns)
	}
}
