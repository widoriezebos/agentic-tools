package main

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/batch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/lane"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/realpath"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/seat"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/stateroot"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/verbresult"
)

// The group's goldens join layoutCases through its hook.
var _ = func() bool {
	layoutGroupCases = append(layoutGroupCases, g1bLayoutCases)
	return true
}()

// G1b's layout goldens (output-style §7): machine list and stop, and the
// landing verbs. An act's golden is a case whose repeat prints the same
// page (the audit runs each bed twice, plain and in colour): an act that
// already holds, a refusal, or a restart whose fresh owner keeps its pid.
func g1bLayoutCases() []layoutCase {
	return []layoutCase{
		{name: "machine-list", args: []string{"machine", "list"}, bed: machineLayoutBed},
		{name: "machine-list-verbose", args: []string{"machine", "list", "--verbose"}, bed: machineLayoutBed},
		{name: "machine-stop", args: []string{"machine", "stop", "m1x"}, bed: machineLayoutBed},
		{name: "machine-stop-refusal", args: []string{"machine", "stop", "m9z"}, bed: machineLayoutBed},
		{name: "machine-start", args: []string{"machine", "start", "m1f"}, bed: machineStartLayoutBed(false)},
		{name: "machine-start-repeat", args: []string{"machine", "start", "m1f"}, bed: machineStartLayoutBed(true)},
		{name: "landing-status", args: []string{"landing", "status"}, bed: landingLayoutBed(landingLayoutRunning)},
		{name: "landing-status-verbose", args: []string{"landing", "status", "--verbose"}, bed: landingLayoutBed(landingLayoutRunning)},
		{name: "landing-status-stopped", args: []string{"landing", "status"}, bed: landingLayoutBed(landingLayoutPaused)},
		{name: "landing-status-none", args: []string{"landing", "status"}, bed: landingLayoutBed(landingLayoutNone)},
		{name: "landing-set", args: []string{"landing", "set", "../agentic-tools-landing"}, bed: landingLayoutBed(landingLayoutRunning)},
		{name: "landing-start", args: []string{"landing", "start"}, bed: landingLayoutBed(landingLayoutRunning)},
		{name: "landing-start-refusal", args: []string{"landing", "start"}, bed: landingLayoutBed(landingLayoutNone)},
		{name: "landing-run", args: []string{"landing", "run"}, bed: inTheLaneCheckout(landingLayoutBed(landingLayoutRunning))},
		{name: "landing-run-refusal", args: []string{"landing", "run"}, bed: inTheLaneCheckout(landingLayoutBed(landingLayoutPaused))},
		{name: "landing-run-no-engine", args: []string{"landing", "run"}, bed: landingLayoutBed(landingLayoutRunning)},
		{name: "landing-stop-refusal", args: []string{"landing", "stop"}, bed: landingLayoutBed(landingLayoutPushing)},
		{name: "landing-unset", args: []string{"landing", "unset"}, bed: landingLayoutBed(landingLayoutNone)},
	}
}

// machineLayoutBed is the machine verbs' bed (newMachineBed): this checkout
// m1e running its helpers, a job and a launch, the stopped m1x, the landing
// lane's checkout running its owner, and m2a on another computer.
func machineLayoutBed(t *testing.T) layoutBed {
	b := newMachineBed(t)
	// Each presence names its engine and free space; m2a has gone silent.
	b.fleetEdit = func(report *seat.Report) {
		for index, engine := range []string{"330eb33af4c99d8f1e2a", "330eb33af4c99d8f1e2a", "4f4c5101e7d2a9b3c6f0"} {
			machine := &report.Machines[index]
			free := []int64{248_463_380_480, 248_463_380_480, 78_700_000_000}[index]
			machine.Record.Engine, machine.Record.DiskFreeBytes = engine, &free
		}
		report.Machines[0].Record.TickAt = machineBedNow.Add(-2 * time.Minute).Format(time.RFC3339)
		report.Machines[2].Standing, report.Machines[2].Reason = seat.Unreachable, "no presence for 3 h, past 30 min"
	}
	return layoutBed{owners: b.owners(), cwd: b.this, now: machineBedNow, replace: layoutPaths(b.this, b.this, "/Users/wido/GitHub/agentic-tools-m1e",
		b.other, "/Users/wido/GitHub/agentic-tools-m1x", b.landing, "/Users/wido/GitHub/agentic-tools-landing", b.home, "/Users/wido/.metasystem-home",
		b.launchDir, "/Users/wido/.metasystem-launches",
		"~/agentic-tools-landing", "~/GitHub/agentic-tools-landing", "~/agentic-tools-m1x", "~/GitHub/agentic-tools-m1x")}
}

// The landing beds: no lane; a lane whose owner runs, one batch proving
// and one collecting behind it; the same lane paused by Wido; and the same
// lane with its batch pushing to main. No golden holds a local time
// (lane.LocalText reads the host's zone), so they hold in every zone.
const (
	landingLayoutNone = iota
	landingLayoutRunning
	landingLayoutPaused
	landingLayoutPushing
)

func landingLayoutBed(kind int) func(t *testing.T) layoutBed {
	return func(t *testing.T) layoutBed {
		base := t.TempDir()
		cwd, home, landing := filepath.Join(base, "agentic-tools-m1e"), filepath.Join(base, "home"), filepath.Join(base, "agentic-tools-landing")
		for _, dir := range []string{cwd, home, landing} {
			if err := os.MkdirAll(dir, 0o755); err != nil {
				t.Fatal(err)
			}
		}
		cwd, home, landing = realpath.Resolve(cwd), realpath.Resolve(home), realpath.Resolve(landing)
		now := layoutNow
		alive := kind != landingLayoutNone
		records := []batch.Record{}
		if kind != landingLayoutNone {
			registerLane(t, home, landing, "Wido", now.Add(-2*time.Hour))
			records = []batch.Record{
				{Schema: 1, BatchID: "4gr18nm8t3nyev9sssda9jgtsq", State: map[bool]string{true: batch.StateLanding, false: batch.StateProving}[kind == landingLayoutPushing],
					Units: []batch.Unit{{GoalID: "verbs-match-intent", Chain: "c1", State: batch.UnitJoined, Claim: batch.Claim{Machine: "m1e"}},
						{GoalID: "switch-on-trial", Chain: "c2", State: batch.UnitJoined, Claim: batch.Claim{Machine: "ui"}}},
					History: []batch.HistoryEntry{{At: now.Add(-9 * time.Minute).Format(time.RFC3339Nano), Verb: "open", To: batch.StateOpen},
						{At: now.Add(-4 * time.Minute).Format(time.RFC3339Nano), Verb: "seal", From: batch.StateOpen, To: batch.StateSealed},
						{At: now.Add(-3 * time.Minute).Format(time.RFC3339Nano), Verb: "prove", From: batch.StateSealed, To: batch.StateProving}}},
				{Schema: 1, BatchID: "7kq2m9x4c1vbn8hzt5pwe3dyra", State: batch.StateOpen,
					Units:   []batch.Unit{{GoalID: "disk-lifetimes", Chain: "c3", State: batch.UnitJoined, Claim: batch.Claim{Machine: "m1e"}}},
					History: []batch.HistoryEntry{{At: now.Add(-time.Minute).Format(time.RFC3339Nano), Verb: "open", To: batch.StateOpen}}},
			}
		}
		if kind == landingLayoutPaused {
			if _, err := lane.SetPause(home, "Wido", now.Add(-30*time.Minute)); err != nil {
				t.Fatal(err)
			}
		}
		const pid = 38928
		notARepository := func(string) (string, error) { return "", errors.New("not a repository") }
		owners := intentOwners{resolver: stateroot.NewResolver(notARepository, os.Executable), landing: laneVerbOwners{
			home: func() (string, error) { return home, nil },
			probe: func(string) (lane.OwnerProbe, error) {
				if !alive {
					return lane.OwnerProbe{}, nil
				}
				return lane.OwnerProbe{Alive: true, PID: pid, Since: now.Add(-5 * time.Minute)}, nil
			},
			person:   func(string) (string, error) { return "Wido", nil },
			records:  func(string) ([]batch.Record, error) { return records, nil },
			validate: func(root, _ string, _ time.Time) (string, error) { return realpath.Resolve(root), nil },
			by:       func(string) string { return "Wido" },
			ready:    func(string) error { return nil },
			machine:  func(string) (string, error) { return "landing", nil },
			now:      func() time.Time { return now },
			// landing run's keeper finds the running agent; it never starts one.
			keeper: func(home, root string) lane.AgentKeeper {
				return lane.AgentKeeper{Home: home, Self: root, Now: func() time.Time { return now },
					Running: func() (string, bool, error) { return "landing-5f0c2a9e7d31b468", alive, nil },
					Start: func(string, lane.Wake) (string, error) {
						t.Error("a layout bed started a landing agent")
						return "", errors.New("no launches in a layout bed")
					}}
			},
		}}
		return layoutBed{owners: owners, cwd: cwd, now: now, replace: layoutPaths(cwd, cwd, "/Users/wido/GitHub/agentic-tools-m1e",
			lane.AccountID(landing), "lane:99af5acdbc67", home, "/Users/wido/.metasystem-home", landing, "/Users/wido/GitHub/agentic-tools-landing",
			"~/agentic-tools-landing", "~/GitHub/agentic-tools-landing")}
	}
}

// inTheLaneCheckout is a landing bed called from inside the lane checkout,
// where landing run takes its step itself.
func inTheLaneCheckout(bed func(t *testing.T) layoutBed) func(t *testing.T) layoutBed {
	return func(t *testing.T) layoutBed {
		layout := bed(t)
		layout.cwd = filepath.Join(filepath.Dir(layout.cwd), "agentic-tools-landing")
		return layout
	}
}

// machineStartLayoutBed is the machine bed with the seat launch owner
// answering machine start: a fresh launch of m1f, or (already) the launch
// that was supervised before, printed as the owner's --json record.
func machineStartLayoutBed(already bool) func(t *testing.T) layoutBed {
	return func(t *testing.T) layoutBed {
		bed := machineLayoutBed(t)
		record := map[string]any{"schemaVersion": 1, "launch": "01K5ZZZZZZZZZZZZZZZZZZZZZZ", "machine": "m1f",
			"destination": "/Users/wido/GitHub/agentic-tools-m1f", "startedAt": "2026-09-30T08:50:00Z", "outcome": "done"}
		if already {
			record["alreadyLaunched"] = true
		}
		encoded, err := json.MarshalIndent(record, "", "  ")
		if err != nil {
			t.Fatal(err)
		}
		bed.owners.delivery = &intentDeliveryOwners{executable: func() (string, error) { return "/fake/metasystem", nil },
			ownerEnvelope: func(_ intentProcess, verb string) (verbresult.Result, error) {
				return verbresult.Result{SchemaVersion: 1, Verb: verb, Outcome: verbresult.Confirmed, Data: encoded}, nil
			}}
		return bed
	}
}
