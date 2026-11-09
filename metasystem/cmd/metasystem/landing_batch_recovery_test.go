package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/channel"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/lane"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/plain"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/launch"
)

func TestTheBatchStepsBeforeRecovery(t *testing.T) {
	t.Parallel()
	for _, hold := range []string{"none", "barren", "question"} {
		t.Run(hold, func(t *testing.T) {
			t.Parallel()
			b := newSelectionBed(t)
			helmMust(t, os.Remove(filepath.Join(plain.Dir(b.lane), "results.jsonl")))
			write := func(name string, value any) {
				t.Helper()
				data, err := json.Marshal(value)
				helmMust(t, err)
				helmMust(t, os.WriteFile(filepath.Join(plain.Dir(b.lane), name), append(data, '\n'), 0600))
			}
			batch := plain.Batch{ID: "batch", Lane: b.record, State: plain.BatchRunning, Base: "main", Members: []plain.GoalSHA{{Goal: "a", SHA: "sha-a"}, {Goal: "b", SHA: "sha-b"}}}
			write("batch.json", batch)
			head, main, merged := "merge-a", "main", false
			notAncestor := replayFalseState(t)
			alive, launches := false, 0
			seams := b.owners.landing.plainProve
			seams.Policy = func(string) (plain.PolicyValue, error) { return plain.PolicyValue{Value: "auto"}, nil }
			git := seams.Git
			seams.Git = func(dir string, args ...string) (string, error) {
				switch strings.Join(args, " ") {
				case "rev-parse --verify HEAD^{commit}":
					return head, nil
				case "rev-parse --verify HEAD^{tree}":
					return head + "-tree", nil
				case "rev-parse --verify origin/main^{commit}":
					return main, nil
				}
				if args[0] == "merge-base" {
					if args[2] == "sha-b" && !merged {
						return "", &exec.ExitError{ProcessState: notAncestor}
					}
					return "", nil
				}
				if args[0] == "rev-parse" && args[len(args)-1] == main+"^{tree}" {
					return main + "-tree", nil
				}
				if args[0] == "show" && args[1] == main+":metasystem.conf" {
					return "proof.full=fixture-proof\n", nil
				}
				return git(dir, args...)
			}
			seams.Alive = func(plain.Running) bool { return alive }
			seams.Launch = func(argv []string, _, _ string) (int64, error) {
				if !strings.Contains(strings.Join(argv, " "), "--trunk") {
					t.Fatalf("unexpected recovery launch: %v", argv)
				}
				launches++
				alive = true
				return int64(os.Getpid()), nil
			}
			keeper := newLandingAgentKeeper(b.lane, b.home, newTestLandingAgent(func(agent *landingAgent) {
				agent.proofEffects, agent.now = seams, func() time.Time { return b.now }
				agent.machine = func(string) (string, error) { return "fixture", nil }
				agent.settings = func(string) (launch.Settings, error) { return launch.DefaultSettings(), nil }
			}))
			observe := func(want int) {
				t.Helper()
				helmMust(t, keeper.Observe(b.record))
				if launches != want {
					t.Fatalf("recovery launches=%d want=%d", launches, want)
				}
				b.now = b.now.Add(time.Minute)
			}
			gate := plain.Result{BatchID: batch.ID, BatchMembers: batch.Members, Commit: head, Tree: head + "-tree", Result: plain.Skipped, Static: plain.Green, Attempt: "gate", At: b.now.Format(time.RFC3339)}
			write("gates.jsonl", gate)
			// A pending member gives the batch a merge step before any proof.
			observe(0)
			merged = true
			head, main = "candidate", "records-main"
			gate.Commit, gate.Tree = head, head+"-tree"
			write("gates.jsonl", gate)
			if hold == "barren" {
				data, err := json.Marshal(lane.AgentState{Barren: 2})
				helmMust(t, err)
				helmMust(t, os.WriteFile(lane.AgentStatePath(b.home), data, 0600))
			} else if hold == "question" {
				_, err := channel.Ask(channel.AskRequest{RepoRoot: b.lane, About: "lane", Kind: "other", Machine: "fixture", Lineage: lane.AgentLineage, Now: b.now, Facts: []string{"May the lane continue?"}})
				helmMust(t, err)
			}
			if hold != "none" {
				observe(1)
				return
			}
			observe(0)
			write("running.json", plain.Running{BatchID: batch.ID, BatchMembers: batch.Members, Commit: head, Tree: head + "-tree", Attempt: "candidate-proof", Pid: int64(os.Getpid())})
			alive = true
			observe(0)
			alive = false
			helmMust(t, os.Remove(filepath.Join(plain.Dir(b.lane), "running.json")))
			candidate := gate
			candidate.Result, candidate.Attempt, candidate.Scope = plain.Green, "candidate-proof", "full"
			write("results.jsonl", candidate)
			observe(0)
			write("pushes.jsonl", plain.Pushed{BatchID: batch.ID, Commit: head, Tree: candidate.Tree, At: b.now.Format(time.RFC3339)})
			observe(1)
		})
	}
}
