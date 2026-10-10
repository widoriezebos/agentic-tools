package main

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/channel"
	channelphase "github.com/widoriezebos/agentic-tools/metasystem/internal/channel/phase"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
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
	agent := newTestLandingAgent(func(agent *landingAgent) {
		agent.manager = func() *launch.Manager { return manager }
		agent.now = func() time.Time { return now }
		agent.nonce = func() (string, error) { return "0011223344556677", nil }
		agent.machine = func(string) (string, error) { return "m-lane", nil }
	})
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

// TestLaneQuestionAnswerIsPolledByTheLanesSteward: the steward that keeps
// the lane runs at the lane installation, where the landing agent's lane
// question is recorded, so that steward's own channel duty polls the
// question store the hold reads: the person's TOTP-valid reply through the
// fake provider is recorded there, the hold lifts and the landing agent
// starts again.
func TestLaneQuestionAnswerIsPolledByTheLanesSteward(t *testing.T) {
	t.Parallel()
	base := t.TempDir()
	home, checkout := filepath.Join(base, "home"), filepath.Join(base, "landing")
	module := filepath.Join(checkout, "metasystem")
	for _, dir := range []string{home, module} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	for _, args := range [][]string{{"init", "-q"}, {"config", "metasystem.goal.machine", "m-lane"}} {
		if out, err := exec.Command("git", append([]string{"-C", module}, args...)...).CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v %s", args, err, out)
		}
	}
	now := time.Now().UTC()
	providerDir, _ := commandFakeBedWithClock(t, func() time.Time { return now })
	files := map[string]string{
		filepath.Join(module, "go.mod"):                "module fixture\n",
		filepath.Join(module, "metasystem.conf"):       "channel.destination.fleet.adapter=fake\nchannel.destination.fleet.fake.dir=" + providerDir + "\nchannel.human.slack.user-id=human-a\n",
		filepath.Join(module, "metasystem.conf.local"): "launch.landing.runtime=claude\nlaunch.landing.model=claude-roster-model\nchannel.human.totp-secret=" + blockedQuestionSecret + "\n",
	}
	for path, content := range files {
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	// The lane's goal ledger on its origin: the steward's poll records each
	// reply in the ledger inbox, as on every installation (Decision 8).
	writeTestingFixtureFile(t, filepath.Join(module, "plans", "goals", "backlog.md"), goal.RenderRoot(&goal.RootRecord{Identity: "01ARZ3NDEKTSV4RRFFQ69G5FAV", FormatVersion: "1", SyncMode: goal.SyncRemote, Revision: 1}), 0o644)
	goalSyncMutationGit(t, module, "add", "plans/goals/backlog.md")
	goalSyncMutationGit(t, module, "-c", "user.name=fixture", "-c", "user.email=fixture@example.invalid", "commit", "-qm", "lane ledger fixture")
	goalSyncMutationGit(t, module, "update-ref", goal.LocalLedgerBranch, "HEAD")
	goalSyncMutationGit(t, module, "update-ref", goal.AcceptedRef, "HEAD")
	origin := filepath.Join(base, "origin.git")
	goalSyncMutationGit(t, base, "init", "-q", "--bare", origin)
	goalSyncMutationGit(t, module, "remote", "add", "origin", origin)
	goalSyncMutationGit(t, module, "push", "-q", "origin", "HEAD:main")
	registerLane(t, home, checkout, "a-person", time.Date(2026, 9, 30, 12, 0, 0, 0, time.UTC))
	module = resolvedPath(module)
	store := launch.Store{Root: filepath.Join(base, "launches")}
	manager := &launch.Manager{Store: store, Adapters: map[string]launch.Adapter{"claude-headless": launch.ClaudeHeadless{Binary: "/fixture/bin/claude", ProjectsRoot: filepath.Join(base, "projects")}},
		Supervisor: recordingSupervisor{store}, Now: func() time.Time { return now }, Sleep: func(time.Duration) {}, Poll: time.Second, StartCap: time.Minute,
		Lane: landingLaneCheckout(func() (string, error) { return home, nil })}
	agent := newTestLandingAgent(func(agent *landingAgent) {
		agent.manager = func() *launch.Manager { return manager }
		agent.now = func() time.Time { return now }
		agent.nonce = func() (string, error) { return "0011223344556677", nil }
		agent.machine = goal.ResolveMachine
	})
	keeper := newLandingAgentKeeper(module, home, agent)
	keeper.Sources.Reasons = func(string) ([]string, error) { return []string{"queued"}, nil }
	// The steward's tick: the keeper step, then the channel duty on its root.
	tick := func() string {
		t.Helper()
		line := keeper.Step()
		if _, err := channelphase.Run(context.Background(), module); err != nil {
			t.Fatalf("the lane steward's channel duty: %v", err)
		}
		return line
	}
	q, err := channel.Ask(channel.AskRequest{RepoRoot: module, About: "lane", Kind: "other", Machine: "m-lane", Lineage: launch.LandingOwnerLineage,
		Facts: []string{"Return the branch that conflicts?", "merge conflict in internal/goal"}, Now: now})
	if err != nil {
		t.Fatal(err)
	}
	launches := func() int {
		records, err := store.List()
		if err != nil {
			t.Fatal(err)
		}
		return len(records)
	}
	if line := tick(); launches() != 0 || !strings.Contains(line, q.ID) {
		t.Fatalf("the open lane question holds the start: line %q, %d launches", line, launches())
	}
	posted, err := channel.ReadQuestion(module, q.ID)
	if err != nil || posted.Thread == nil {
		t.Fatalf("the lane's steward delivered the lane question: %+v %v", posted, err)
	}
	replyInThread(t, providerDir, posted, "return it", now)
	tick()
	if answered, err := channel.ReadQuestion(module, q.ID); err != nil || answered.Answer == nil || answered.Answer.Text != "return it" {
		t.Fatalf("the answer is recorded in the lane's question store: %+v %v", answered, err)
	}
	if line := keeper.Step(); launches() != 1 {
		t.Fatalf("answered through the channel, the hold lifts: line %q, %d launches", line, launches())
	}
}
