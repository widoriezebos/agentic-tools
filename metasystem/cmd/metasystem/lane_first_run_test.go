package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal/branch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/plain"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/steward"
)

func TestLandingStatusLeadsWithCurrentTrunkRed(t *testing.T) {
	t.Parallel()
	incidents := holdIncidentFixture(t, newIntentBed(t, false, nil))
	b, _ := holdLaneFixture(t, incidents)
	proof := plain.Result{Trunk: true, Commit: "main", Result: plain.Red, Failed: []plain.FailedUnit{{Unit: "metasystem/internal/broken", Tests: []string{"TestBroken"}}}}
	writeQuestionFixture(t, filepath.Join(plain.Dir(b.install), "results.jsonl"), proof)
	code, words := b.run(t, b.root, "status")
	first := strings.Join(strings.Fields(strings.Split(strings.TrimSpace(words), "\n\n")[0]), " ")
	for _, want := range []string{"main main proven red", "metasystem/internal/broken", incidents[0].ID, "hot-fix", "metasystem landing prove --trunk"} {
		if code != 0 || !strings.Contains(first, want) {
			t.Fatalf("headline lacks %q: exit %d %s", want, code, words)
		}
	}
	// A newer batch record cannot hide the failed proof of main.
	data, err := json.Marshal(plain.Result{Commit: "batch", Result: plain.Green})
	helmMust(t, err)
	f, err := os.OpenFile(filepath.Join(plain.Dir(b.install), "results.jsonl"), os.O_APPEND|os.O_WRONLY, 0600)
	helmMust(t, err)
	_, err = fmt.Fprintf(f, "\n%s\n", data)
	helmMust(t, err)
	helmMust(t, f.Close())
	code, words = b.run(t, b.root, "status", "--json")
	var result struct {
		Summary string
		Data    plain.Status
	}
	helmMust(t, json.Unmarshal([]byte(words), &result))
	if code != 0 || !strings.Contains(result.Summary, "main main proven red") || !strings.Contains(result.Data.Summary, "main main proven red") {
		t.Fatalf("batch hid main: %d %s", code, words)
	}
	// A closed current incident and a newer green trunk proof clear the red.
	incidents[0].Closed = &goal.TrunkRedClosure{At: incidents[0].Opened, How: "hand", By: "Wido", Why: "fixed", Opid: incidents[0].Sightings[0].Opid}
	b.owners.landing.plainProve.Incidents = func(string, string, string) ([]goal.TrunkRedEntry, error) { return incidents, nil }
	writeQuestionFixture(t, filepath.Join(plain.Dir(b.install), "results.jsonl"), plain.Result{Trunk: true, Commit: "main", Result: plain.Green})
	code, words = b.run(t, b.root, "status")
	if code != 0 || strings.Contains(strings.Split(strings.TrimSpace(words), "\n")[0], "proven red") {
		t.Fatalf("closed proof still red: %d %s", code, words)
	}
}

func TestLandingStatusReportsLiveProofProgress(t *testing.T) {
	t.Parallel()
	b, _ := holdLaneFixture(t, nil)
	now := laneTestNow.Add(7 * time.Minute)
	b.owners.landing.plainProve.Now = func() time.Time { return now }
	b.owners.landing.plainProve.Alive = func(plain.Running) bool { return true }
	log := filepath.Join(b.install, "proof.log")
	helmMust(t, os.MkdirAll(plain.Dir(b.install), 0755))
	helmMust(t, os.WriteFile(log, []byte("landing planned 3\nlanding package metasystem/a 1 ok 60000\nlanding package metasystem/b 2 fail 120000\nlanding package metasystem/b 2 fail 120000\nlanding package partial"), 0600))
	writeQuestionFixture(t, filepath.Join(plain.Dir(b.install), "running.json"), plain.Running{Commit: "main", Tree: "tree", Trunk: true, Since: laneTestNow.Format(time.RFC3339), Log: log, Attempt: "proof", Pid: 4242})
	code, words := b.run(t, b.root, "status")
	first := strings.Join(strings.Fields(strings.Split(strings.TrimSpace(words), "\n\n")[0]), " ")
	if code != 0 || !strings.Contains(first, "2/3 units done") || !strings.Contains(first, "7.0 min elapsed") {
		t.Fatalf("live progress missing: %d %s", code, words)
	}
	code, words = b.run(t, b.root, "status", "--json")
	var progress struct{ Data plain.Status }
	helmMust(t, json.Unmarshal([]byte(words), &progress))
	if code != 0 || progress.Data.RunningProof == nil || progress.Data.RunningProof.UnitsDone != 2 || progress.Data.RunningProof.UnitsTotal == nil || *progress.Data.RunningProof.UnitsTotal != 3 {
		t.Fatalf("JSON progress missing: %d %s", code, words)
	}
	f, err := os.OpenFile(log, os.O_APPEND|os.O_WRONLY, 0600)
	helmMust(t, err)
	_, err = f.WriteString("\nlanding planned 2\nlanding package metasystem/a 1 ok 60000\n")
	helmMust(t, err)
	helmMust(t, f.Close())
	code, words = b.run(t, b.root, "status", "--json")
	helmMust(t, json.Unmarshal([]byte(words), &progress))
	if code != 0 || progress.Data.RunningProof.UnitsDone != 3 || progress.Data.RunningProof.UnitsTotal == nil || *progress.Data.RunningProof.UnitsTotal != 5 {
		t.Fatalf("later phase erased completed units: %d %s", code, words)
	}
}

func TestWorkLandMissingOriginBranchNamesPush(t *testing.T) {
	t.Parallel()
	b, state, _ := plainLaneBedWith(t, true, "critic-root")
	state.status = intentBranchState{EndpointTip: strings.Repeat("e", 40)}
	code, result := b.do("work", "land", bedGoal)
	expectOutcome(t, "missing origin branch", code, result, intentRefused)
	if result.Next == nil || !slices.Equal(result.Next.Argv, []string{"git", "push", "origin", "goal/" + bedGoal}) {
		t.Fatalf("unusable remedy: %+v", result)
	}
}

func TestWorkLandMergeCommitNamesRebase(t *testing.T) {
	t.Parallel()
	b, _, _ := plainLaneBedWith(t, true, "critic-root")
	commit := strings.Repeat("a", 40)
	// ReadRange is the production owner of the error shape, with Git facts injected.
	_, refusal := branch.ValidateRangeWithGit(b.root(), "main", commit, bedGoal, func(_ string, args ...string) ([]byte, error) {
		switch args[0] {
		case "merge-base":
			return []byte("base"), nil
		case "rev-list":
			return []byte(commit + " parent1 parent2"), nil
		}
		t.Fatalf("unstubbed range read: %v", args)
		return nil, nil
	})
	if refusal == nil {
		t.Fatal("merge commit was accepted")
	}
	b.owners.branchState = func(string, string) (intentBranchState, error) { return intentBranchState{BranchTip: commit}, refusal }
	code, result := b.do("work", "land", bedGoal)
	expectOutcome(t, "merge commit", code, result, intentRefused)
	if !strings.Contains(result.Summary, "2 parents") || result.Next == nil || !slices.Equal(result.Next.Argv, []string{"metasystem", "work", "rebase", bedGoal}) {
		t.Fatalf("unusable remedy: %+v", result)
	}
}

func TestSystemStartWaitsForStewardAnnouncement(t *testing.T) {
	t.Parallel()
	for _, verb := range []string{"start", "restart"} {
		t.Run(verb, func(t *testing.T) {
			t.Parallel()
			b := newProcessBed(t)
			owners := b.owners()
			elapsed := time.Duration(0)
			self, state, err := (identity.KernelProber{}).Probe(int64(os.Getpid()))
			if err != nil || state != identity.Alive {
				t.Fatalf("fixture identity: %s %v", state, err)
			}
			announced := steward.RunnerRecord{Pid: self.Pid, PidStartedAt: self.StartedAt.Unix(), StartTicks: self.StartTicks, BootID: self.BootID}
			owners.processes.process.armSteps = func(scope processScope, _ int, _ processArmAuthority) (processArmResult, error) {
				got, err := steward.AwaitRunnerStart(scope.Installation.Path(), func() (steward.RunnerRecord, bool) { return steward.LiveRunner(scope.Installation.Path()) }, make(chan error), func(d time.Duration) {
					elapsed += d
					if elapsed == 12*time.Second {
						writeQuestionFixture(t, filepath.Join(scope.Installation.Path(), "artifacts", "agents", "steward", "runner.json"), announced)
					}
					if elapsed > 12*time.Second {
						t.Fatal("announcement was ignored")
					}
				})
				if err != nil || got != announced {
					t.Fatalf("startup after %s: %+v %v", elapsed, got, err)
				}
				return processArmResult{lines: []string{"steward announced"}}, nil
			}
			code, result := b.runJSON(owners, "system", verb)
			if code != 0 || result.Outcome != intentConfirmed || elapsed != 12*time.Second {
				t.Fatalf("%s: exit %d elapsed %s %+v", verb, code, elapsed, result)
			}
		})
	}
}

func TestSystemStartStewardWaitHonorsExit(t *testing.T) {
	t.Parallel()
	b := newProcessBed(t)
	owners := b.owners()
	owners.processes.process.armSteps = func(scope processScope, _ int, _ processArmAuthority) (processArmResult, error) {
		exited := make(chan error, 1)
		polls := 0
		_, err := steward.AwaitRunnerStart(scope.Installation.Path(), func() (steward.RunnerRecord, bool) { return steward.LiveRunner(scope.Installation.Path()) }, exited, func(time.Duration) {
			polls++
			if polls > 1 {
				t.Fatal("exit was ignored")
			}
			exited <- nil
		})
		if err == nil || !strings.Contains(err.Error(), "died before guarding") {
			t.Fatalf("lost exit: %v", err)
		}
		return processArmResult{}, err
	}
	code, result := b.runJSON(owners, "system", "start")
	if code != 1 || result.Outcome != intentPartial {
		t.Fatalf("exited runner accepted: %d %+v", code, result)
	}
}
