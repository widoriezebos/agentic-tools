package main

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/lane"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/plain"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/launch"
)

// Plain lane step 6: the keeper of the registered nested lane wakes the
// landing agent with "queued" once a hand-in waits; holds the start while
// running.json names a live proof; and, once that proof ended after the
// agent's launch, wakes it with "proof-finished" too. landing status
// --json shows the same wake reasons. Nothing else in the keeper changes.
func TestKeeperWakesOnQueuedAndProofFinishedAndHoldsWhileProving(t *testing.T) {
	t.Parallel()
	bed := newPlainVerbBed(t)
	module := bed.installation
	now := laneTestNow
	// The launcher finds the lane installation by its module file
	// (batch.ModuleRoot), as for any landing agent.
	for path, text := range map[string]string{
		filepath.Join(module, "go.mod"):                "module fixture\n",
		filepath.Join(module, "metasystem.conf.local"): "launch.landing.runtime=claude\nlaunch.landing.model=claude-roster-model\n",
	} {
		if err := os.WriteFile(path, []byte(text), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	seedRecentTrunkCheck(t, module, bed.home)
	store := launch.Store{Root: filepath.Join(t.TempDir(), "launches")}
	manager := &launch.Manager{Store: store, Adapters: map[string]launch.Adapter{"claude-headless": launch.ClaudeHeadless{Binary: "/fixture/bin/claude", ProjectsRoot: filepath.Join(t.TempDir(), "projects")}},
		Supervisor: recordingSupervisor{store}, Now: func() time.Time { return now }, Sleep: func(time.Duration) {}, Poll: time.Second, StartCap: time.Minute,
		Lane: landingLaneCheckout(func() (string, error) { return bed.home, nil })}
	nonces := 0
	agent := landingAgent{manager: func() *launch.Manager { return manager }, settings: installationSettings, now: func() time.Time { return now },
		nonce: func() (string, error) { nonces++; return strings.Repeat(string(rune('0'+nonces)), 16), nil }}
	keeper := newLandingAgentKeeper(module, bed.home, agent)
	wake := func(home string) lane.WakeSources {
		return lane.WakeSources{Reasons: func(string) ([]string, error) {
			state, err := lane.ReadAgentState(home)
			if err != nil {
				return nil, err
			}
			launched, _ := time.Parse(time.RFC3339, state.StartedAt)
			return plain.WakeReasons(module, bed.checkout, launched, now)
		}}
	}
	keeper.Sources = wake(bed.home)
	bed.owners.landing.wake = wake
	launches := func() int {
		t.Helper()
		records, err := store.List()
		if err != nil {
			t.Fatal(err)
		}
		return len(records)
	}
	endAll := func() {
		t.Helper()
		records, _ := store.List()
		for _, record := range records {
			if _, err := store.Update(record.ID, func(r *launch.Record) error {
				r.State, r.FinishedAt = launch.Completed, now.Format(time.RFC3339Nano)
				return nil
			}); err != nil {
				t.Fatal(err)
			}
		}
	}

	if line := keeper.Step(); launches() != 0 || !strings.Contains(line, "idle") {
		t.Fatalf("an empty queue wakes nothing: %q", line)
	}
	bed.merge(t, bed.seat(t, "goal-a"))
	run := keeper.Run()
	if run.Outcome != lane.AgentStarted || strings.Join(run.Reasons, ",") != plain.WakeQueued || launches() != 1 {
		t.Fatalf("a hand-in wakes the agent: %+v", run)
	}
	if wake, _ := bed.status(t)["wake"].(map[string]any); wake == nil || !strings.Contains(strings.Join(anyStrings(wake["reasons"].([]any)), ","), plain.WakeQueued) {
		t.Fatalf("status shows the wake: %v", wake)
	}
	endAll()

	// The agent started a proof and ended its turn: the keeper holds.
	exact, live, err := identity.KernelProber{}.Probe(int64(os.Getpid()))
	if err != nil || live != identity.Alive {
		t.Fatal(err)
	}
	ref, err := identity.EncodeRef(exact.Ref())
	if err != nil {
		t.Fatal(err)
	}
	tree := bed.git(t, bed.checkout, "rev-parse", "HEAD^{tree}")
	running := `{"attempt":"a1","tree":"` + tree + `","since":"` + now.Format(time.RFC3339) + `","pid":` + strconv.Itoa(os.Getpid()) + `,"process":"` + ref + `"}`
	if err := os.WriteFile(filepath.Join(plain.Dir(module), "running.json"), []byte(running), 0o644); err != nil {
		t.Fatal(err)
	}
	if run := keeper.Run(); run.Outcome != lane.AgentHeld || !strings.Contains(run.Line, "a1") || launches() != 1 {
		t.Fatalf("a running proof holds the start: %+v", run)
	}

	// The proof ended after the agent's launch: proof-finished.
	if err := os.Remove(filepath.Join(plain.Dir(module), "running.json")); err != nil {
		t.Fatal(err)
	}
	result := `{"tree":"` + tree + `","result":"green","log":"/x.log","at":"` + now.Add(time.Minute).Format(time.RFC3339) + `"}` + "\n"
	// Appended, so the bed's recent trunk check still stands.
	previous, err := os.ReadFile(filepath.Join(plain.Dir(module), "results.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(plain.Dir(module), "results.jsonl"), append(previous, result...), 0o644); err != nil {
		t.Fatal(err)
	}
	run = keeper.Run()
	if run.Outcome != lane.AgentStarted || strings.Join(run.Reasons, ",") != plain.WakeQueued+","+plain.WakeProofFinished || launches() != 2 {
		t.Fatalf("a finished proof wakes the agent: %+v", run)
	}
}
