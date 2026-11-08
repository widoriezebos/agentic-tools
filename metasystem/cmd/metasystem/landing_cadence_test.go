package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/lane"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/plain"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/launch"
)

// cadenceBed drives the production keeper with main's recorded batch push and
// no queued work. Detached launches remain alive throughout the tick sequence.
type cadenceBed struct {
	*selectionBed
	keeper     lane.AgentKeeper
	argv       [][]string
	started    []time.Time
	recoveries int
}

func newCadenceBed(t *testing.T, interval string, fullAge time.Duration, debt, red, unproved bool) *cadenceBed {
	t.Helper()
	b := &cadenceBed{selectionBed: newSelectionBed(t)}
	conf := "proof.full=fixture-proof\nlanding.batch=auto\n"
	if interval != "" {
		conf += "proof.trunk-every=" + interval + "\n"
	}
	helmMust(t, os.WriteFile(filepath.Join(b.lane, "metasystem.conf"), []byte(conf), 0600))
	helmMust(t, os.WriteFile(filepath.Join(plain.Dir(b.lane), "queue.jsonl"), nil, 0600))
	fullAt := b.now.Add(-fullAge).Format(time.RFC3339)
	proof := plain.Result{Commit: "main", Tree: "main", Scope: "full", Result: plain.Green, At: fullAt, FullAt: fullAt, FullTree: "main", Environment: "fixture-toolchain", Attempt: "batch-full", BatchID: "landed"}
	if red {
		proof.Failed = []plain.FailedUnit{{Unit: "fixture"}}
		proof.Trunk = true
		proof.Result = plain.Red
		proof.Cause = &plain.Cause{Kind: "main"}
	}
	writeQuestionFixture(t, filepath.Join(plain.Dir(b.lane), "results.jsonl"), proof)
	writeQuestionFixture(t, filepath.Join(plain.Dir(b.lane), "pushes.jsonl"), plain.Pushed{Tree: "main", Commit: "main", At: fullAt, BatchID: "landed"})
	if debt {
		scoped := proof
		scoped.Scope = "scoped"
		scoped.Attempt = "scoped"
		scoped.At = b.now.Add(-time.Minute).Format(time.RFC3339)
		scoped.FullAt = b.now.Add(-2 * time.Hour).Format(time.RFC3339)
		scoped.FullTree = "older-tree"
		f, err := os.OpenFile(filepath.Join(plain.Dir(b.lane), "results.jsonl"), os.O_APPEND|os.O_WRONLY, 0600)
		helmMust(t, err)
		data, err := json.Marshal(scoped)
		helmMust(t, err)
		_, err = f.Write(append(append([]byte{'\n'}, data...), '\n'))
		helmMust(t, err)
		helmMust(t, f.Close())
		f, err = os.OpenFile(filepath.Join(plain.Dir(b.lane), "pushes.jsonl"), os.O_APPEND|os.O_WRONLY, 0600)
		helmMust(t, err)
		data, err = json.Marshal(plain.Pushed{Tree: "main", Commit: "main", At: scoped.At})
		helmMust(t, err)
		_, err = f.Write(append(append([]byte{'\n'}, data...), '\n'))
		helmMust(t, err)
		helmMust(t, f.Close())
	}
	if unproved {
		helmMust(t, os.WriteFile(filepath.Join(plain.Dir(b.lane), "results.jsonl"), nil, 0600))
		helmMust(t, os.WriteFile(filepath.Join(plain.Dir(b.lane), "pushes.jsonl"), nil, 0600))
	}
	seams := b.owners.landing.plainProve
	git := seams.Git
	seams.Git = func(dir string, args ...string) (string, error) {
		if unproved && args[0] == "rev-parse" {
			if strings.HasSuffix(args[len(args)-1], "^{tree}") {
				return "outside-tree", nil
			}
			return "outside-main", nil
		}
		if unproved && args[0] == "diff" {
			return "metasystem/internal/outside.go", nil
		}
		if args[0] == "show" && strings.HasSuffix(args[len(args)-1], ":metasystem.conf") {
			return "proof.full=fixture-proof\n", nil
		}
		return git(dir, args...)
	}
	seams.Policy = func(string) (plain.PolicyValue, error) {
		return plain.PolicyValue{Value: "auto", Source: "fixture"}, nil
	}
	seams.RecoveryStarted = func(r plain.Running, already bool) {
		if r.Reused != nil || !already && r.Attempt != "" {
			b.recoveries++
		}
	}
	alive := false
	seams.Alive = func(plain.Running) bool { return alive }
	seams.Incidents = func(string, string, string) ([]goal.TrunkRedEntry, error) {
		if red {
			return []goal.TrunkRedEntry{{ID: "main-red", Identity: "red:fixture", Opened: fullAt, Sightings: []goal.TrunkRedSighting{{BaseCommit: "main"}}}}, nil
		}
		return nil, nil
	}
	seams.Launch = func(argv []string, _, _ string) (int64, error) {
		if !slices.Contains(argv, "--trunk") {
			t.Fatalf("unexpected launch %v", argv)
		}
		b.argv = append(b.argv, slices.Clone(argv))
		b.started = append(b.started, b.now)
		alive = true
		return int64(os.Getpid()), nil
	}
	b.keeper = newLandingAgentKeeper(b.lane, b.home, newTestLandingAgent(func(agent *landingAgent) {
		agent.proofEffects = seams
		agent.now = func() time.Time { return b.now }
		agent.machine = func(string) (string, error) { return "fixture", nil }
		agent.settings = func(string) (launch.Settings, error) { return launch.DefaultSettings(), nil }
	}))
	b.keeper.Running = func() (string, bool, error) { return "", false, nil }
	b.keeper.Start = func(string, lane.Wake) (string, error) { t.Fatal("empty queue started an agent"); return "", nil }
	return b
}

func TestKeeperCadenceOffStartsNoSuite(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, interval string
		debt, red      bool
		age            time.Duration
		ticks, want    int
	}{
		{name: "off", ticks: 48}, {name: "daily", interval: "24h", ticks: 48, want: 1},
		{name: "debt off", debt: true, age: 2 * time.Hour, ticks: 48}, {name: "debt daily", interval: "24h", debt: true, age: time.Hour, ticks: 22},
		{name: "invalid", interval: "4h", ticks: 48}, {name: "known red", red: true, ticks: 48},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			b := newCadenceBed(t, tc.interval, tc.age, tc.debt, tc.red, false)
			origin := b.now
			if tc.name == "invalid" {
				var output bytes.Buffer
				code := runConfigValidate([]string{"--conf", filepath.Join(b.lane, "metasystem.conf"), "--repo", b.lane}, &output, &output)
				out := output.String()
				if code == 0 || !strings.Contains(out, "proof.trunk-every must be off or a duration of at least 24h") {
					t.Fatalf("validation: %d %s", code, out)
				}
			}
			var wakes [][]string
			for tick := 0; tick < tc.ticks; tick++ {
				b.now = origin.Add(time.Duration(tick) * time.Hour)
				run := b.keeper.Run()
				if run.Outcome == lane.AgentFailed || len(run.Problems) > 0 {
					t.Fatalf("tick %d: %+v", tick, run)
				}
				reasons, err := plain.WakeReasons(b.lane, b.lane, time.Time{}, b.now, b.owners.landing.plainProve)
				helmMust(t, err)
				wakes = append(wakes, reasons)
				if tc.interval != "24h" && (slices.Contains(reasons, plain.WakeFullDue) || run.Outcome == lane.AgentHeld) {
					t.Fatalf("disabled tick %d: %+v reasons=%v", tick, run, reasons)
				}
				if tc.interval == "24h" && tick < 24-int(tc.age/time.Hour) && len(b.argv) != 0 {
					t.Fatalf("early launch at tick %d: %v", tick, b.argv)
				}
			}
			if len(b.argv) != tc.want {
				t.Fatalf("launches=%v want=%d wakes=%v", b.argv, tc.want, wakes)
			}
			if tc.want == 1 && (!slices.Contains(b.argv[0], "--fresh") || !b.started[0].Equal(origin.Add(24*time.Hour))) {
				t.Fatalf("daily start %v %v", b.started, b.argv)
			}
			reason := plain.ClockReason(b.lane)
			if tc.interval == "" && strings.Contains(reason, "proof.trunk-every") {
				t.Fatalf("unset key did not use off: %q", reason)
			}
			if tc.interval != "24h" && !strings.HasPrefix(reason, "automatic full check is off; main is proven at each landing") {
				t.Fatalf("off reason %q", reason)
			}
			if tc.name == "invalid" && !strings.Contains(reason, `proof.trunk-every is "4h"; it takes off or at least 24h`) {
				t.Fatalf("invalid reason %q", reason)
			}
		})
	}
}

func TestRecoverMovedMainProvesOnlyWhatIsUnproved(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, interval string
		red, unproved  bool
		want           int
	}{
		{name: "unproved executable main", unproved: true, want: 1}, {name: "known red", red: true}, {name: "batch push proven main", interval: "24h"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			b := newCadenceBed(t, tc.interval, 0, false, tc.red, tc.unproved)
			origin := b.now
			for tick := 0; tick < 48; tick++ {
				b.now = origin.Add(time.Duration(tick) * time.Hour)
				run := b.keeper.Run()
				if run.Outcome == lane.AgentFailed || len(run.Problems) > 0 {
					t.Fatalf("tick %d: %+v", tick, run)
				}
			}
			recovery := b.recoveries
			if recovery != tc.want {
				t.Fatalf("recovery launches=%d want=%d argv=%v", recovery, tc.want, b.argv)
			}
			if tc.interval == "24h" && (len(b.argv) != 1 || !b.started[0].Equal(origin.Add(24*time.Hour))) {
				t.Fatalf("daily launches %v at %v", b.argv, b.started)
			}
		})
	}
}
