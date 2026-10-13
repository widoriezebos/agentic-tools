package main

import (
	"encoding/json"
	"fmt"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/lane"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/plain"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/state"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/stateroot"
)

func TestLandingTornShadowStopRetainsPersonAndRevokesContinuation(t *testing.T) {
	t.Parallel()
	b, _ := batchVerbFixture(t, "sealed")
	b.owners.landing.person = func(string) (string, error) { return "Wido", nil }
	code, out := b.run(t, b.seat, "landing", "batch", "reopen", "--with", "shared-state-in-the-ledger", "--by", "Wido")
	if code != 0 {
		t.Fatalf("reopen: %d %s", code, out)
	}
	if plain.PersonBatchContinuation(b.lane, b.record, b.home) == "" {
		t.Fatal("fixture has no continuation to revoke")
	}
	stoppedRegeneration := false
	b.owners.landing.stopRegeneration = func(install string) error {
		if install != b.lane {
			t.Errorf("stopped regeneration in %s", install)
		}
		stoppedRegeneration = true
		return nil
	}
	helmMust(t, os.WriteFile(filepath.Join(plain.Dir(b.lane), "state.json"), []byte(`{torn`), 0600))
	code, out = b.run(t, b.seat, "landing", "stop", "--json")
	if code != 0 || !stoppedRegeneration {
		t.Fatalf("stop: %d %s regeneration stopped=%v", code, out, stoppedRegeneration)
	}
	pause, present := lane.ReadPause(b.home)
	if !present || pause.By != "Wido" || plain.PersonBatchContinuation(b.lane, b.record, b.home) != "" {
		t.Fatalf("stop lost authority or continuation: %+v", pause)
	}
	data, err := os.ReadFile(filepath.Join(plain.Dir(b.lane), "stop-person.json"))
	helmMust(t, err)
	var person plain.ActProvenance
	helmMust(t, json.Unmarshal(data, &person))
	if person.Person != "Wido" || person.Kind != "stop" || person.Destination != b.record {
		t.Fatalf("stop person: %+v", person)
	}
	// Force the start itself to encounter the torn shadow too.
	helmMust(t, os.WriteFile(filepath.Join(plain.Dir(b.lane), "state.json"), []byte(`{torn`), 0600))
	code, out = b.run(t, b.seat, "landing", "start")
	if code != 0 {
		t.Fatalf("start: %d %s", code, out)
	}
	if _, paused := lane.ReadPause(b.home); paused {
		t.Fatal("start left pause")
	}
	shadow, present, err := state.Read(plain.Dir(b.lane))
	if err != nil || !present || shadow.Lane.Word != "running" {
		t.Fatalf("start repair: %+v %v %v", shadow.Lane, present, err)
	}
}

func TestLandingKeeperLaunchAndReapEmitTransitionsWithoutDrift(t *testing.T) {
	t.Parallel()
	b := shadowCorrectionVerbBed(t)
	registration, _, err := lane.Read(b.home)
	helmMust(t, err)
	now := laneTestNow
	live := false
	s := b.plainProve
	s.AgentProbe = func(string) (lane.OwnerProbe, error) { return lane.OwnerProbe{Alive: live, Since: now}, nil }
	b.owners.landing.probe = s.AgentProbe
	b.owners.landing.plainProve = s
	keeper := lane.AgentKeeper{Home: b.home, Self: registration.Root, Now: func() time.Time { return now },
		Sources:     lane.WakeSources{Reasons: func(string) ([]string, error) { return []string{plain.WakeQueued}, nil }},
		Running:     func() (string, bool, error) { return "agent", live, nil },
		Start:       func(string, lane.Wake) (string, error) { live = true; return "agent", nil },
		Fingerprint: func(string) (string, error) { return "same", nil },
	}
	b.owners.landing.keeper = func(string, string) lane.AgentKeeper { return keeper }
	_, err = plain.RefreshShadow(b.home, now, "keeper", s)
	helmMust(t, err)
	for _, action := range []string{"run", "stop", "start"} {
		code, out, stderr := b.run(t, "landing", action)
		if code != 0 {
			t.Fatalf("%s: %d %s %s", action, code, out, stderr)
		}
	}
	live = false
	now = now.Add(time.Minute)
	_, err = lane.SetPause(b.home, "Wido", now)
	helmMust(t, err)
	if run := keeper.Run(); run.Outcome != lane.AgentPaused {
		t.Fatalf("reap: %+v", run)
	}
	_, err = plain.RefreshShadow(b.home, now, "keeper", s)
	helmMust(t, err)
	events, _, err := state.ReadEvents(plain.Dir(registration.Install))
	helmMust(t, err)
	agent, holds := 0, 0
	for _, event := range events {
		if event.Kind == "drift" || event.Kind == "observation-unstable" {
			t.Fatalf("keeper drift: %+v", event)
		}
		if event.Kind == "agent" && event.By == "keeper" {
			agent++
		}
		if event.Kind == "holds" && event.By == "keeper" {
			holds++
		}
	}
	if agent != 2 || holds != 1 {
		t.Fatalf("launch/reap events: agent=%d holds=%d %+v", agent, holds, events)
	}
}

func TestLandingEngineIncidentWritesEmitTransitions(t *testing.T) {
	t.Parallel()
	b := shadowCorrectionVerbBed(t)
	registration, _, err := lane.Read(b.home)
	helmMust(t, err)
	s := b.owners.landing.plainProve
	s.BudgetHome = b.home
	s.Now = func() time.Time { return laneTestNow }
	b.owners.landing.plainProve = s
	_, err = plain.RefreshShadow(b.home, laneTestNow, "keeper", s)
	helmMust(t, err)
	command, _ := findIntentAction("landing", "health")
	command.run = func(*intentInvocation) int { panic("fixture engine failure") }
	for range 2 {
		code := runIntentIn(command, nil, io.Discard, io.Discard, b.cwd, b.owners)
		if code != 2 {
			t.Fatalf("guard: %d", code)
		}
	}
	code, out, stderr := b.run(t, "landing", "incident", "clear", "--all")
	if code != 0 {
		t.Fatalf("clear: %d %s %s", code, out, stderr)
	}
	events, _, err := state.ReadEvents(plain.Dir(registration.Install))
	helmMust(t, err)
	writers := []string{}
	for _, event := range events {
		if event.Kind == "drift" {
			t.Fatalf("incident drift: %+v", event)
		}
		if event.Kind == "engine" {
			writers = append(writers, event.By)
		}
	}
	if !reflect.DeepEqual(writers, []string{"health", "health", "incident"}) {
		t.Fatalf("incident writers: %v", writers)
	}
}

// Commands own their writer names. Git and proof execution answer from this
// synthetic checkout so a healthy verb cannot conceal a transition as drift.
func TestLandingWriterTransitionsDriveThrough(t *testing.T) {
	t.Parallel()
	b := newReplayVerbBed(t)
	b.owners.resolver = stateroot.NewResolver(func(string) (string, error) { return b.root, nil }, os.Executable)
	b.owners.prove = enrolledPersonProver(t, b.install, laneTestNow)
	b.owners.landing.mainEndpoint = func(string) (goal.Endpoint, error) {
		return goal.Endpoint{Root: b.root, Repository: emptyTransitionLedger{}}, nil
	}
	b.prepareBatch(t)
	batch, err := plain.ReadBatch(b.install)
	helmMust(t, err)
	batch.Open = true
	writeQuestionFixture(t, filepath.Join(plain.Dir(b.install), "batch.json"), batch)
	helmMust(t, os.WriteFile(filepath.Join(b.install, "metasystem.conf"), []byte("metasystem.template=true\ntesting.contract=testing.json\nproof.full=fixture\nproof.cheap=fixture\n"), 0600))
	s := b.owners.landing.plainProve
	s.BudgetHome = b.home
	s.Launch = func(_ []string, _ string, log string) (int64, error) {
		return int64(os.Getpid()), os.WriteFile(log, nil, 0600)
	}
	s.Incidents = func(string, string, string) ([]goal.TrunkRedEntry, error) { return nil, nil }
	s.Machine = func(string) (string, error) { return "fixture", nil }
	main := "main"
	git := s.Git
	s.Git = func(dir string, args ...string) (string, error) {
		if len(args) == 2 && args[0] == "show" && strings.HasSuffix(args[1], ":metasystem/testing.json") {
			return b.contract, nil
		}
		if args[0] == "worktree" && len(args) > 4 && args[1] == "add" {
			result, err := git(dir, args...)
			if err == nil {
				err = os.WriteFile(filepath.Join(args[3], "metasystem", "testing.json"), []byte(b.contract), 0600)
			}
			return result, err
		}
		if args[0] == "push" {
			main = b.head
			return "", nil
		}
		if args[0] == "rev-parse" && strings.Contains(args[len(args)-1], "origin/main^{commit}") {
			return main, nil
		}
		return git(dir, args...)
	}
	s.ImpactCost = func(string, string) (int, bool, int, string, error) {
		return 100, false, 110, "fixture whole check", nil
	}
	s.Command = func(cmd *exec.Cmd) error {
		if len(cmd.Args) > 2 && cmd.Args[1] == "test" && cmd.Args[2] == "impact" {
			writeImpactPlanResult(t, cmd, "plan: base fixture\nselection: fixture\n")
			return nil
		}
		if commandEnv(cmd, "LANDING_ONLY") == "fast-static-build" {
			fmt.Fprint(cmd.Stdout, "landing group fast-static-build green 1\n")
		} else {
			fmt.Fprint(cmd.Stdout, "LANDING-CHECKED\t0\n")
		}
		return nil
	}
	b.owners.landing.plainProve = s
	b.owners.landing.machine = s.Machine
	_, err = plain.RefreshShadow(b.home, laneTestNow, "keeper", s)
	helmMust(t, err)
	steps := []struct {
		words    []string
		kind, by string
	}{
		{[]string{"stop"}, "lane", "stop"},
		{[]string{"start"}, "lane", "resume"},
		{[]string{"batch", "seal", "--by", "Wido"}, "batch.execution", "batch"},
		{[]string{"prove", "--gate", "--wait"}, "proof", "prove"},
		{[]string{"prove", "--wait"}, "proof", "prove"},
		{[]string{"push"}, "main", "push"},
	}
	for _, step := range steps {
		before, _, err := state.ReadEvents(plain.Dir(b.install))
		helmMust(t, err)
		code, out := b.run(t, b.root, step.words...)
		if code != 0 {
			t.Fatalf("%v: %d %s", step.words, code, out)
		}
		events, _, err := state.ReadEvents(plain.Dir(b.install))
		helmMust(t, err)
		found := false
		for _, event := range events[len(before):] {
			if event.Kind == "drift" || event.Kind == "observation-unstable" || event.Kind == "defect" {
				t.Fatalf("%v: %+v; all new events: %+v", step.words, event, events[len(before):])
			}
			if event.Kind == step.kind && event.By == step.by {
				found = true
			}
		}
		if !found {
			t.Fatalf("%v omitted %s by %s: %+v", step.words, step.kind, step.by, events[len(before):])
		}
		if diff, err := plain.RefreshShadow(b.home, laneTestNow, "keeper", s); err != nil || len(diff) != 0 {
			t.Fatalf("%v refresh: %v %v", step.words, diff, err)
		}
	}
}

// Only committed incident reads are needed by this empty ledger fixture.
type emptyTransitionLedger struct{ goal.Repository }

func (emptyTransitionLedger) Files(string, ...string) (map[string][]byte, error) {
	return map[string][]byte{}, nil
}
