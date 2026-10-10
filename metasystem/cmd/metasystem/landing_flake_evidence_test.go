package main

import (
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/plain"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/stateroot"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testgoal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testpolicy"
)

func flakeTestEvents(names []string, outcome string) string {
	var output strings.Builder
	for i, name := range names {
		for _, part := range []string{strings.Repeat(fmt.Sprintf("%s-%d\n", outcome, i), 10000), "tail\t\r\n"} {
			event, _ := json.Marshal(map[string]string{"Action": "output", "Package": "u/a", "Test": name, "Output": part})
			output.Write(event)
			output.WriteByte('\n')
		}
		event, _ := json.Marshal(map[string]string{"Action": outcome, "Package": "u/a", "Test": name})
		output.Write(event)
		output.WriteByte('\n')
	}
	event, _ := json.Marshal(map[string]string{"Action": outcome, "Package": "u/a"})
	output.Write(event)
	output.WriteByte('\n')
	return output.String()
}

func newFlakeEvidenceBed(t *testing.T, names []string) (*replayVerbBed, *goalCLIBed) {
	t.Helper()
	b := newReplayVerbBed(t)
	ledger := newGoalCLIBed(t, goalCLISeed{checkout: b.install})
	ledger.setNow(laneTestNow)
	at := laneTestNow.Add(-time.Hour).Format(time.RFC3339)
	legacy := goal.TrunkRedEntry{ID: "flaky:u/a", Identity: "flaky:u/a", Group: "u/a", Class: goal.TrunkRedClassPendingFlake, Status: "failed", Opened: at, Holds: []string{},
		Sightings: []goal.TrunkRedSighting{{Attempt: "old", BaseCommit: "old", SeenAt: at, Opid: goal.Opid("01J5X0000000000000000000F0", "lane", "fixture")}}}
	for _, name := range names {
		legacy.Failures = append(legacy.Failures, goal.TrunkRedFailure{Report: "old.log", Classname: "u/a", Name: name})
	}
	seed, err := ledger.repo.Files(ledger.tip(), "")
	if err != nil {
		t.Fatal(err)
	}
	seed["plans/goals/trunk-red.json"] = goal.RenderTrunkRed([]goal.TrunkRedEntry{legacy})
	ledger.repo = testgoal.New(seed, ledger.clock(), ledger.tip())
	endpoint, err := ledger.endpoint(b.install)
	if err != nil {
		t.Fatal(err)
	}
	contract := testpolicy.Contract{SchemaVersion: 1, ProjectRisk: testpolicy.ProjectRisk{Severity: 1, Exposure: 1, Reversibility: "revert", Detection: "immediate", Recovery: "bounded"},
		Fallback: "residual", Surfaces: []testpolicy.Surface{{ID: "unit", Paths: []string{"u/a/**"}, Standard: []string{"check"}}, {ID: "other", Paths: []string{"b/**", "metasystem/plans/goals/**"}}, {ID: "residual"}},
		Groups: []testpolicy.Group{{ID: "check", Kind: "unit", Adapter: "go", CWD: ".", Inputs: []string{"u/**"}, Platforms: []string{"any"}, TargetMS: 1, Packages: []string{"u/a"}, Tests: json.RawMessage(`"all"`)}},
		Always: testpolicy.Always{Canary: []string{"check"}}, Unknown: []string{"check"}, Cadence: []string{"check"}}
	data, err := json.Marshal(contract)
	if err != nil || contract.Validate() != nil {
		t.Fatalf("contract: %v %v", err, contract.Validate())
	}
	if err := os.WriteFile(filepath.Join(b.install, "metasystem.conf"), []byte("metasystem.template=true\nproof.full=fixture\ntesting.contract=testing.json\n"), 0600); err != nil {
		t.Fatal(err)
	}
	git := b.owners.landing.plainProve.Git
	b.owners.landing.plainProve.Git = func(dir string, args ...string) (string, error) {
		switch strings.Join(args, " ") {
		case "show origin/main:metasystem/testing.json":
			return string(data), nil
		case "show origin/main:metasystem/plans/goals/trunk-red.json":
			return ledger.accepted("plans/goals/trunk-red.json"), nil
		case "diff --name-only origin/main...merge-b":
			return "b/change.go", nil
		case "ls-tree -z origin/main:u/a":
			return "100644 blob abc\tunit.go\x00", nil
		}
		return git(dir, args...)
	}
	b.owners.landing.plainProve.Judge = nil
	b.owners.landing.mainEndpoint = func(string) (goal.Endpoint, error) { return endpoint, nil }
	b.prepareBatch(t)
	return b, ledger
}

func TestLandingProveKeepsTheFlakeRepeatSpent(t *testing.T) {
	t.Parallel()
	for _, name := range []string{"pass", "red again", "incomplete", "deadline", "crash before launch", "missing output", "write failure", "publication repair"} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			names := []string{"TestOne/sub/leaf", "TestTwo"}
			b, ledger := newFlakeEvidenceBed(t, names)
			failure := &exec.ExitError{ProcessState: replayFalseState(t)}
			publications := 0
			repair := name == "publication repair"
			b.owners.landing.recordFlake = func(r goal.VerbRequest, args goal.FlakeRecordArgs) (goal.FlakeRecordResult, error) {
				publications++
				result, err := goal.RecordFlake(r, args)
				if repair {
					return result, errors.New("the publication confirmation could not be read")
				}
				return result, err
			}
			b.fail = func(cmd *exec.Cmd, only string) (string, error) {
				if only == "" {
					return flakeTestEvents(names, "fail") + "LANDING-FAILED\tu/a\t" + strings.Join(names, " ") + "\nLANDING-CHECKED\t1\n", failure
				}
				results, err := plain.Results(b.install)
				if err != nil {
					t.Fatal(err)
				}
				started := results[len(results)-1]
				if started.Repeat != "started" || len(started.FlakeRepeats) != 1 || started.FlakeRepeats[0].Tree != started.Tree || started.FlakeRepeats[0].Attempt == started.Attempt {
					t.Fatalf("repeat executed without its frozen record: %+v", started)
				}
				switch name {
				case "red again":
					return flakeTestEvents(names, "fail") + "LANDING-FAILED\tu/a\t" + strings.Join(names, " ") + "\nLANDING-CHECKED\t1\n", failure
				case "incomplete":
					return flakeTestEvents(names, "pass"), nil
				case "deadline":
					return "", os.ErrDeadlineExceeded
				case "crash before launch":
					return "", &exec.Error{Name: "fixture", Err: os.ErrNotExist}
				case "missing output":
					return "LANDING-CHECKED\t0\n", nil
				case "write failure":
					key, _ := json.Marshal([2]string{"u/a", names[0]})
					if err := os.Mkdir(fmt.Sprintf("%s.%x.output", started.Log, sha256.Sum256(key)), 0700); err != nil {
						t.Fatal(err)
					}
				}
				return flakeTestEvents(names, "pass") + "LANDING-CHECKED\t0\n", nil
			}
			code, text := b.run(t, b.root, "prove", "--wait", "--json")
			var proof struct{ Data plain.Result }
			if err := json.Unmarshal([]byte(text), &proof); err != nil {
				t.Fatal(err, text)
			}
			red := name != "pass"
			if (code != 0) != red || (proof.Data.Result == plain.Red) != red || len(b.runs) != 2 {
				t.Fatalf("%s: exit=%d result=%+v runs=%v", name, code, proof.Data, b.runs)
			}
			if !red && (!proof.Data.FlakePublished || !proof.Data.RepeatComplete) {
				t.Fatalf("green lacks confirmed repeat: %+v", proof.Data)
			}
			if name == "pass" || name == "red again" || name == "publication repair" {
				if len(proof.Data.Flakes) != 1 {
					t.Fatalf("both attempt links missing: %+v", proof.Data)
				}
				f := proof.Data.Flakes[0]
				for i, test := range names {
					for _, repeat := range []bool{false, true} {
						outcome := "fail"
						output := f.Outputs[test]
						if repeat {
							output = f.RepeatOutputs[test]
							if name != "red again" {
								outcome = "pass"
							}
						}
						want := strings.Repeat(fmt.Sprintf("%s-%d\n", outcome, i), 10000) + "tail\t\r\n"
						data, err := os.ReadFile(output.Path)
						if err != nil || string(data) != want || output.Digest != fmt.Sprintf("%x", sha256.Sum256([]byte(want))) || output.Outcome != outcome {
							t.Fatalf("test output lost: test=%s repeat=%t evidence=%+v bytes=%d error=%v", test, repeat, output, len(data), err)
						}
					}
				}
			}
			if red {
				// Classification is also a public re-entry: repair uses retained logs only.
				repair = false
				if name == "publication repair" {
					_, text = b.run(t, b.root, "prove", "--wait", "--json")
				} else {
					resolver := b.owners.resolver
					b.owners.resolver = stateroot.NewResolver(func(string) (string, error) { return b.root, nil }, os.Executable)
					b.owners.prove = enrolledPersonProver(t, b.install, laneTestNow)
					_, text = b.run(t, b.root, "prove", "--classify", proof.Data.Attempt, "--json")
					latest, _, err := plain.LastResult(b.install)
					if err != nil || latest.ClassificationPerson == nil || latest.Repeat != "started" {
						t.Fatalf("classification did not retain the spent repeat: %+v %v %s", latest, err, text)
					}
					b.owners.resolver, b.owners.prove = resolver, nil
				}
				if len(b.runs) != 2 {
					t.Fatalf("classification bought another repeat: runs=%v output=%s", b.runs, text)
				}
				if name == "publication repair" {
					latest, _, err := plain.LastResult(b.install)
					if err != nil || latest.Result != plain.Green || !latest.FlakePublished || publications != 2 {
						t.Fatalf("publication did not recover: %+v %v output=%s", latest, err, text)
					}
					if latest.FullTree != latest.Tree || latest.FullAt != latest.At {
						t.Fatalf("recovered full proof lost its clock: %+v", latest)
					}
				} else if code, text := b.run(t, b.root, "prove", "--wait", "--json"); code == 0 || len(b.runs) != 2 {
					t.Fatalf("spent tree ran again: exit=%d %s runs=%v", code, text, b.runs)
				}
			}
			if name == "write failure" {
				key, _ := json.Marshal([2]string{"u/a", names[0]})
				if err := os.Remove(fmt.Sprintf("%s.%x.output", proof.Data.Log, sha256.Sum256(key))); err != nil {
					t.Fatal(err)
				}
				code, text := b.run(t, b.root, "prove", "--wait", "--json")
				latest, _, err := plain.LastResult(b.install)
				if code != 0 || err != nil || latest.Result != plain.Green || !latest.FlakePublished || publications != 1 || len(b.runs) != 2 {
					t.Fatalf("evidence repair reran or stayed red: exit=%d %s %+v %v runs=%v", code, text, latest, err, b.runs)
				}
				if latest.FullTree != latest.Tree || latest.FullAt != latest.At {
					t.Fatalf("recovered full proof lost its clock: %+v", latest)
				}
			}
			if name == "red again" {
				stops, err := plain.OpenStops(b.install)
				if err != nil {
					t.Fatal(err)
				}
				command := ""
				for _, stop := range stops {
					if stop.Loop == "lane-proof" {
						command = stop.Command()
					}
				}
				if command != "metasystem landing prove" {
					t.Fatalf("no actionable fresh proof command: %q stops=%+v", command, stops)
				}
				b.owners.resolver = stateroot.NewResolver(func(string) (string, error) { return b.root, nil }, os.Executable)
				b.owners.prove = enrolledPersonProver(t, b.install, laneTestNow)
				b.fail = func(*exec.Cmd, string) (string, error) {
					return flakeTestEvents(names, "pass") + "LANDING-CHECKED\t0\n", nil
				}
				code, text := b.run(t, b.root, append(strings.Fields(command)[2:], "--wait", "--json")...)
				var override struct{ Data plain.Result }
				if code != 0 || json.Unmarshal([]byte(text), &override) != nil || override.Data.Result != plain.Green || override.Data.Attempt == proof.Data.Attempt || override.Data.Person == nil || len(override.Data.Executions) == 0 || !override.Data.Executions[len(override.Data.Executions)-1].Override || len(b.runs) != 3 {
					t.Fatalf("person's fresh proof failed: exit=%d %s runs=%v", code, text, b.runs)
				}
			}
			entries, problems := goal.ParseTrunkRed([]byte(ledger.accepted("plans/goals/trunk-red.json")))
			if len(problems) != 0 {
				t.Fatal(problems)
			}
			want := 1
			if name == "pass" || name == "publication repair" || name == "write failure" {
				want = 3
			}
			if len(entries) != want {
				t.Fatalf("wrong sighting count: %+v", entries)
			}
			for _, entry := range entries {
				if entry.Class != goal.TrunkRedClassFlake {
					continue
				}
				if len(entry.Sightings) != 1 || entry.Sightings[0].Output == nil || entry.Sightings[0].Rerun == nil || entry.Sightings[0].Rerun.Output == nil {
					t.Fatalf("sighting dropped output evidence: %+v", entry)
				}
			}
		})
	}
}
