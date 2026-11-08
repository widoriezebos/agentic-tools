package main

import (
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/plain"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/launch"
)

func scopedIncidentFixture(t *testing.T, b *intentBed) (laneVerbOwners, plain.Result, []plain.Result) {
	t.Helper()
	owners := incidentRecorderFixture(t, b, "finder")
	checks := []plain.Result{incidentCheckFixture(t, "TestStatic"), incidentCheckFixture(t, "TestCommand")}
	checks[0].Failed[0].Unit, checks[1].Failed[0].Unit = "fast-static-build", "cmd-metasystem"
	if err := owners.landingIncidentRecorder(b.root())(checks); err != nil {
		t.Fatal(err)
	}
	return owners, plain.Result{Trunk: true, Result: plain.Green, Scope: "scoped", Commit: b.repo.canonical, Tree: "proved-tree", Attempt: "scoped-green", At: syncRequestTestNow.Format(time.RFC3339), Environment: "fixture", Ran: []string{"fast-static-build"}}, checks
}

func TestIncidentClearReadsThroughLaneGit(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	reads := 0
	owners := laneVerbOwners{
		mainEndpoint: func(string) (goal.Endpoint, error) { return goal.Endpoint{Root: root}, nil },
		machine:      func(string) (string, error) { return "lane-machine", nil },
		now:          func() time.Time { return syncRequestTestNow },
		clearMain: func(goal.VerbRequest, []goal.TrunkRedClearArgs) (goal.PublishResult, error) {
			t.Fatal("an absent incident register must not publish a clear")
			return goal.PublishResult{}, nil
		},
		plainProve: plain.ProveSeams{Git: func(dir string, args ...string) (string, error) {
			reads++
			if dir == root {
				switch strings.Join(args, " ") {
				case "rev-parse --show-toplevel":
					return root, nil
				case "ls-tree --name-only main -- plans/goals/trunk-red.json":
					return "", nil
				}
			}
			t.Fatalf("unstubbed lane Git: %s %v", dir, args)
			return "", nil
		}},
	}
	proof := plain.Result{Result: plain.Green, Trunk: true, Scope: "scoped", Commit: "main", Ran: []string{"static"}}
	if err := owners.clearLandingIncidents(root, proof); err != nil || reads != 2 {
		t.Fatalf("incident read bypassed lane Git: reads=%d err=%v", reads, err)
	}
}

func TestAScopedGreenClosesOnlyItsGroupsRedWhileItApplies(t *testing.T) {
	t.Parallel()
	for _, movement := range []string{"same tip", "records", "executable", "new sighting"} {
		t.Run(movement, func(t *testing.T) {
			b := newIntentBed(t, false, nil)
			owners, proof, _ := scopedIncidentFixture(t, b)
			path := "records/moved.md"
			if movement == "executable" {
				path = "internal/code.go"
			}
			owners.plainProve.Git = func(_ string, args ...string) (string, error) {
				switch args[0] {
				case "rev-parse":
					return b.root(), nil
				case "cat-file", "merge-base":
					return "", nil
				case "diff":
					return path, nil
				default:
					return "", fmt.Errorf("unexpected Git read %v", args)
				}
			}
			if movement == "new sighting" {
				entries, problems := goal.ParseTrunkRed(b.repo.commit(proof.Commit).files["plans/goals/trunk-red.json"])
				if len(problems) != 0 {
					t.Fatal(problems)
				}
				ulid, err := goalUlid()
				helmMust(t, err)
				for i := range entries {
					if entries[i].Group == "fast-static-build" {
						sighting := entries[i].Sightings[0]
						sighting.Attempt, sighting.Opid = "new-red", goal.Opid(ulid, "finder", "sighting")
						entries[i].Sightings = append(entries[i].Sightings, sighting)
					}
				}
				commit, err := b.repo.Build("sighting", proof.Commit, []goal.Change{{Path: "plans/goals/trunk-red.json", Content: goal.RenderTrunkRed(entries)}}, "record another sighting")
				helmMust(t, err)
				_, err = b.repo.Publish(proof.Commit, commit)
				helmMust(t, err)
			} else if movement != "same tip" {
				commit, err := b.repo.Build("move", proof.Commit, []goal.Change{{Path: path, Content: []byte("moved")}}, "move main")
				helmMust(t, err)
				_, err = b.repo.Publish(proof.Commit, commit)
				helmMust(t, err)
			}
			before := b.repo.publications
			err := owners.clearLandingIncidents(b.root(), proof)
			if (err != nil) != (movement == "new sighting") {
				t.Fatalf("clear refusal: %v", err)
			}
			if movement == "new sighting" && !strings.Contains(err.Error(), "changed while it was being cleared") {
				t.Fatalf("lost expected-entry refusal: %v", err)
			}
			entries, problems := goal.ParseTrunkRed(b.repo.commit(b.repo.canonical).files["plans/goals/trunk-red.json"])
			closed := 0
			for _, entry := range entries {
				if entry.Closed != nil {
					closed++
					if entry.Group != "fast-static-build" || entry.Closed.Attempt != proof.Attempt {
						t.Fatalf("cleared a group the proof did not run: %+v", entry)
					}
				}
			}
			want := 1
			if movement == "executable" || movement == "new sighting" {
				want = 0
			}
			if len(problems) != 0 || closed != want || b.repo.publications-before != want {
				t.Fatalf("closed=%d publications=%d want=%d problems=%v", closed, b.repo.publications-before, want, problems)
			}
		})
	}
}

func TestAFailedClearIsReconciledWithoutARerun(t *testing.T) {
	t.Parallel()
	for _, batch := range []bool{false, true} {
		t.Run(map[bool]string{false: "scoped trunk", true: "published full batch"}[batch], func(t *testing.T) {
			b := newSelectionBed(t)
			register := newIntentBed(t, false, nil)
			owners, proof, _ := scopedIncidentFixture(t, register)
			proof.At = b.now.Format(time.RFC3339)
			main, tree := proof.Commit, proof.Tree
			if batch {
				proof.Trunk, proof.Scope, proof.FullTree, proof.FullAt = false, "full", proof.Tree, proof.At
				main, tree = main+"-records", "records-tree"
				inherited := proof
				inherited.Trunk, inherited.Scope, inherited.Commit, inherited.Tree = true, "inherited", main, tree
				inherited.Reason = "inherits green from tree proved-tree"
				writeCauseProof(t, b.lane, "results.jsonl", proof, inherited)
			} else {
				writeCauseProof(t, b.lane, "results.jsonl", proof)
			}
			seams := b.owners.landing.plainProve
			git := seams.Git
			launches := 0
			seams.Launch = func([]string, string, string) (int64, error) {
				launches++
				return 0, errors.New("unexpected proof launch")
			}
			seams.Git = func(dir string, args ...string) (string, error) {
				switch args[0] {
				case "rev-parse":
					if args[len(args)-1] == "--show-toplevel" {
						return b.lane, nil
					}
					if strings.HasSuffix(args[len(args)-1], "^{tree}") {
						return tree, nil
					}
					return main, nil
				case "merge-base", "cat-file":
					return "", nil
				case "diff":
					return "plans/goals/trunk-red.json", nil
				default:
					return git(dir, args...)
				}
			}
			fail := true
			keeper := newLandingAgentKeeper(b.lane, b.home, newTestLandingAgent(func(agent *landingAgent) {
				agent.proofEffects, agent.now = seams, func() time.Time { return b.now }
				agent.machine = owners.machine
				agent.settings = func(string) (launch.Settings, error) { return launch.DefaultSettings(), nil }
				agent.mainEndpoint = func(root string) (goal.Endpoint, error) {
					if fail {
						fail = false
						return goal.Endpoint{}, errors.New("endpoint unavailable once")
					}
					return owners.mainEndpoint(root)
				}
			}))
			before := register.repo.publications
			if err := keeper.Observe(b.record); err == nil || !strings.Contains(err.Error(), "endpoint unavailable once") {
				t.Fatalf("first failed clear was hidden: %v", err)
			}
			helmMust(t, keeper.Observe(b.record))
			entries, problems := goal.ParseTrunkRed(register.repo.commit(register.repo.canonical).files["plans/goals/trunk-red.json"])
			closed := 0
			for _, entry := range entries {
				if entry.Closed != nil {
					closed++
					if entry.Closed.Attempt != proof.Attempt || entry.Closed.BaseCommit != proof.Commit {
						t.Fatalf("reconciliation relabelled original execution: %+v", entry)
					}
				}
			}
			want := 1
			if batch {
				want = 2
			}
			if closed != want || len(problems) != 0 || register.repo.publications != before+1 {
				t.Fatalf("reconciliation lost incident effects: closed=%d problems=%v", closed, problems)
			}
			helmMust(t, keeper.Observe(b.record))
			if launches != 0 || register.repo.publications != before+1 {
				t.Fatalf("reconciliation repeated work: launches=%d publications=%d", launches, register.repo.publications-before)
			}
		})
	}
}
