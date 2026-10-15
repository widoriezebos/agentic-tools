package main

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/humanauthority"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/lane"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/plain"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/launch"
)

func TestLandingKeeperClockWiringAndVerboseReason(t *testing.T) {
	t.Parallel()
	for _, policy := range []string{"auto", "person", "paused during admission"} {
		t.Run(policy, func(t *testing.T) {
			t.Parallel()
			b := newSelectionBed(t)
			helmMust(t, os.Remove(filepath.Join(plain.Dir(b.lane), "results.jsonl")))
			seams := b.owners.landing.plainProve
			git := seams.Git
			seams.Git = func(dir string, args ...string) (string, error) {
				if args[len(args)-1] == "origin/main^{commit}" && policy == "paused during admission" {
					_, err := lane.SetPause(b.home, "Wido", b.now)
					helmMust(t, err)
				}
				if strings.Join(args, " ") == "show main:metasystem.conf" {
					return "proof.full=fixture-proof\n", nil
				}
				return git(dir, args...)
			}
			seams.Policy = func(key string) (plain.PolicyValue, error) {
				value := "auto"
				if key == "landing.batch" {
					value = policy
					if policy == "paused during admission" {
						value = "person"
					}
				}
				return plain.PolicyValue{Value: value, Source: "fixture"}, nil
			}
			seams.GoalTier = func(string, string) (uint8, error) { return 1, nil }
			alive, trunks, agents := false, 0, 0
			seams.Alive = func(plain.Running) bool { return alive }
			seams.Executable = func() (string, error) { return "fixture-engine", nil }
			seams.CommandForCommit = func(string) (string, error) { return "fixture-proof", nil }
			seams.Launch = func(args []string, _, _ string) (int64, error) {
				if !strings.Contains(strings.Join(args, " "), "--trunk") {
					t.Fatalf("unexpected proof: %v", args)
				}
				trunks++
				alive = true
				return int64(os.Getpid()), nil
			}
			keeper := newLandingAgentKeeper(b.lane, b.home, newTestLandingAgent(func(agent *landingAgent) {
				agent.proofEffects = seams
				agent.now = func() time.Time { return b.now }
				agent.settings = func(string) (launch.Settings, error) { return launch.DefaultSettings(), nil }
			}))
			keeper.Observe = nil
			keeper.Running = func() (string, bool, error) { return "", false, nil }
			keeper.Start = func(string, lane.Wake) (string, error) { agents++; return "agent", nil }
			run := keeper.Run()
			if policy == "paused during admission" {
				if trunks != 0 || agents != 0 || !strings.Contains(plain.ClockReason(b.lane), "clock check permission changed") {
					t.Fatalf("late pause did not fence the clock: %+v", run)
				}
			} else if policy == "person" {
				if trunks != 1 || agents != 0 || run.Outcome != lane.AgentHeld {
					t.Fatalf("person selection fallback: %+v trunks=%d agents=%d", run, trunks, agents)
				}
				keeper.Run()
				if trunks != 1 {
					t.Fatal("running trunk started again")
				}
			} else {
				batch, err := plain.ReadBatch(b.lane)
				if err != nil || batch == nil || !batch.ClockFull || trunks != 0 || agents != 1 {
					t.Fatalf("batch priority: %+v %v %+v", batch, err, run)
				}
			}
			reason := plain.ClockReason(b.lane)
			if reason == "" {
				t.Fatal("keeper did not retain its reason")
			}
			code, text := b.run(t, b.lane, "landing", "status", "--verbose")
			if code != 0 || !strings.Contains(text, reason) {
				t.Fatalf("verbose clock reason: %d %s want %q", code, text, reason)
			}
		})
	}
}

func TestLandingKeeperAgentTrunkAdmitsMainRedCheckpoint(t *testing.T) {
	t.Parallel()
	b := newSelectionBed(t)
	checkpoint := plain.Running{Checkpoint: true, Attempt: "main-red", BatchID: "candidate", Tree: "candidate-tree"}
	data, err := json.Marshal(checkpoint)
	helmMust(t, err)
	helmMust(t, os.WriteFile(filepath.Join(plain.Dir(b.lane), "running.json"), data, 0600))
	red, err := json.Marshal(plain.Result{Attempt: checkpoint.Attempt, BatchID: checkpoint.BatchID, Tree: checkpoint.Tree, Result: plain.Red, Cause: &plain.Cause{Kind: "main"}, At: b.now.Format(time.RFC3339)})
	helmMust(t, err)
	helmMust(t, os.WriteFile(filepath.Join(plain.Dir(b.lane), "results.jsonl"), append(red, '\n'), 0600))
	// Main has its own applicable result; this check exercises the agent's explicit request.
	current, err := json.Marshal(plain.Result{Trunk: true, Commit: "main", Tree: "main", Scope: "full", Result: plain.Green, At: b.now.Format(time.RFC3339), FullAt: b.now.Format(time.RFC3339), FullTree: "main", Environment: "fixture"})
	helmMust(t, err)
	helmMust(t, os.WriteFile(filepath.Join(plain.Dir(b.lane), "results.jsonl"), append(append(append(red, '\n'), current...), '\n'), 0600))
	b.owners.prove = func(string, int64, humanauthority.Reader, string, string, time.Time) (humanauthority.Proof, error) {
		return humanauthority.Proof{}, errors.New("agent caller")
	}
	seams := b.owners.landing.plainProve
	git := seams.Git
	seams.Git = func(dir string, args ...string) (string, error) {
		if args[0] == "diff" || args[0] == "merge-base" {
			return "", nil
		}
		if strings.Join(args, " ") == "show main:metasystem.conf" {
			return "proof.full=fixture-proof\n", nil
		}
		return git(dir, args...)
	}
	seams.Executable = func() (string, error) { return "fixture-engine", nil }
	seams.Alive = func(plain.Running) bool { return false }
	launches := 0
	seams.Launch = func(args []string, _, _ string) (int64, error) {
		if !strings.Contains(strings.Join(args, " "), "--trunk") {
			t.Fatalf("wrong proof: %v", args)
		}
		launches++
		return int64(os.Getpid()), nil
	}
	b.owners.landing.plainProve = seams
	b.keeper.Explicit = true
	b.keeper.Prepare = func(lane.Record) error { return nil }
	b.keeper.Start = func(string, lane.Wake) (string, error) {
		code, text := b.run(t, b.lane, "landing", "prove", "--trunk")
		if code != 0 {
			t.Fatalf("agent trunk command: exit %d: %s", code, text)
		}
		return "agent", nil
	}
	run := b.keeper.Run()
	running, recorded, _, err := plain.ReadRunning(b.lane, seams)
	if err != nil || !recorded || !running.Trunk || running.Person != nil || running.Admission == nil || running.Admission.Scope != "full" || running.Admission.Reason != "the tree comparison names no changed paths" || launches != 1 || run.Outcome != lane.AgentStarted {
		t.Fatalf("agent trunk proof: run=%+v proof=%+v launches=%d err=%v", run, running, launches, err)
	}
}
