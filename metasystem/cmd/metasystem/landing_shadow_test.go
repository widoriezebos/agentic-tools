package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/lane"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/plain"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/state"
)

// The built CLI must locate the fixture's host registration and nested installation.
func TestLandingShadowCLIRefreshAndJSON(t *testing.T) {
	t.Parallel()
	b := newPlainVerbBed(t)
	engine := os.Getenv("METASYSTEM_WAIT_BINARY")
	if engine == "" {
		t.Fatal("the package did not build the CLI")
	}
	if _, err := lane.SetPauseBecause(b.home, "fixture", "maintenance", laneTestNow); err != nil {
		t.Fatal(err)
	}
	call := func(action string) json.RawMessage {
		t.Helper()
		cmd := exec.CommandContext(t.Context(), engine, "landing", action, "--json")
		cmd.Dir = b.checkout
		cmd.Env = fixtureCommandEnvironment(t, "METASYSTEM_SUPERVISION_REGISTRY_HOME="+b.registry)
		output, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("landing %s: %v %s", action, err, output)
		}
		var result struct{ Data json.RawMessage }
		if err := json.Unmarshal(output, &result); err != nil {
			t.Fatal(err)
		}
		return result.Data
	}
	call("status")
	record, exists, err := state.Read(plain.Dir(b.installation))
	if err != nil || !exists || record.Lane.Word != "paused" || record.Lane.By != "fixture" || !record.Lane.Since.At.Equal(laneTestNow) || record.Lane.Since.Observed {
		t.Fatalf("status exit did not refresh: %+v %v %v", record.Lane, exists, err)
	}
	var health struct {
		State  string        `json:"state"`
		Shadow *state.Record `json:"shadow"`
	}
	if err := json.Unmarshal(call("health"), &health); err != nil {
		t.Fatal(err)
	}
	if health.State == "" || health.Shadow == nil || health.Shadow.Lane.Word != "paused" {
		t.Fatalf("health changed its state shape or omitted shadow: %+v", health)
	}
	var status plain.Status
	if err := json.Unmarshal(call("status"), &status); err != nil {
		t.Fatal(err)
	}
	if !status.Paused || status.Shadow == nil || status.Shadow.Lane.Word != "paused" || status.Health == nil || status.Health.Shadow == nil {
		t.Fatalf("status shadow: %+v", status)
	}
	resumedAt := laneTestNow.Add(time.Minute)
	if _, err := lane.ClearPauseAt(b.home, "fixture", resumedAt); err != nil {
		t.Fatal(err)
	}
	call("health")
	record, _, err = state.Read(plain.Dir(b.installation))
	if err != nil || record.Lane.Word != "running" || record.Lane.Since.Observed || !record.Lane.Since.At.Equal(resumedAt) {
		t.Fatalf("health exit did not refresh resume: %+v %v", record.Lane, err)
	}
	data, err := os.ReadFile(filepath.Join(plain.Dir(b.installation), "events.jsonl"))
	if err != nil || strings.Count(string(data), "\n") != 3 {
		t.Fatalf("unchanged verbs appended rebuilds: %s %v", data, err)
	}
	if err := os.WriteFile(filepath.Join(plain.Dir(b.installation), "state.json"), []byte(`{not json`), 0600); err != nil {
		t.Fatal(err)
	}
	cmd := exec.CommandContext(t.Context(), engine, "landing", "health")
	cmd.Dir = b.checkout
	cmd.Env = fixtureCommandEnvironment(t, "METASYSTEM_SUPERVISION_REGISTRY_HOME="+b.registry)
	output, err := cmd.CombinedOutput()
	if err != nil || !strings.Contains(string(output), "the lane shadow cannot be read:") || !strings.Contains(string(output), "the next refresh replaces it") {
		t.Fatalf("CLI lost unreadable-shadow finding: %v %s", err, output)
	}
	record, exists, err = state.Read(plain.Dir(b.installation))
	if err != nil || !exists || record.Lane.Word != "running" {
		t.Fatalf("CLI exit did not repair shadow: %+v %v %v", record, exists, err)
	}
}

func TestLandingShadowKeeperProductionWiring(t *testing.T) {
	t.Parallel()
	b := newPlainVerbBed(t)
	agent := newLandingAgent()
	agent.now = func() time.Time { return laneTestNow }
	keeper := newLandingAgentKeeper(b.checkout, b.home, agent)
	if keeper.Refresh == nil {
		t.Fatal("keeper has no shadow refresh")
	}
	// A paused tick stops before preparation and launch but still refreshes.
	if _, err := lane.SetPause(b.home, "fixture", laneTestNow); err != nil {
		t.Fatal(err)
	}
	keeper.Prepare, keeper.Observe = nil, nil
	if run := keeper.Run(); run.Outcome != lane.AgentPaused {
		t.Fatalf("paused tick: %+v", run)
	}
	r, exists, err := state.Read(plain.Dir(b.installation))
	if err != nil || !exists || r.Lane.Word != "paused" {
		t.Fatalf("keeper wiring did not create state: %+v %v %v", r.Lane, exists, err)
	}
}
