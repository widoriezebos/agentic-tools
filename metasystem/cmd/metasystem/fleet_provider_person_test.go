package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/lane"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/plain"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/launch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/outage"
)

func TestFleetProviderMarkPublicPersonStart(t *testing.T) {
	t.Parallel()
	b := newPlainVerbBed(t)
	b.cwd = b.checkout
	b.seat(t, "one")
	settings := launch.DefaultSettings()
	settings.LandingRuntime, settings.LandingModel = "claude", "landing-model"
	store := launch.Store{Root: t.TempDir()}
	manager := &launch.Manager{Store: store, Adapters: map[string]launch.Adapter{"claude-headless": launch.ClaudeHeadless{Binary: "/fixture/bin/claude"}},
		Supervisor: recordingSupervisor{store}, Now: func() time.Time { return laneTestNow }, Sleep: func(time.Duration) {}, Poll: time.Second, StartCap: time.Minute,
		Lane: landingLaneCheckout(func() (string, error) { return b.home, nil })}
	agent := newTestLandingAgent(func(agent *landingAgent) {
		agent.manager = func() *launch.Manager { return manager }
		agent.now = func() time.Time { return laneTestNow }
		agent.settings = func(string) (launch.Settings, error) { return settings, nil }
		agent.nonce = func() (string, error) { return fmt.Sprintf("11223344556677%02d", len(landingLaunches(t, store))), nil }
		agent.machine = func(string) (string, error) { return "landing", nil }
	})
	b.owners.landing.keeper = func(home, root string) lane.AgentKeeper { return newLandingAgentKeeper(root, home, agent) }
	if _, err := outage.Observe(b.home, "claude", "landing-model", outage.ProviderLimit, "HTTP 429 Too Many Requests", "limited-call", laneTestNow); err != nil {
		t.Fatal(err)
	}
	if code, text := b.run(t, "landing", "run", "--json"); code == 0 || !strings.Contains(text, "provider") || len(landingLaunches(t, store)) != 0 {
		t.Fatalf("an agent start bypassed the provider hold: exit %d %s", code, text)
	}
	b.owners.prove = enrolledPersonProver(t, b.installation, laneTestNow)
	if code, text := b.run(t, "landing", "run", "--goals", "one", "--by", "Wido", "--json"); code != 0 || len(landingLaunches(t, store)) != 1 {
		t.Fatalf("a proved person's exact start was held: exit %d %s", code, text)
	}
	record := landingLaunches(t, store)[0]
	if _, err := store.Update(record.ID, func(r *launch.Record) error {
		r.State, r.FinishedAt = launch.Failed, laneTestNow.Format(time.RFC3339)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	dir, err := store.StateDir(record.ID)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "result.json"), []byte(`{"is_error":true,"result":"HTTP 429 Too Many Requests"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	b.owners.prove = nil
	selected, err := plain.ReadBatch(b.installation)
	if err != nil || selected == nil {
		t.Fatalf("recorded person selection: %v", err)
	}
	if code, text := b.run(t, "landing", "run", "--batch", selected.ID, "--json"); code != 0 || !strings.Contains(text, "selection recorded, execution not started") || !strings.Contains(text, "provider") || len(landingLaunches(t, store)) != 1 {
		t.Fatalf("a later agent inherited the person's provider bypass: exit %d %s", code, text)
	}
	// A second exact person act starts and ends another launch before the hint breaks.
	b.owners.prove = enrolledPersonProver(t, b.installation, laneTestNow)
	if code, text := b.run(t, "landing", "run", "--goals", "one", "--by", "Wido", "--json"); code != 0 || len(landingLaunches(t, store)) != 2 {
		t.Fatalf("second person start: %d %s", code, text)
	}
	ended := landingLaunches(t, store)[1]
	if _, err := store.Update(ended.ID, func(r *launch.Record) error {
		r.State, r.FinishedAt = launch.Failed, laneTestNow.Format(time.RFC3339Nano)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	dir, err = store.StateDir(ended.ID)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "result.json"), []byte(`{"is_error":true,"result":"HTTP 429 Too Many Requests"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	// An unreadable provider hint must not prevent reaping and an explicit person start.
	owner, _, err := lane.Read(b.home)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(b.installation, "artifacts", "agents", fmt.Sprintf("providers-%d.json", owner.CustodyEpoch))
	if err := os.WriteFile(path, []byte("{"), 0o600); err != nil {
		t.Fatal(err)
	}
	b.owners.prove = enrolledPersonProver(t, b.installation, laneTestNow)
	if code, text := b.run(t, "landing", "run", "--goals", "one", "--by", "Wido", "--json"); code != 0 || len(landingLaunches(t, store)) != 3 {
		t.Fatalf("unreadable provider state held a person's start: exit %d %s", code, text)
	}
	state, err := lane.ReadAgentState(b.home)
	if err != nil || state.Launch == ended.ID {
		t.Fatalf("ended launch was not reaped and replaced: %+v %v", state, err)
	}
}
