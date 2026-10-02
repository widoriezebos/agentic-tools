package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/channel"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/launch"
)

// TestLandingAgentQuestionHoldsTheRelaunch: a landing agent that asked a
// person about the lane holds the keeper's next start while its question is
// open on this machine; a lane question from another machine, or a question
// about anything else, holds nothing; once the question is answered or
// withdrawn the start proceeds.
func TestLandingAgentQuestionHoldsTheRelaunch(t *testing.T) {
	t.Parallel()
	base := t.TempDir()
	home, checkout := filepath.Join(base, "home"), filepath.Join(base, "landing")
	module := filepath.Join(checkout, "metasystem")
	for _, dir := range []string{home, module} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	files := map[string]string{
		filepath.Join(module, "go.mod"):                "module fixture\n",
		filepath.Join(module, "metasystem.conf"):       "# overrides only\n",
		filepath.Join(module, "metasystem.conf.local"): "launch.landing.runtime=claude\nlaunch.landing.model=claude-roster-model\n",
	}
	for path, content := range files {
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	registerLane(t, home, checkout, "a-person", time.Date(2026, 9, 30, 12, 0, 0, 0, time.UTC))
	module = resolvedPath(module)
	now := time.Date(2026, 9, 30, 13, 0, 0, 0, time.UTC)
	store := launch.Store{Root: filepath.Join(base, "launches")}
	manager := &launch.Manager{Store: store, Adapters: map[string]launch.Adapter{"claude-headless": launch.ClaudeHeadless{Binary: "/fixture/bin/claude", ProjectsRoot: filepath.Join(base, "projects")}},
		Supervisor: recordingSupervisor{store}, Now: func() time.Time { return now }, Sleep: func(time.Duration) {}, Poll: time.Second, StartCap: time.Minute,
		Lane: landingLaneCheckout(func() (string, error) { return home, nil })}
	agent := landingAgent{manager: func() *launch.Manager { return manager }, settings: installationSettings, now: func() time.Time { return now },
		nonce: func() (string, error) { return "0011223344556677", nil }, machine: func(string) (string, error) { return "m-lane", nil }}
	keeper := newLandingAgentKeeper(module, home, agent)
	keeper.Sources.Reasons = func(string) ([]string, error) { return []string{"queued"}, nil }
	ask := func(about, machine string) channel.Question {
		t.Helper()
		q, err := channel.Ask(channel.AskRequest{RepoRoot: module, About: about, Kind: "other", Machine: machine, Lineage: launch.LandingOwnerLineage,
			Facts: []string{"Return the branch that conflicts?", "merge conflict in " + about + " on " + machine}, Now: now})
		if err != nil {
			t.Fatal(err)
		}
		return q
	}
	launches := func() int {
		records, err := store.List()
		if err != nil {
			t.Fatal(err)
		}
		return len(records)
	}

	ask("lane", "m-other")
	ask("machine", "m-lane")
	lane := ask("lane", "m-lane")
	line := keeper.Step()
	if launches() != 0 || !strings.Contains(line, lane.ID) {
		t.Fatalf("an open lane question on this machine holds the start: line %q, %d launches", line, launches())
	}
	if _, err := channel.Withdraw(module, lane.ID, "the conflict was resolved by hand", nil, channel.DestinationConfig{}); err != nil {
		t.Fatal(err)
	}
	if line := keeper.Step(); launches() != 1 {
		t.Fatalf("withdrawn, the start proceeds: line %q, %d launches", line, launches())
	}
}
