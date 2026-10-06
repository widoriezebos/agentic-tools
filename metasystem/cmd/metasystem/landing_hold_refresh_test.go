package main

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/lane"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/plain"
)

func holdRefreshKeeper(b *resolveVerbFixture) (*lane.AgentKeeper, *int) {
	starts := 0
	keeper := newLandingAgentKeeper(b.root, b.home, landingAgent{now: func() time.Time { return laneTestNow }})
	keeper.Sources = b.owners.landing.wake(b.home)
	keeper.Running = func() (string, bool, error) { return "", false, nil }
	keeper.Start = func(string, lane.Wake) (string, error) { starts++; return "hold-launch", nil }
	keeper.Observe, keeper.BarrenStop, keeper.Fingerprint, keeper.Waiting = nil, nil, nil, nil
	keeper.Holds, keeper.Reap = nil, nil
	b.owners.landing.keeper = func(string, string) lane.AgentKeeper { return keeper }
	return &keeper, &starts
}

func TestExceptionWorkLandAgainReleasesWaitingLine(t *testing.T) {
	t.Parallel()
	b, _, install := plainLaneBedWith(t, true, "critic-root", "critic-root")
	code, result := b.do("work", "land", bedGoal)
	expectOutcome(t, "hand-in", code, result, intentConfirmed)
	register := holdIncidentFixture(t, b.intentBed)
	args := []string{"work", "land", bedGoal, "--again", "--exception", goal.LandTrunkRedCode, "--reason", "Ship this line", "--by", "Wido"}
	code, result = b.do(args...)
	expectOutcome(t, "exception on waiting line", code, result, intentUnchanged)
	entries, err := plain.Entries(install)
	if err != nil || len(entries) != 1 || entries[0].Exception == nil || entries[0].Exception.By != "Wido" {
		t.Fatalf("retry lost the exception: %+v %v", entries, err)
	}
	if held := plain.HoldEntries(entries, register); held[0].Held {
		t.Fatalf("exception left the line held: %+v", held)
	}
	before, err := os.ReadFile(filepath.Join(plain.Dir(install), "queue.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	code, result = b.do(args...)
	expectOutcome(t, "repeat", code, result, intentUnchanged)
	after, err := os.ReadFile(filepath.Join(plain.Dir(install), "queue.jsonl"))
	if err != nil || string(before) != string(after) {
		t.Fatalf("repeat appended: %v", err)
	}
}

func TestWorkLandIncidentFixWakesWithoutPush(t *testing.T) {
	t.Parallel()
	b, state, _ := plainLaneBedWith(t, true, "critic-root", "critic-root")
	register := holdIncidentFixture(t, b.intentBed)
	l, _ := holdLaneFixture(t, register)
	git := l.owners.landing.plainProve.Git
	l.owners.landing.plainProve.Git = func(dir string, args ...string) (string, error) {
		if args[0] == "fetch" {
			t.Fatal("a fix hand-in should wake without fetching its claim")
		}
		return git(dir, args...)
	}
	b.owners.laneInstall = func(string) (string, error) { return l.install, nil }
	code, result := b.do("incident", "claim", holdIncidentID, "--goal", bedGoal, "--by", "Wido")
	expectOutcome(t, "claim", code, result, intentConfirmed)
	code, result = b.do("work", "land", bedGoal)
	expectOutcome(t, "fix hand-in", code, result, intentConfirmed)
	data, err := os.ReadFile(filepath.Join(plain.Dir(l.install), "queue.jsonl"))
	var line map[string]any
	if err != nil || json.Unmarshal(data, &line) != nil || line["fix"] != holdIncidentID {
		t.Fatalf("fix incident missing from hand-in: %s %v", data, err)
	}
	_, starts := holdRefreshKeeper(l)
	code, words := l.run(t, l.root, "run", "--json")
	if code != 0 || *starts != 1 || len(state.pushes) != 0 {
		t.Fatalf("fix hand-in failed to wake without a push: %d starts=%d pushes=%v %s", code, *starts, state.pushes, words)
	}
	code, words = l.run(t, l.root, "status", "--json")
	var status struct{ Data plain.Status }
	if code != 0 || json.Unmarshal([]byte(words), &status) != nil || len(status.Data.Queue) != 1 || status.Data.Queue[0].Held || !slices.Contains(status.Data.Wake.Reasons, plain.WakeQueued) {
		t.Fatalf("fix remains held in status: %d %s", code, words)
	}
}

func TestLandingKeeperRefreshesIncidentClosure(t *testing.T) {
	t.Parallel()
	for _, mode := range []string{"closed", "fetch fails", "still open", "conflict"} {
		t.Run(mode, func(t *testing.T) {
			t.Parallel()
			b, _, _ := plainLaneBedWith(t, true, "critic-root", "critic-root")
			l, _ := holdLaneFixture(t, nil)
			b.owners.laneInstall = func(string) (string, error) { return l.install, nil }
			code, result := b.do("work", "land", bedGoal)
			expectOutcome(t, "hand-in", code, result, intentConfirmed)
			register := holdIncidentFixture(t, b.intentBed)
			remote := register
			if mode != "still open" {
				code, result = b.do("incident", "close", holdIncidentID, "--reason", "Main is repaired", "--by", "Wido")
				expectOutcome(t, "close on origin", code, result, intentConfirmed)
				var problems []goal.Problem
				remote, problems = goal.ParseTrunkRed(b.repo.commit(b.repo.accepted).files["plans/goals/trunk-red.json"])
				if len(problems) != 0 || remote[0].Closed == nil {
					t.Fatalf("closure: %+v %v", remote, problems)
				}
			}
			cachedMain, fetches := "main", 0
			git := l.owners.landing.plainProve.Git
			l.owners.landing.plainProve.Git = func(dir string, args ...string) (string, error) {
				path := "metasystem/plans/goals/trunk-red.json"
				if args[0] == "rev-parse" {
					return cachedMain, nil
				}
				if args[0] == "ls-tree" {
					return path, nil
				}
				if args[0] == "show" {
					switch args[1] {
					case "main:" + path:
						return string(goal.RenderTrunkRed(register)), nil
					case "remote-main:" + path:
						return string(goal.RenderTrunkRed(remote)), nil
					default:
						t.Fatalf("register read at unexpected ref: %v", args)
					}
				}
				if args[0] == "fetch" {
					fetches++
					if strings.Join(args, " ") != "fetch --quiet origin +refs/heads/main:refs/remotes/origin/main" {
						t.Fatalf("fetch: %v", args)
					}
					if mode == "fetch fails" {
						return "", errors.New("origin unavailable")
					}
					cachedMain = "remote-main"
					return "", nil
				}
				return git(dir, args...)
			}
			if mode == "conflict" {
				entries, err := plain.Entries(l.install)
				if err != nil {
					t.Fatal(err)
				}
				writeHoldLines(t, l.install, plain.Line{Goal: bedGoal, SHA: entries[0].SHA}, plain.Line{Goal: "before", SHA: "before"}, plain.Line{Goal: bedGoal, SHA: entries[0].SHA, Outcome: plain.StateWaiting, Held: true, After: []plain.GoalSHA{{Goal: "before", SHA: "before"}}, Reason: "waits for before"})
			}
			keeper, starts := holdRefreshKeeper(l)
			run := keeper.Run()
			wantFetch, wantStarts := 1, 0
			if mode == "closed" {
				wantStarts = 1
			}
			if mode == "conflict" {
				wantFetch = 0
			}
			if fetches != wantFetch || *starts != wantStarts || (run.Outcome == lane.AgentStarted) != (wantStarts == 1) {
				t.Fatalf("next tick: %+v fetches=%d starts=%d", run, fetches, *starts)
			}
			code, words := l.run(t, l.root, "status", "--json")
			var status struct{ Data plain.Status }
			if code != 0 || json.Unmarshal([]byte(words), &status) != nil || len(status.Data.Queue) == 0 || status.Data.Queue[0].Held != (mode != "closed") {
				t.Fatalf("closure status: %d %s", code, words)
			}
			if mode == "fetch fails" && !strings.Contains(strings.Join(status.Data.Problems, " "), "origin unavailable") {
				t.Fatalf("fetch failure hidden: %s", words)
			}
			if fetches-wantFetch > 1 {
				t.Fatalf("status fetched more than once: %d", fetches-wantFetch)
			}
		})
	}
}
