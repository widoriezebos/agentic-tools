package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/humanauthority"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/lane"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/plain"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/launch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/stateroot"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testpolicy"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testpolicy/adapter"
)

func TestLandingKeeperObservesRemoteRecordsWithEmptyQueue(t *testing.T) {
	t.Parallel()
	for _, scenario := range []string{"records advance", "fetch fails", "candidate repair", "landed batch"} {
		t.Run(scenario, func(t *testing.T) {
			failedFetch := scenario == "fetch fails"
			t.Parallel()
			b := newSelectionBed(t)
			if err := os.Remove(filepath.Join(plain.Dir(b.lane), "queue.jsonl")); err != nil {
				t.Fatal(err)
			}
			// This fixture is an adopted installation: paths are relative to its root.
			contract := testpolicy.Contract{SchemaVersion: testpolicy.ExecutionContractSchemaVersion,
				ProjectRisk: testpolicy.ProjectRisk{Severity: 1, Exposure: 1, Reversibility: "revert", Detection: "immediate", Recovery: "bounded"},
				Surfaces:    []testpolicy.Surface{{ID: "fixture", Paths: []string{"plans/**", "src/**"}, Standard: []string{"records"}}}, Unknown: []string{"records"}}
			for _, id := range []string{"records", "other"} {
				input := "plans/**"
				if id == "other" {
					input = "src/**"
				}
				contract.Groups = append(contract.Groups, testpolicy.Group{ID: id, Kind: "unit", Adapter: "command", CWD: ".", Phase: "acceptance", EnvironmentMode: "inherit", Inputs: []string{input}, Platforms: []string{"any"}, TargetMS: 1, Argv: []string{"fixture"}, Format: "exit-status"})
			}
			encoded, err := json.Marshal(contract)
			helmMust(t, err)
			helmMust(t, os.WriteFile(filepath.Join(b.lane, "metasystem.conf"), []byte("testing.contract=testing.json\nproof.trunk-every=4h\n"), 0600))
			at := b.now.Add(-10 * time.Minute).Format(time.RFC3339)
			writeCauseProof(t, b.lane, "results.jsonl", plain.Result{Trunk: true, Commit: "old-main", Tree: "old-tree", Result: plain.Green, Scope: "full", FullAt: at, At: at, FullTree: "old-tree", Environment: "fixture-toolchain"})
			if scenario == "landed batch" {
				writeCauseProof(t, b.lane, "results.jsonl", plain.Result{Commit: "remote-records", Tree: "records-tree", Result: plain.Green, Scope: "full", FullAt: at, At: at, FullTree: "records-tree", Environment: "fixture-toolchain"})
			}

			seams := b.owners.landing.plainProve
			fallback := seams.Git
			local := "old-main"
			fetches, launches := 0, 0
			seams.Git = func(dir string, args ...string) (string, error) {
				switch args[0] {
				case "fetch":
					fetches++
					if failedFetch {
						return "", errors.New("remote unavailable")
					}
					local = "remote-records"
					return "", nil
				case "rev-parse":
					if strings.Contains(args[len(args)-1], "origin/main^{commit}") {
						return local, nil
					}
					if args[len(args)-1] == "remote-records^{tree}" {
						return "records-tree", nil
					}
				case "show":
					if args[1] == "records-tree:testing.json" {
						return string(encoded), nil
					}
					if args[1] == "remote-records:metasystem.conf" {
						return "proof.full=fixture-proof\n", nil
					}
				case "diff":
					return "plans/designs/fixture.md", nil
				case "merge-base":
					if args[2] == "candidate" {
						return fallback(dir, args...)
					}
					return "", nil
				}
				return fallback(dir, args...)
			}
			seams.Policy = func(string) (plain.PolicyValue, error) { return plain.PolicyValue{Value: "auto"}, nil }
			seams.Closure = func(string, string, string) (adapter.Closure, error) { return adapter.Closure{}, nil }
			seams.Executable = func() (string, error) { return "fixture-engine", nil }
			seams.Launch = func([]string, string, string) (int64, error) { launches++; return int64(os.Getpid()), nil }
			seams.Alive = func(r plain.Running) bool { return !r.Checkpoint }
			checkpoint := plain.Running{Checkpoint: true, Attempt: "candidate", Commit: "candidate", Tree: "candidate-tree"}
			if scenario == "candidate repair" {
				checkpointData, err := json.Marshal(checkpoint)
				helmMust(t, err)
				helmMust(t, os.WriteFile(filepath.Join(plain.Dir(b.lane), "running.json"), checkpointData, 0600))
				f, err := os.OpenFile(filepath.Join(plain.Dir(b.lane), "results.jsonl"), os.O_APPEND|os.O_WRONLY, 0600)
				helmMust(t, err)
				data, err := json.Marshal(plain.Result{Attempt: checkpoint.Attempt, Commit: checkpoint.Commit, Tree: checkpoint.Tree, Result: plain.Red})
				helmMust(t, err)
				_, err = f.Write(append(data, '\n'))
				helmMust(t, err)
				helmMust(t, f.Close())
			}
			keeper := newLandingAgentKeeper(b.lane, b.home, newTestLandingAgent(func(a *landingAgent) {
				a.proofEffects = seams
				a.now = func() time.Time { return b.now }
				a.machine = func(string) (string, error) { return "fixture", nil }
				a.settings = func(string) (launch.Settings, error) { return launch.DefaultSettings(), nil }
			}))
			// Observe is the production keeper hook, including the independent bounded fetch.
			err = keeper.Observe(b.record)
			if failedFetch {
				if err == nil || !strings.Contains(err.Error(), "remote unavailable") || launches != 0 {
					t.Fatalf("failed observation: %v launches=%d", err, launches)
				}
				return
			}
			if scenario == "landed batch" {
				_, recorded, _, readErr := plain.ReadRunning(b.lane, seams)
				if err != nil || readErr != nil || recorded || launches != 0 || fetches != 1 {
					t.Fatalf("landed batch rechecked: recorded=%v launches=%d fetches=%d errors=%v/%v", recorded, launches, fetches, err, readErr)
				}
				return
			}

			wantFetches := 1
			if scenario == "candidate repair" {
				stored, _, _, readErr := plain.ReadRunning(b.lane, seams)
				if err != nil || readErr != nil || launches != 0 || !reflect.DeepEqual(stored, checkpoint) || !plain.RecordedFixRed(b.lane, stored) {
					t.Fatalf("observation replaced repair or revoked admission: %+v %v %v", stored, err, readErr)
				}
				if err := keeper.Observe(b.record); err != nil {
					t.Fatal(err)
				}
				helmMust(t, os.Remove(filepath.Join(plain.Dir(b.lane), "running.json")))
				err = keeper.Observe(b.record)
				wantFetches = 3
			}
			running, recorded, _, readErr := plain.ReadRunning(b.lane, seams)
			if err != nil || readErr != nil || fetches != wantFetches || launches != 1 || !recorded || running.Commit != "remote-records" || running.Admission.Scope != "scoped" || strings.Join(running.Admission.Groups, " ") != "records" {
				t.Fatalf("production observation: %+v err=%v read=%v fetch=%d launch=%d", running, err, readErr, fetches, launches)
			}
			if queue, err := plain.Waiting(b.lane); err != nil || len(queue) != 0 {
				t.Fatalf("observation depended on a queue: %+v %v", queue, err)
			}
		})
	}
}

func TestLandingTrunkPersonReuseAndRed(t *testing.T) {
	t.Parallel()
	for _, scenario := range []string{"green", "fresh", "red", "trunk red", "newer red", "damaged history", "impact", "expired scoped", "missing environment", "changed environment"} {
		for _, wait := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/wait=%v", scenario, wait), func(t *testing.T) {
				t.Parallel()
				b := newResolveVerbFixture(t)
				b.owners.resolver = stateroot.NewResolver(func(string) (string, error) { return b.root, nil }, os.Executable)
				b.owners.prove = enrolledPersonProver(t, b.root, laneTestNow)
				seams := b.owners.landing.plainProve
				git := seams.Git
				seams.Git = func(dir string, args ...string) (string, error) {
					switch args[0] {
					case "fetch", "diff", "worktree":
						if len(args) > 3 && args[0] == "worktree" && args[1] == "add" {
							helmMust(t, os.MkdirAll(filepath.Join(args[3], "metasystem"), 0700))
						}
						return "", nil
					case "show":
						return "proof.full=fixture-proof\n", nil
					}
					return git(dir, args...)
				}
				seams.Now = func() time.Time { return laneTestNow }
				seams.NewID = func() string { return "new-proof" }
				seams.Executable = func() (string, error) { return "fixture-engine", nil }
				launches := 0
				seams.Launch = func([]string, string, string) (int64, error) { launches++; return int64(os.Getpid()), nil }
				seams.Command = func(cmd *exec.Cmd) error {
					fmt.Fprintln(cmd.Stdout, "landing environment fixture\nLANDING-CHECKED\t0")
					return nil
				}
				b.owners.landing.plainProve = seams
				at := laneTestNow.Add(-time.Minute).Format(time.RFC3339)
				r := plain.Result{Attempt: "saved", Commit: "head", Tree: "head", Result: plain.Green, Scope: "full", FullTree: "head", FullAt: at, At: at, Environment: "fixture"}
				switch scenario {
				case "red", "trunk red":
					r.Result = plain.Red
					r.Trunk = scenario == "trunk red"
				case "impact":
					r.Scope, r.FullTree, r.FullAt = "impact", "", ""
				case "expired scoped":
					r.Scope, r.FullTree, r.FullAt = "scoped", "original", laneTestNow.Add(-61*time.Minute).Format(time.RFC3339)
				case "missing environment":
					r.Environment = ""
				}
				writeCauseProof(t, b.install, "results.jsonl", r)
				if scenario == "newer red" || scenario == "changed environment" {
					newer := r
					newer.Attempt = "newer"
					if scenario == "newer red" {
						newer.Result = plain.Red
					} else {
						newer.Tree, newer.Environment = "other", "new environment"
					}
					data, err := json.Marshal(newer)
					helmMust(t, err)
					f, err := os.OpenFile(filepath.Join(plain.Dir(b.install), "results.jsonl"), os.O_APPEND|os.O_WRONLY, 0600)
					helmMust(t, err)
					_, err = f.Write(append(data, '\n'))
					helmMust(t, err)
					helmMust(t, f.Close())
				}
				if scenario == "damaged history" {
					f, err := os.OpenFile(filepath.Join(plain.Dir(b.install), "results.jsonl"), os.O_APPEND|os.O_WRONLY, 0600)
					helmMust(t, err)
					_, err = f.WriteString("broken\n")
					helmMust(t, err)
					helmMust(t, f.Close())
				}
				args := []string{"prove", "--trunk"}
				if wait {
					args = append(args, "--wait")
				}
				if scenario == "fresh" {
					args = append(args, "--fresh")
				}
				code, out := b.run(t, b.install, args...)
				if code != 0 {
					t.Fatalf("person trunk: exit %d %s", code, out)
				}
				if scenario == "green" {
					if launches != 0 || !strings.Contains(out, "main at head is proven by the") || !strings.Contains(strings.Join(strings.Fields(out), " "), "check of the same tree; landing prove --trunk --fresh runs a new full check") {
						t.Fatalf("green settlement: launches=%d %s", launches, out)
					}
				} else {
					if launches != 1 || strings.Contains(out, "already proven green") || strings.Contains(out, "check of the same tree") {
						t.Fatalf("red or invalid proof settled: launches=%d %s", launches, out)
					}
					if !wait {
						running, _, _, err := plain.ReadRunning(b.install, seams)
						if err != nil || running.Admission.Scope != "full" {
							t.Fatalf("admission: %+v %v", running, err)
						}
					}
				}
			})
		}
	}
}

func TestLandingTrunkPersonExplainsRepairBeforeReplacement(t *testing.T) {
	t.Parallel()
	for _, route := range []string{"prove", "resume"} {
		t.Run(route, func(t *testing.T) {
			t.Parallel()
			b := newSelectionBed(t)
			b.owners.prove = enrolledPersonProver(t, b.lane, b.now)
			checkpoint := plain.Running{Checkpoint: true, Attempt: "candidate", Commit: "candidate", Tree: "candidate-tree"}
			data, err := json.Marshal(checkpoint)
			helmMust(t, err)
			helmMust(t, os.WriteFile(filepath.Join(plain.Dir(b.lane), "running.json"), data, 0600))
			writeCauseProof(t, b.lane, "results.jsonl", plain.Result{Attempt: checkpoint.Attempt, Commit: checkpoint.Commit, Tree: checkpoint.Tree, Result: plain.Red})
			helmMust(t, plain.WriteFix(b.lane, &plain.Fix{Attempt: checkpoint.Attempt, Goal: "repair", Units: []string{"unit"}, Job: "job", Commit: "candidate", State: "building"}))
			seams := b.owners.landing.plainProve
			seams.Incidents = func(string, string, string) ([]goal.TrunkRedEntry, error) {
				return []goal.TrunkRedEntry{{Identity: "red", Opened: b.now.Format(time.RFC3339)}}, nil
			}
			git := seams.Git
			missing := exec.Command("/bin/sh", "-c", "exit 128")
			_ = missing.Run()
			absent := missing.ProcessState
			seams.Git = func(dir string, args ...string) (string, error) {
				if strings.Contains(strings.Join(args, " "), "MERGE_HEAD") {
					return "", &exec.ExitError{ProcessState: absent}
				}
				if args[0] == "show" {
					return "proof.full=fixture-proof\n", nil
				}
				return git(dir, args...)
			}
			seams.Executable = func() (string, error) { return "fixture-engine", nil }
			var output bytes.Buffer
			notified := false
			seams.NotifyRepairOverride = func(impact string) error {
				notified = true
				stored, _, _, err := plain.ReadRunning(b.lane, seams)
				fix, fixErr := plain.ReadFix(b.lane)
				rows, rowsErr := os.ReadFile(filepath.Join(plain.Dir(b.lane), "interventions.jsonl"))
				var entry plain.Intervention
				decodeErr := json.Unmarshal(bytes.TrimSpace(rows), &entry)
				if err != nil || fixErr != nil || rowsErr != nil || decodeErr != nil || fix == nil || fix.State != "building" || !reflect.DeepEqual(stored, checkpoint) || entry.Person == nil || entry.Person.Person != "Wido" || entry.What != impact || !strings.Contains(output.String(), impact) {
					t.Fatalf("CLI notification boundary: output=%s stored=%+v fix=%+v row=%+v errors=%v/%v/%v/%v", &output, stored, fix, entry, err, fixErr, rowsErr, decodeErr)
				}
				return nil
			}
			seams.Launch = func([]string, string, string) (int64, error) {
				fix, err := plain.FixForAttempt(b.lane, checkpoint.Attempt)
				running, _, _, readErr := plain.ReadRunning(b.lane, seams)
				if !notified || err != nil || readErr != nil || fix == nil || fix.State != "closed" || !running.Trunk || running.Attempt == checkpoint.Attempt {
					t.Fatalf("launch boundary: notified=%v fix=%+v running=%+v errors=%v/%v", notified, fix, running, err, readErr)
				}
				return int64(os.Getpid()), nil
			}
			b.owners.landing.plainProve = seams
			args := []string{}
			if route == "prove" {
				args = append(args, "--trunk")
			}
			code := runIntentIn(mustIntentCommand(t, "landing "+route), args, &output, &output, b.lane, b.owners)
			if code != 0 || !notified {
				t.Fatalf("person repair override: exit %d %s", code, &output)
			}
		})
	}
}
func TestLandingTrunkMachineWaitsForCandidateRepair(t *testing.T) {
	t.Parallel()
	for _, checkpointPresent := range []bool{true, false} {
		t.Run(fmt.Sprint(checkpointPresent), func(t *testing.T) {
			t.Parallel()
			b := newSelectionBed(t)
			b.owners.prove = func(string, int64, humanauthority.Reader, string, string, time.Time) (humanauthority.Proof, error) {
				return humanauthority.Proof{}, errors.New("agent caller")
			}
			checkpoint := plain.Running{Checkpoint: true, Attempt: "candidate", Commit: "candidate", Tree: "candidate-tree"}
			if checkpointPresent {
				data, err := json.Marshal(checkpoint)
				helmMust(t, err)
				helmMust(t, os.WriteFile(filepath.Join(plain.Dir(b.lane), "running.json"), data, 0600))
				writeCauseProof(t, b.lane, "results.jsonl", plain.Result{Attempt: checkpoint.Attempt, Commit: checkpoint.Commit, Tree: checkpoint.Tree, Result: plain.Red})
			}
			helmMust(t, plain.WriteFix(b.lane, &plain.Fix{Attempt: checkpoint.Attempt, Goal: "repair", Units: []string{"unit"}, Job: "job", Commit: "candidate", State: "building"}))
			seams := b.owners.landing.plainProve
			git := seams.Git
			seams.Git = func(dir string, args ...string) (string, error) {
				if args[0] == "show" {
					return "proof.full=fixture-proof\n", nil
				}
				return git(dir, args...)
			}
			seams.Launch = func([]string, string, string) (int64, error) {
				t.Fatal("machine replaced candidate repair")
				return 0, nil
			}
			b.owners.landing.plainProve = seams
			code, output := b.run(t, b.lane, "landing", "prove", "--trunk", "--json")
			var result struct{ Outcome, Summary string }
			want := "main's check waits for the pending fix of repair"
			if checkpointPresent {
				want = "main's check waits for the candidate's repair of tree candidate-tr to finish"
			}
			if err := json.Unmarshal([]byte(output), &result); err != nil || code != 1 || result.Outcome != "refused" || result.Summary != want {
				t.Fatalf("machine refusal: exit=%d result=%+v err=%v output=%s", code, result, err, output)
			}
			stored, recorded, _, err := plain.ReadRunning(b.lane, seams)
			if err != nil || recorded != checkpointPresent || recorded && !reflect.DeepEqual(stored, checkpoint) {
				t.Fatalf("machine changed checkpoint: %+v recorded=%v err=%v", stored, recorded, err)
			}
		})
	}
}

func TestLandingTrunkMachineReportsCompletedReuse(t *testing.T) {
	t.Parallel()
	for _, scenario := range []string{"trunk green", "trunk red", "batch green"} {
		for _, wait := range []bool{false, true} {
			for _, jsonOutput := range []bool{false, true} {
				t.Run(fmt.Sprintf("%s/wait=%v/json=%v", scenario, wait, jsonOutput), func(t *testing.T) {
					t.Parallel()
					b := newResolveVerbFixture(t)
					b.owners.prove = func(string, int64, humanauthority.Reader, string, string, time.Time) (humanauthority.Proof, error) {
						return humanauthority.Proof{}, errors.New("agent caller")
					}
					seams := b.owners.landing.plainProve
					git := seams.Git
					seams.Git = func(dir string, args ...string) (string, error) {
						if args[0] == "fetch" {
							return "", nil
						}
						if args[0] == "show" {
							return "proof.full=fixture-proof\n", nil
						}
						return git(dir, args...)
					}
					seams.Now = func() time.Time { return laneTestNow }
					seams.Alive = func(r plain.Running) bool { return r.Process != "" && !r.Checkpoint }
					seams.WaitPoll = func() { t.Fatal("completed reuse waited for a process") }
					seams.Launch = func([]string, string, string) (int64, error) {
						t.Fatal("completed reuse launched a check")
						return 0, nil
					}
					seams.Incidents = func(string, string, string) ([]goal.TrunkRedEntry, error) {
						return []goal.TrunkRedEntry{{ID: "incident", Identity: "red:fixture-unit"}}, nil
					}
					b.owners.landing.plainProve = seams
					at := laneTestNow.Add(-34 * time.Second).Format(time.RFC3339)
					r := plain.Result{Attempt: "saved-attempt", Commit: "head", Tree: "head", Result: plain.Green, Trunk: true,
						Scope: "full", FullTree: "head", FullAt: at, At: at, Environment: "fixture"}
					if scenario == "batch green" {
						r.Trunk, r.Commit = false, "batch-source"
					}
					if scenario == "trunk red" {
						r.Result, r.Cause, r.Failed = plain.Red, &plain.Cause{Kind: "main"}, []plain.FailedUnit{{Unit: "fixture-unit"}}
						checkpoint, err := json.Marshal(plain.Running{Checkpoint: true, Attempt: r.Attempt, Tree: r.Tree, Commit: r.Commit, Trunk: true})
						helmMust(t, err)
						helmMust(t, os.MkdirAll(plain.Dir(b.install), 0700))
						helmMust(t, os.WriteFile(filepath.Join(plain.Dir(b.install), "running.json"), checkpoint, 0600))
					}
					writeCauseProof(t, b.install, "results.jsonl", r)
					args := []string{"--trunk"}
					if wait {
						args = append(args, "--wait")
					}
					if jsonOutput {
						args = append(args, "--json")
					}
					var output bytes.Buffer
					code := runIntentIn(mustIntentCommand(t, "landing prove"), args, &output, &output, b.root, b.owners)
					if code != 0 {
						t.Fatalf("reuse exit=%d: %s", code, &output)
					}
					line := output.String()
					if jsonOutput {
						var response struct {
							Summary string
							Data    map[string]json.RawMessage
						}
						helmMust(t, json.Unmarshal(output.Bytes(), &response))
						line = response.Summary
						for key, want := range map[string]string{"commit": r.Commit, "attempt": r.Attempt, "at": r.At, "result": r.Result} {
							var got string
							if err := json.Unmarshal(response.Data[key], &got); err != nil || got != want {
								t.Errorf("data.%s=%s want=%q err=%v", key, response.Data[key], want, err)
							}
						}
						var trunk bool
						_, present := response.Data["trunk"]
						if present {
							helmMust(t, json.Unmarshal(response.Data["trunk"], &trunk))
						}
						if present != r.Trunk || trunk != r.Trunk {
							t.Errorf("source trunk=%s want=%v", response.Data["trunk"], r.Trunk)
						}
					}
					for _, want := range []string{"main at head", r.Commit, r.Attempt, r.Result, lane.LocalText(r.At), "nothing started"} {
						if !strings.Contains(strings.Join(strings.Fields(line), " "), want) {
							t.Errorf("missing %q: %s", want, line)
						}
					}
					if strings.Contains(line, "landing push may put it on main") || strings.Contains(line, "already proving") {
						t.Fatalf("wrong reuse message: %s", line)
					}
				})
			}
		}
	}
}

func TestLandingTrunkMachineLiveJoinStillReportsRunning(t *testing.T) {
	t.Parallel()
	b := newResolveVerbFixture(t)
	b.owners.prove = func(string, int64, humanauthority.Reader, string, string, time.Time) (humanauthority.Proof, error) {
		return humanauthority.Proof{}, errors.New("agent caller")
	}
	seams := b.owners.landing.plainProve
	git := seams.Git
	seams.Git = func(dir string, args ...string) (string, error) {
		if args[0] == "fetch" {
			return "", nil
		}
		if args[0] == "show" {
			return "proof.full=fixture-proof\n", nil
		}
		return git(dir, args...)
	}
	seams.Alive = func(r plain.Running) bool { return r.Process != "" }
	seams.Launch = func([]string, string, string) (int64, error) { t.Fatal("live join launched again"); return 0, nil }
	b.owners.landing.plainProve = seams
	running := plain.Running{Attempt: "live", Commit: "head", Tree: "head", Trunk: true, Process: "fixture-process", Pid: 42, Since: laneTestNow.Format(time.RFC3339)}
	data, err := json.Marshal(running)
	helmMust(t, err)
	helmMust(t, os.MkdirAll(plain.Dir(b.install), 0700))
	helmMust(t, os.WriteFile(filepath.Join(plain.Dir(b.install), "running.json"), data, 0600))
	code, out := b.run(t, b.root, "prove", "--trunk", "--json")
	var response struct {
		Summary string
		Data    plain.Running
	}
	err = json.Unmarshal([]byte(out), &response)
	if err != nil || code != 0 || !strings.Contains(response.Summary, "already proving") || !strings.Contains(response.Summary, "since "+lane.LocalText(running.Since)) || response.Data.Attempt != running.Attempt || response.Data.Reused != nil || strings.Contains(response.Summary, "nothing started") {
		t.Fatalf("live join: code=%d response=%+v err=%v output=%s", code, response, err, out)
	}
}
