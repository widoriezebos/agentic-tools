package main

import (
	"encoding/json"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/lane"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/plain"
)

func init() {
	registerIdempotency("landing resume", idemStateful, "a healthy running lane changes no record", landingResumeRepeat)
}
func TestLandingResumeRunningLaneRepeat(t *testing.T) { t.Parallel(); landingResumeRepeat(t) }
func landingResumeRepeat(t *testing.T) {
	t.Helper()
	b := newSelectionBed(t)
	b.owners.prove = enrolledPersonProver(t, b.seat, b.now)
	git := b.owners.landing.plainProve.Git
	absent := exec.Command("/bin/sh", "-c", "exit 128").Run()
	b.owners.landing.plainProve.Git = func(dir string, args ...string) (string, error) {
		if strings.Contains(strings.Join(args, " "), "MERGE_HEAD") {
			return "", absent
		}
		return git(dir, args...)
	}
	b.keeper.Running = func() (string, bool, error) { return "running-agent", true, nil }
	code, out := b.run(t, b.seat, "landing", "resume", "--json")
	if code != 0 || !strings.Contains(out, "nothing to resume") {
		t.Fatalf("exit=%d: %s", code, out)
	}
	beforeHome, beforeLane := idemTreeDigest(t, b.home), idemTreeDigest(t, b.lane)
	b.now = b.now.Add(time.Minute)
	code, out = b.run(t, b.seat, "landing", "resume", "--json")
	if code != 0 || !strings.Contains(out, "nothing to resume") {
		t.Fatalf("repeat exit=%d: %s", code, out)
	}
	idemSameTree(t, "running resume", beforeHome, idemTreeDigest(t, b.home))
	idemSameTree(t, "running resume", beforeLane, idemTreeDigest(t, b.lane))
}

func TestLandingRestartContinuesHeldSelection(t *testing.T) {
	t.Parallel()
	b, _ := batchVerbFixture(t, "sealed")
	b.keeper.Fingerprint = func(string) (string, error) { return "same", nil }
	writeQuestionFixture(t, lane.AgentStatePath(b.home), lane.AgentState{Barren: 2, BarrenFingerprint: "same"})
	// The hold is represented by the keeper record; its stop register is empty.
	code, out := b.run(t, b.seat, "landing", "restart", "--by", "Wido")
	if code != 0 || b.starts != 1 {
		t.Fatalf("exit=%d starts=%d: %s", code, b.starts, out)
	}
	state, err := lane.ReadAgentState(b.home)
	if err != nil || state.Barren != 0 {
		t.Fatalf("hold=%+v err=%v", state, err)
	}
	batch := b.batch(t)
	if batch.Person == nil || len(batch.Person.Subject) != len(batch.Members) {
		t.Fatalf("selection=%+v", batch)
	}
}

func TestLandingStopSettlesAndRevokesContinuation(t *testing.T) {
	t.Parallel()
	b, _ := batchVerbFixture(t, "sealed")
	_, err := lane.SetPause(b.home, "Wido", b.now)
	helmMust(t, err)
	code, out := b.run(t, b.seat, "landing", "run", "--goals", "a,b", "--by", "Wido")
	if code != 0 {
		t.Fatalf("selection exit=%d: %s", code, out)
	}
	if plain.PersonBatchContinuation(b.lane, b.record, b.home) == "" {
		t.Fatal("selection did not cover pause")
	}
	live := true
	settles := 0
	b.owners.landing.probe = func(string) (lane.OwnerProbe, error) { return lane.OwnerProbe{Alive: live}, nil }
	b.owners.landing.person = func(string) (string, error) { return "Wido", nil }
	b.owners.landing.settle = settleLandingAgent(func() (string, bool, error) { return "agent", live, nil }, func(string) error { settles++; live = false; return nil })
	code, out = b.run(t, b.seat, "landing", "stop", "--json")
	var result intentResult
	helmMust(t, json.Unmarshal([]byte(out), &result))
	if code != 0 || settles != 1 || live || !strings.Contains(result.Summary, "the landing agent's session ended") {
		t.Fatalf("exit=%d settles=%d live=%v result=%+v", code, settles, live, result)
	}
	if plain.PersonBatchContinuation(b.lane, b.record, b.home) != "" {
		t.Fatal("stop retained continuation")
	}
	before := b.starts
	if run := b.keeper.Run(); run.Outcome != lane.AgentPaused || b.starts != before {
		t.Fatalf("keeper after stop=%+v starts=%d", run, b.starts)
	}
	code, out = b.run(t, b.seat, "landing", "stop", "--json")
	if code != 0 || !strings.Contains(out, "no landing agent was running") || settles != 1 {
		t.Fatalf("repeat exit=%d settles=%d: %s", code, settles, out)
	}
}
func TestLandingResumeRequiresPerson(t *testing.T) {
	t.Parallel()
	b := newSelectionBed(t)
	before := idemTreeDigest(t, b.home)
	code, out := b.run(t, b.seat, "landing", "resume", "--json")
	if code == 0 || !strings.Contains(out, "this terminal is not enrolled for the landing install; type: metasystem system enroll --name NAME") {
		t.Fatalf("exit=%d: %s", code, out)
	}
	idemSameTree(t, "unproved resume", before, idemTreeDigest(t, b.home))
}
