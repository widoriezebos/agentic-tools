package main

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/adopt"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/board"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/channel"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/lease"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/realpath"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/stateroot"
)

// The layout goldens of group G5 (plans/designs/output-style.md §7): goal
// sync, the design and decision records, settings coordinator, the agent
// messages, the channel and mission questions, system adopt and the app
// verbs. Each bed is built fresh for every run, so an act's golden is its
// first run.

func g5LayoutCases() []layoutCase {
	return []layoutCase{
		{name: "goal-sync", args: []string{"goal", "sync"}, bed: goalSyncLayoutBed(true)},
		{name: "goal-sync-clean", args: []string{"goal", "sync"}, bed: goalSyncLayoutBed(false)},
		{name: "goal-sync-refusal", args: []string{"goal", "sync", "--upgrade", "--by", "Wido", "--sync-mode", "sideways"}, bed: outsideLayoutBed},
		{name: "design-list", args: []string{"design", "list"}, bed: recordsLayoutBed},
		{name: "design-show", args: []string{"design", "show", bedGoal}, bed: recordsLayoutBed},
		{name: "design-show-refusal", args: []string{"design", "show"}, bed: recordsLayoutBed},
		{name: "decision-list", args: []string{"decision", "list"}, bed: recordsLayoutBed},
		{name: "decision-show", args: []string{"decision", "show", layoutDecisionID}, bed: recordsLayoutBed},
		{name: "decision-show-refusal", args: []string{"decision", "show", "NOSUCH"}, bed: recordsLayoutBed},
		{name: "settings-coordinator", args: []string{"settings", "coordinator"}, bed: recordsLayoutBed},
		{name: "settings-coordinator-refusal", args: []string{"settings", "coordinator", "--by", "Wido"}, bed: recordsLayoutBed},
		{name: "agent-ask", args: []string{"agent", "ask", "m1b", "--text", "Is batch 19 still proving?", "--deadline", "30m", "--if-silent", "I land alone at 11:30"}, bed: agentLayoutBed(false)},
		{name: "agent-ask-refusal", args: []string{"agent", "ask", "m1z", "--text", "Anyone there?"}, bed: agentLayoutBed(false)},
		{name: "agent-reply", args: []string{"agent", "reply", "q-batch-19", "--text", "Yes, 120 of 189 sections done."}, bed: agentLayoutBed(true)},
		{name: "agent-inbox", args: []string{"agent", "inbox"}, bed: agentLayoutBed(true)},
		{name: "agent-inbox-empty", args: []string{"agent", "inbox"}, bed: agentLayoutBed(false)},
		{name: "question-show", args: []string{"question", "show", "q1"}, bed: questionLayoutBed},
		{name: "question-show-refusal", args: []string{"question", "show", "nosuch"}, bed: questionLayoutBed},
		{name: "question-retry", args: []string{"question", "retry", "q1"}, bed: questionLayoutBed},
		{name: "question-withdraw", args: []string{"question", "withdraw", "q2", "--reason", "decided in the review"}, bed: questionLayoutBed},
		{name: "question-wait", args: []string{"question", "wait", "demo/done-ask"}, bed: questionLayoutBed},
		{name: "system-adopt", args: []string{"system", "adopt", "../my-app"}, bed: adoptLayoutBed},
		{name: "system-adopt-refusal", args: []string{"system", "adopt"}, bed: adoptLayoutBed},
		{name: "app-status", args: []string{"app", "status"}, bed: appLayoutBed},
		{name: "app-stop", args: []string{"app", "stop"}, bed: appLayoutBed},
		{name: "app-log", args: []string{"app", "log", "--lines", "3"}, bed: appLayoutBed},
		{name: "app-check-refusal", args: []string{"app", "check"}, bed: appLayoutBed},
		{name: "app-start-refusal", args: []string{"app", "start", "--goal", "nosuch"}, bed: appLayoutBed},
		{name: "app-restart-refusal", args: []string{"app", "restart", "--wait-seconds", "soon"}, bed: appLayoutBed},
		{name: "app-reset-refusal", args: []string{"app", "reset", "--at", "main", "--goal", "nosuch"}, bed: appLayoutBed},
	}
}

// goalSyncLayoutBed is a synced goal ledger in a real Git checkout; edited
// hand-edits one goal's page and adds a stray file beside it.
func goalSyncLayoutBed(edited bool) func(t *testing.T) layoutBed {
	return func(t *testing.T) layoutBed {
		root, _, _ := goalBranchMainCLIFixtureBelow(t, "m1", ".")
		base, err := goal.BaseTip(root)
		if err != nil {
			t.Fatal(err)
		}
		if edited {
			page := filepath.Join(root, "plans", "goals", "standing-validation.md")
			data, err := os.ReadFile(page)
			if err != nil {
				t.Fatal(err)
			}
			file, problems := goal.ParseFile(data)
			if len(problems) != 0 {
				t.Fatalf("parse goal page: %v", problems)
			}
			file.NextStep = "Hand-edited next step, accepted by a person."
			writeTestingFixtureFile(t, page, goal.RenderFile(file), 0o644)
		}
		owners := defaultIntentOwners()
		owners.resolver = stateroot.NewResolver(fakeTop(root), noExecutable)
		resolved := realpath.Resolve(root)
		return layoutBed{owners: owners, cwd: root, replace: append([]string{base, "5e1f0c2a9d3b7e6f4a8c1d0b2e9f7a6c5d4b3a21", shortSHA(base), "5e1f0c2a9"},
			layoutPaths(root, resolved, "/Users/wido/GitHub/agentic-tools-m1e")...)}
	}
}

// layoutDecisionID is the decision record of the records bed.
const layoutDecisionID = "01M3MKDZKHW0X90ERJ2M3YS36V"

// recordsLayoutBed is the intent bed's goal ledger with two design records,
// one naming the bed's goal, and one decision record.
func recordsLayoutBed(t *testing.T) layoutBed {
	b := newIntentBed(t, false, nil)
	records := map[string]string{
		"plans/designs/agent-help.md":      "# Help an agent choose and complete the right task\n\n- Kind: design\n- Id: 01M3FDYDMV0G5V9YC1M9XAKGZR\n- Status: done\n- Goals: " + bedGoal + "\n\nThe design.\n",
		"plans/designs/output-style.md":    "# Output style: one shape for every metasystem command\n\n- Kind: design\n- Id: 01M3QQ5T6Y7Z8A9B0C1D2E3F4G\n- Status: accepted\n\nThe layout.\n",
		"docs/decisions/checkout-lease.md": "# Checkout lease: decisions\n\n- Kind: decision\n- Id: " + layoutDecisionID + "\n- Status: accepted\n\nOne writer per checkout.\n",
	}
	for path, body := range records {
		writeTestingFixtureFile(t, filepath.Join(b.root(), filepath.FromSlash(path)), []byte(body), 0o644)
	}
	root := realpath.Resolve(b.root())
	return layoutBed{owners: b.owners(), cwd: b.root(), replace: layoutPaths(b.root(), root, "/Users/wido/GitHub/agentic-tools-m1e")}
}

// agentLayoutBed is seat m1e of a host with three armed seats; asked has
// m1b ask m1e one question with the id q-batch-19. The bed is at a person's
// terminal, so an inbox marks nothing read.
func agentLayoutBed(asked bool) func(t *testing.T) layoutBed {
	return func(t *testing.T) layoutBed {
		b := newAgentBed(t, "m1e")
		b.seats = []string{"m1b", "m1c", "m1e"}
		b.now = layoutNow.Add(-2 * time.Minute)
		b.caller = lease.ClassHuman
		if asked {
			from := b.as("m1b")
			from.caller = lease.ClassMain
			if code, _, stderr := from.run("agent", "ask", "m1e", "--id", "q-batch-19", "--text", "Is batch 19 still proving?",
				"--deadline", "30m", "--if-silent", "I land alone at 11:30"); code != 0 {
				t.Fatalf("the bed's ask: %d %s", code, stderr)
			}
		}
		b.now = layoutNow
		home, root := realpath.Resolve(b.home), realpath.Resolve(b.root)
		return layoutBed{owners: b.owners(), cwd: b.root, replace: append(layoutPaths(b.root, root, "/Users/wido/GitHub/agentic-tools-m1e"),
			home, "/Users/wido/.metasystem", b.home, "/Users/wido/.metasystem")}
	}
}

// layoutProvider is a channel that takes every post.
type layoutProvider struct{ questionProvider }

// questionLayoutBed holds two open channel questions, q1 never delivered and
// q2 delivered, and mission demo's answered question done-ask.
func questionLayoutBed(t *testing.T) layoutBed {
	b := newProcessBed(t)
	provider := &layoutProvider{}
	owners := b.owners()
	owners.processes.question = channel.ReadQuestion
	owners.processes.channelLink = func(string) (channel.Provider, channel.DestinationConfig) {
		return provider, channel.DestinationConfig{}
	}
	root := b.root()
	questions := filepath.Join(root, "artifacts", "agents", "channel", "questions")
	writeQuestionFixture(t, filepath.Join(questions, "q1.json"), map[string]any{"id": "q1", "goal": bedGoal, "kind": "other", "state": "open",
		"facts": []string{"Land it?"}, "options": []any{map[string]any{"label": "yes", "consequence": "land"}}, "undelivered": 2})
	delivered := map[string]any{"id": "q2", "goal": bedGoal, "kind": "other", "state": "open",
		"facts": []string{"Split the batch?"}, "options": []any{map[string]any{"label": "yes", "consequence": "split"}}}
	writeQuestionFixture(t, filepath.Join(questions, "q2.json"), delivered)
	if _, _, err := channel.RetryDelivery(context.Background(), root, "q2", provider, channel.DestinationConfig{}); err != nil {
		t.Fatal(err)
	}
	writeQuestionFixture(t, filepath.Join(root, "artifacts", "agents", "missions", "demo", "asks", "done-ask.json"),
		map[string]any{"askId": "done-ask", "answeredAt": "2026-09-30T08:40:00Z", "answer": "yes"})
	resolved := realpath.Resolve(root)
	return layoutBed{owners: owners, cwd: root, replace: layoutPaths(root, resolved, "/Users/wido/GitHub/agentic-tools-m1e")}
}

// adoptLayoutBed adopts through an owner that writes nothing and reports a
// fresh adoption with one note and two registrations.
func adoptLayoutBed(t *testing.T) layoutBed {
	b := newIntentBed(t, false, nil)
	owners := b.owners()
	owners.adopt = func(options adopt.Options) (adopt.Result, error) {
		return adopt.Result{SHA: "ab0bc127117582063d021e2794c9a43a76ddf130", Target: options.Target,
			Notes:     []string{"the target had no README; none was written"},
			Installed: []string{filepath.Join(options.Target, ".claude", "skills", "verify"), filepath.Join(options.Target, ".claude", "skills", "retro")}}, nil
	}
	root := realpath.Resolve(b.root())
	app := filepath.Join(filepath.Dir(b.root()), "my-app")
	return layoutBed{owners: owners, cwd: b.root(), replace: append([]string{app, "/Users/wido/GitHub/my-app", filepath.Join(filepath.Dir(root), "my-app"), "/Users/wido/GitHub/my-app"},
		layoutPaths(b.root(), root, "/Users/wido/GitHub/agentic-tools-m1e")...)}
}

// appLayoutBed is a project with a launch contract whose application has
// never run, and a log of three lines from an earlier run.
func appLayoutBed(t *testing.T) layoutBed {
	root := realpath.Resolve(t.TempDir())
	installation := filepath.Join(root, "metasystem")
	contract, err := json.Marshal(map[string]any{"schemaVersion": 1, "name": "fixture", "start": map[string]any{"argv": []string{"/bin/sh", "-c", "sleep 60"}},
		"log": "app.log"})
	if err != nil {
		t.Fatal(err)
	}
	for path, body := range map[string]string{
		"metasystem/metasystem.conf":       "metasystem.version=1\nmetasystem.template=true\ntesting.contract=testing.json\nlaunch.contract=launch.json\n",
		"metasystem/launch.json":           string(contract),
		"development/metasystem-design.md": "fixture\n",
		".gitignore":                       "metasystem/artifacts/\napp.log\n",
		"app.log":                          "fixture: listening on 127.0.0.1:4100\nfixture: ready\nfixture: served /-/health\n",
	} {
		writeTestingFixtureFile(t, filepath.Join(root, filepath.FromSlash(path)), []byte(body), 0o644)
	}
	for _, args := range [][]string{{"init", "--quiet", "--initial-branch=main"}, {"config", "user.email", "fixture@invalid"},
		{"config", "user.name", "Fixture"}, {"add", "-A"}, {"commit", "--quiet", "-m", "the fixture project"}} {
		command := exec.Command("git", args...)
		command.Dir = root
		if out, err := command.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	_ = installation
	return layoutBed{owners: defaultIntentOwners(), cwd: root, replace: layoutPaths(root, root, "/Users/wido/GitHub/agentic-tools-m1e")}
}

var _ = board.KindAsk
