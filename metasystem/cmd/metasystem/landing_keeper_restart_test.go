package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/config"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/helm"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/lane"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/plain"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/outage"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/textui"
)

func TestLandingKeeperStatusJSONNamesOneTick(t *testing.T) {
	t.Parallel()
	bed, _, _, _, _ := landingRestartBed(t)
	queueRestartWork(t, bed)
	owners := bed.owners()
	owners.landing.wake = func(string) lane.WakeSources {
		return lane.WakeSources{Reasons: func(string) ([]string, error) { return []string{plain.WakeQueued}, nil }}
	}
	owners.landing.status = func(_ string, _ lane.Record, view lane.View) landingStatusData {
		waiting, err := plain.Waiting(bed.landingA)
		if err != nil {
			t.Fatal(err)
		}
		return landingStatusData{View: view, Queue: waiting}
	}
	command, _ := findIntentAction("landing", "status")
	var stdout, stderr bytes.Buffer
	code := runIntentIn(command, []string{"--json"}, &stdout, &stderr, bed.cwd, owners)
	var result struct {
		Summary string
		Data    landingStatusData
	}
	if err := json.Unmarshal(stdout.Bytes(), &result); code != 0 || err != nil {
		t.Fatalf("landing status --json: exit=%d, decode=%v, stdout=%q, stderr=%q", code, err, stdout.String(), stderr.String())
	}
	const want = "idle; its agent starts within one tick (15 s)"
	if !strings.Contains(result.Summary, want) || !strings.Contains(result.Data.Summary, want) || len(result.Data.Queue) != 2 {
		t.Fatalf("landing status --json: summary=%q, data=%+v; want %q with two waiting lines", result.Summary, result.Data, want)
	}
	t.Logf("landing status --json: exit=%d, summary=%q", code, result.Summary)
}

func TestLandingKeeperStatusShowsBarrenHold(t *testing.T) {
	t.Parallel()
	bed, keeper, now, starts, _ := landingRestartBed(t)
	queueRestartWork(t, bed)
	for tick := 0; tick < 4; tick++ {
		*now = now.Add(lane.AgentTick)
		keeper.Run()
		bed.alive = false
	}
	if run := keeper.Run(); run.Outcome != lane.AgentHeld || *starts != 2 {
		t.Fatalf("unchanged work: %+v, starts=%d; want two starts then a hold", run, *starts)
	}
	owners := bed.owners()
	owners.landing.wake = func(string) lane.WakeSources { return keeper.Sources }
	owners.landing.status = func(_ string, _ lane.Record, view lane.View) landingStatusData {
		waiting, err := plain.Waiting(bed.landingA)
		if err != nil {
			t.Fatal(err)
		}
		return landingStatusData{View: view, Queue: waiting}
	}
	command, _ := findIntentAction("landing", "status")
	const want = "held after 2 runs that left the lane unchanged; a person's metasystem landing run starts it"
	for _, args := range [][]string{{"--json"}, {}} {
		var stdout, stderr bytes.Buffer
		code := runIntentIn(command, args, &stdout, &stderr, bed.cwd, owners)
		if code != 0 || !strings.Contains(oneSpaced(stdout.String()), want) {
			t.Fatalf("landing status %v: exit=%d, stdout=%q, stderr=%q; want %q", args, code, stdout.String(), stderr.String(), want)
		}
		if len(args) > 0 {
			var result struct {
				Summary string
				Data    landingStatusData
			}
			if err := json.Unmarshal(stdout.Bytes(), &result); err != nil || result.Data.Owner.State != lane.OwnerHeld || result.Data.Owner.Barren != 2 || !strings.Contains(result.Summary, want) || !strings.Contains(result.Data.Summary, want) {
				t.Fatalf("held JSON: decode=%v, result=%+v", err, result)
			}
		}
		t.Logf("landing status %v: exit=%d, output=%q", args, code, oneSpaced(stdout.String()))
	}
	*now = now.Add(lane.AgentTick)
	if run := keeper.Run(); run.Outcome != lane.AgentHeld || *starts != 2 {
		t.Fatalf("status lifted the hold: %+v, starts=%d", run, *starts)
	}
	if _, _, err := plain.HandIn(bed.landingA, plain.Line{Goal: "new", SHA: "new"}); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	if code := runIntentIn(command, []string{"--json"}, &stdout, &stderr, bed.cwd, owners); code != 0 || !strings.Contains(stdout.String(), "idle; its agent starts within one tick (15 s)") {
		t.Fatalf("changed lane is still held: exit=%d, stdout=%q, stderr=%q", code, stdout.String(), stderr.String())
	}
}

func landingRestartBed(t *testing.T) (*laneVerbBed, *lane.AgentKeeper, *time.Time, *int, *bool) {
	t.Helper()
	root := resolvedPath(t.TempDir())
	home := resolvedPath(t.TempDir())
	bed := &laneVerbBed{cwd: root, home: home, landingA: root}
	if err := os.MkdirAll(lane.HostDir(home), 0o755); err != nil {
		t.Fatal(err)
	}
	record, _ := json.Marshal(lane.Record{Root: root, Install: root, CustodyEpoch: 1})
	if err := os.WriteFile(filepath.Join(lane.HostDir(home), "landing-lane.json"), record, 0o600); err != nil {
		t.Fatal(err)
	}
	// Keeper launches prepare a selection from these queue and main facts;
	// no external Git repository or host policy participates in this fixture.
	bed.policies = config.PolicyReaders{
		Registry: func(string) (config.PolicyRegistry, error) { return config.PolicyRegistry{}, nil },
		ConfPath: func(string) (string, error) { return filepath.Join(root, "metasystem.conf"), nil },
		Helm:     func(string) helm.State { return helm.State{} },
	}
	falseState := replayFalseState(t)
	bed.plainProve = plain.ProveSeams{Incidents: func(string, string, string) ([]goal.TrunkRedEntry, error) { return nil, nil }, Now: func() time.Time { return laneTestNow }, Git: func(_ string, args ...string) (string, error) {
		switch args[0] {
		case "fetch":
			return "", nil
		case "rev-parse":
			return "main", nil
		case "cat-file":
			return "", nil
		case "merge-base":
			return "", &exec.ExitError{ProcessState: falseState}
		default:
			t.Fatalf("unstubbed selection Git: %v", args)
			return "", nil
		}
	}}
	now, starts, proofAlive := laneTestNow, 0, false
	keeper := newLandingAgentKeeper(root, home, landingAgent{now: func() time.Time { return now }, machine: func(string) (string, error) { return "lane-fixture", nil }})
	keeper.Prepare = func(record lane.Record) error {
		_, err := plain.SelectBatch(record.Install, record.Root, record, bed.plainProve)
		return err
	}
	if keeper.Fingerprint == nil {
		t.Fatal("the landing keeper has no lane fingerprint, so barren runs cannot hold it")
	}
	keeper.Fingerprint = func(string) (string, error) {
		entries, err := plain.Entries(root)
		if err != nil {
			return "", err
		}
		data, err := json.Marshal(entries)
		return string(data), err
	}
	keeper.Sources.Reasons = func(string) ([]string, error) {
		waiting, err := plain.Waiting(root)
		if err != nil || len(waiting) == 0 {
			return nil, err
		}
		return []string{plain.WakeQueued}, nil
	}
	keeper.Running = func() (string, bool, error) { return "fixture-agent", bed.alive, nil }
	keeper.Start = func(string, lane.Wake) (string, error) { starts++; bed.alive = true; return "fixture-agent", nil }
	keeper.Reap = nil
	keeper.Waiting = func(install, _ string) (int, error) {
		waiting, err := plain.Waiting(install)
		return len(waiting), err
	}
	keeper.Holds[0] = func(string) (string, error) {
		return plain.ProofHold(root, plain.ProveSeams{Alive: func(plain.Running) bool { return proofAlive }})
	}
	bed.keeper = func(string, string) lane.AgentKeeper { return keeper }
	return bed, &keeper, &now, &starts, &proofAlive
}

func queueRestartWork(t *testing.T, bed *laneVerbBed) {
	t.Helper()
	for _, goal := range []string{"first", "second"} {
		if _, _, err := plain.HandIn(bed.landingA, plain.Line{Goal: goal, SHA: goal, At: laneTestNow.Format(time.RFC3339)}); err != nil {
			t.Fatal(err)
		}
	}
}

func TestLandingKeeperRestartsOnEveryTickWithWaitingWork(t *testing.T) {
	t.Parallel()
	bed, keeper, now, starts, _ := landingRestartBed(t)
	if run := keeper.Run(); run.Outcome != lane.AgentIdle || *starts != 0 {
		t.Fatalf("empty queue: %+v, starts=%d", run, *starts)
	}
	queueRestartWork(t, bed)
	if err := os.WriteFile(filepath.Join(lane.HostDir(bed.home), "landing-agent-keeper.json"), []byte(`{"barren":2,"barrenFingerprint":"unchanged","barrenLaunches":["old-1","old-2"]}`), 0o600); err != nil {
		t.Fatal(err)
	}
	for tick := 1; tick <= 4; tick++ {
		*now = now.Add(15 * time.Second)
		run := keeper.Run()
		if run.Outcome != lane.AgentStarted || *starts != tick || !strings.HasPrefix(run.Line, fmt.Sprintf("lane agent started by the keeper: %d waiting line(s)", tick+1)) {
			t.Fatalf("tick %d: %+v, starts=%d; waiting work must restart", tick, run, *starts)
		}
		bed.alive = false
		// A new hand-in changes the lane and lifts its barren hold.
		if _, _, err := plain.HandIn(bed.landingA, plain.Line{Goal: fmt.Sprintf("new-%d", tick), SHA: fmt.Sprintf("new-%d", tick)}); err != nil {
			t.Fatal(err)
		}
	}
}

func TestLandingKeeperPublicRunLiftsBarrenHold(t *testing.T) {
	t.Parallel()
	bed, keeper, now, starts, _ := landingRestartBed(t)
	queueRestartWork(t, bed)
	for tick := 0; tick < 4; tick++ {
		*now = now.Add(lane.AgentTick)
		keeper.Run()
		bed.alive = false
	}
	if run := keeper.Run(); run.Outcome != lane.AgentHeld || *starts != 2 {
		t.Fatalf("unchanged work: %+v, starts=%d; want two starts then a hold", run, *starts)
	}
	if code, stdout, stderr := bed.run(t, "landing", "run", "--json"); code != 0 || *starts != 3 || !strings.Contains(oneSpaced(stdout), `"outcome": "started"`) {
		t.Fatalf("public run with barren hold: %d %q %q, starts=%d; want a third start", code, stdout, stderr, *starts)
	}
}

func TestLandingKeeperPublicRunHoldsLiveAttemptAndAgent(t *testing.T) {
	t.Parallel()
	bed, keeper, _, starts, proofAlive := landingRestartBed(t)
	queueRestartWork(t, bed)
	if err := os.WriteFile(filepath.Join(plain.Dir(bed.landingA), "running.json"), []byte(`{"attempt":"fixture-proof","pid":42,"tree":"fixture-tree"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	*proofAlive = true
	if run := keeper.Run(); run.Outcome != lane.AgentHeld || *starts != 0 {
		t.Fatalf("live attempt: %+v, starts=%d", run, *starts)
	}
	if code, stdout, stderr := bed.run(t, "landing", "run", "--json"); code == 0 || *starts != 0 || !strings.Contains(stdout+stderr, "fixture-proof") {
		t.Fatalf("public run during proof: %d %q %q, starts=%d", code, stdout, stderr, *starts)
	}
	*proofAlive = false
	if code, stdout, stderr := bed.run(t, "landing", "run", "--json"); code != 0 || *starts != 1 || !strings.Contains(oneSpaced(stdout), `"outcome": "started"`) {
		t.Fatalf("public run after proof: %d %q %q, starts=%d", code, stdout, stderr, *starts)
	}
	if run := keeper.Run(); run.Outcome != lane.AgentRunning || *starts != 1 {
		t.Fatalf("live agent: %+v, starts=%d", run, *starts)
	}
	if code, stdout, stderr := bed.run(t, "landing", "run", "--json"); code != 0 || *starts != 1 || !strings.Contains(oneSpaced(stdout), `"outcome": "running"`) {
		t.Fatalf("repeated public run: %d %q %q, starts=%d", code, stdout, stderr, *starts)
	}
}

func TestLandingKeeperStartsWhenProviderHoldExpires(t *testing.T) {
	t.Parallel()
	bed, keeper, now, starts, _ := landingRestartBed(t)
	queueRestartWork(t, bed)
	if _, err := outage.Record(bed.landingA, "overloaded", "fixture", "fixture", *now); err != nil {
		t.Fatal(err)
	}
	first := keeper.Run()
	*now = now.Add(15 * time.Second)
	second := keeper.Run()
	if first.Outcome != lane.AgentHeld || second.Outcome != lane.AgentHeld || first.Line != second.Line || *starts != 0 {
		t.Fatalf("provider hold: %+v, %+v, starts=%d", first, second, *starts)
	}
	*now = now.Add(outage.Horizon)
	if run := keeper.Run(); run.Outcome != lane.AgentStarted || *starts != 1 {
		t.Fatalf("hold expired: %+v, starts=%d", run, *starts)
	}
}

func TestLandingKeeperStatusNamesOneTick(t *testing.T) {
	t.Parallel()
	root := "/fixture/lane"
	view := lane.View{Root: &root}
	view.Owner.State = lane.OwnerIdle
	for _, waiting := range []bool{false, true} {
		want := "idle; its landing agent starts when there is work"
		view.Wake = &lane.Wake{}
		if waiting {
			want = "idle; its agent starts within one tick (15 s)"
			view.Wake.Reasons = []string{plain.WakeQueued}
		}
		page := textui.New(textui.Env{ASCII: true})
		(&intentInvocation{}).landingStatusView(view, false, nil, waiting)(page)
		if got := page.String(); !strings.Contains(oneSpaced(got), want) {
			t.Fatalf("landing status: %q; want %q", got, want)
		}
		page = textui.New(textui.Env{ASCII: true, Verbose: true})
		(statusBoard{lane: &view}).drawLane(page, page.Env())
		if got := page.String(); !strings.Contains(oneSpaced(got), want) {
			t.Fatalf("status board: %q; want %q", got, want)
		}
	}
}
