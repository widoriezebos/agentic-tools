package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/channel"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/lane"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/plain"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testpolicy"
)

func mainRepeatRecord(t *testing.T, name string) plain.Result {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", name))
	helmMust(t, err)
	var result plain.Result
	helmMust(t, json.Unmarshal(data, &result))
	return result
}

func mainRepeatEvents(unit string, names []string, outcome string) string {
	var text strings.Builder
	for _, name := range names {
		for _, event := range []map[string]string{
			{"Action": "output", "Package": unit, "Test": name, "Output": "fixture output\n"},
			{"Action": outcome, "Package": unit, "Test": name},
		} {
			data, _ := json.Marshal(event)
			fmt.Fprintln(&text, string(data))
		}
	}
	data, _ := json.Marshal(map[string]string{"Action": outcome, "Package": unit})
	fmt.Fprintln(&text, string(data))
	return text.String()
}

func TestLandingTrunkRedRepeatsBeforeIncidentOrQuestion(t *testing.T) {
	t.Parallel()
	for _, outcome := range []string{"pass", "fail", "incomplete", "killed after report", "publication failure"} {
		t.Run(outcome, func(t *testing.T) {
			t.Parallel()
			row := mainRepeatRecord(t, "trunk-repeat-red.json")
			if !row.Trunk || row.Attempt != "20261012T014045.916632000Z" || row.Failed[0].Tests[0] != "TestProofRunLegacyPublicLaunchUsesAndClearsHostPhase" {
				t.Fatalf("saved trunk shape: %+v", row)
			}
			b, register, _ := incidentProveFixture(t, false)
			unit := row.Failed[0]
			before := register.repo.canonical
			register.repo.commits["main"] = register.repo.commit(before)
			b.owners.landing.plainProve.NewID = func() string { return row.Attempt }
			var flakes []plain.FlakeRecord
			writeFlake := b.owners.landing.landingFlakeRecorder(b.install)
			b.owners.landing.plainProve.RecordFlake = func(f plain.FlakeRecord) (plain.FlakeRecorded, error) {
				flakes = append(flakes, f)
				if outcome == "publication failure" {
					return plain.FlakeRecorded{}, errors.New("publication unavailable")
				}
				return writeFlake(f)
			}
			b.fail = func(cmd *exec.Cmd, only string) (string, error) {
				if only != "" {
					if only != unit.Unit || freshProofEnv(cmd, "LANDING_ONLY_TESTS") != strings.Join(unit.Tests, " ") || freshProofEnv(cmd, "LANDING_COMMIT") != "main" {
						t.Fatalf("repeat selector or tree: %s %v", only, cmd.Env)
					}
					if len(b.runs) != 2 {
						t.Fatalf("repeat repeated itself: %v", b.runs)
					}
					effects := b.owners.landing.plainProve
					effects.Lane = func() (lane.Record, error) { r, _, err := lane.Read(b.home); return r, err }
					effects.Policy = func(string) (plain.PolicyValue, error) { return plain.PolicyValue{Value: "person"}, nil }
					effects.Incidents = func(string, string, string) ([]goal.TrunkRedEntry, error) {
						return []goal.TrunkRedEntry{{ID: "older-red", Identity: "older-red"}}, nil
					}
					if err := plain.SyncPolicyQuestion(b.install, b.owners.landing.machine, laneTestNow, effects); err != nil {
						t.Fatal(err)
					}
					questions, err := channel.WalkQuestions(b.install)
					if err != nil || len(questions) != 0 || register.repo.canonical != before {
						t.Fatalf("pending repeat opened an incident or question: %v %v", questions, err)
					}
					if outcome == "incomplete" {
						return "LANDING-NOT-RUN\tfixture\n", errors.New("lost")
					}
					if outcome == "killed after report" {
						return mainRepeatEvents(unit.Unit, unit.Tests, "fail") + fmt.Sprintf("LANDING-FAILED\t%s\t%s\nLANDING-CHECKED\t1\n", unit.Unit, strings.Join(unit.Tests, " ")), exec.Command("/bin/sh", "-c", "kill -KILL $$").Run()
					}
					if outcome != "fail" {
						return mainRepeatEvents(unit.Unit, unit.Tests, "pass") + "LANDING-CHECKED\t0\n", nil
					}
				}
				return mainRepeatEvents(unit.Unit, unit.Tests, "fail") + fmt.Sprintf("LANDING-FAILED\t%s\t%s\nLANDING-LOAD\t2.75\nLANDING-CHECKED\t1\n", unit.Unit, strings.Join(unit.Tests, " ")), errors.New("red")
			}
			code, out := b.run(t, b.root, "prove", "--trunk", "--wait", "--json")
			var response struct{ Data plain.Result }
			helmMust(t, json.Unmarshal([]byte(out), &response))
			check := response.Data
			if len(b.runs) != 2 {
				t.Fatalf("trunk repeat skipped: %v; %s", b.runs, out)
			}
			var entries []goal.TrunkRedEntry
			var problems []goal.Problem
			if data := register.repo.commit(register.repo.canonical).files["plans/goals/trunk-red.json"]; len(data) > 0 {
				entries, problems = goal.ParseTrunkRed(data)
			}
			if len(problems) != 0 {
				t.Fatal(problems)
			}
			if outcome == "fail" {
				if code != 1 || check.Cause == nil || check.Cause.Kind != "main" || len(entries) != 1 || register.repo.commit(register.repo.canonical).parent != before {
					t.Fatalf("persistent red: %s incidents=%+v", out, entries)
				}
				return
			}
			if outcome != "pass" && (len(entries) != 0 || register.repo.canonical != before) {
				t.Fatalf("flake or incomplete repeat became incident: %+v", entries)
			}
			if outcome == "pass" {
				if code != 0 || check.Result != plain.Green || check.Cause != nil || len(flakes) != 1 || flakes[0].Where != "main" || flakes[0].Attempt != row.Attempt || flakes[0].Load != 2.75 {
					t.Fatalf("flake record: %s %+v", out, flakes)
				}
				facts := entries
				if len(facts) != 1 || facts[0].Class != goal.TrunkRedClassFlake || facts[0].Sightings[0].Where != "" || facts[0].Sightings[0].Attempt != row.Attempt || facts[0].Sightings[0].Load != 2.75 {
					t.Fatalf("flake register: %+v %v", facts, problems)
				}
				statusCode, status := b.run(t, b.root, "status", "--json")
				if statusCode != 0 || !strings.Contains(status, "flake on main: "+unit.Tests[0]+", green on repeat; registered") {
					t.Fatalf("status: %d %s", statusCode, status)
				}
			} else if code != 1 || check.Cause == nil || check.Cause.Kind == "main" {
				t.Fatalf("unconfirmed repeat attributed main: %s", out)
			}
		})
	}
}

func TestLandingReplayMainFlakeDoesNotHideOwnFailure(t *testing.T) {
	t.Parallel()
	row := mainRepeatRecord(t, "main-repeat-batch-red.json")
	if row.Attempt != "20261011T235626.846906000Z" || row.Cause.Name != "red:metasystem/cmd/metasystem:TestPendingWaitFromChildShell" {
		t.Fatalf("saved replay shape: %+v", row)
	}
	b, register, _ := incidentProveFixture(t, false)
	before := register.repo.canonical
	unit := row.Failed[0]
	flakeTest := "TestPendingWaitFromChildShell"
	var flakes []plain.FlakeRecord
	writeFlake := b.owners.landing.landingFlakeRecorder(b.install)
	b.owners.landing.plainProve.RecordFlake = func(f plain.FlakeRecord) (plain.FlakeRecorded, error) {
		flakes = append(flakes, f)
		return writeFlake(f)
	}
	b.fail = func(cmd *exec.Cmd, only string) (string, error) {
		if freshProofEnv(cmd, "LANDING_COMMIT") == "main" {
			switch freshProofEnv(cmd, "LANDING_ONLY_TESTS") {
			case strings.Join(unit.Tests, " "):
				return "LANDING-CHECKED\t0\n", nil
			case "":
				return mainRepeatEvents(unit.Unit, []string{flakeTest}, "fail") + fmt.Sprintf("LANDING-FAILED\t%s\t%s\nLANDING-LOAD\t3.5\nLANDING-CHECKED\t1\n", unit.Unit, flakeTest), errors.New("red")
			case flakeTest:
				return mainRepeatEvents(unit.Unit, []string{flakeTest}, "pass") + "LANDING-CHECKED\t0\n", nil
			default:
				t.Fatalf("wrong named selection: %v", cmd.Env)
			}
		}
		return mainRepeatEvents(unit.Unit, unit.Tests, "fail") + fmt.Sprintf("LANDING-FAILED\t%s\t%s\nLANDING-CHECKED\t1\n", unit.Unit, strings.Join(unit.Tests, " ")), errors.New("own red")
	}
	check := b.prove(t)
	results, err := plain.Results(b.install)
	helmMust(t, err)
	isolated := 0
	for _, r := range results {
		if r.Tree == "main-tree" && r.FlakePublished {
			isolated++
			if r.Scope != "scoped" || r.Trunk || r.CountedFull || r.FullTree != "" || len(r.Goals) != len(check.Goals) {
				t.Fatalf("isolated green became full evidence or lost batch context: %+v", r)
			}
		}
	}
	if isolated != 1 {
		t.Fatalf("isolated flake outcome missing: %+v", results)
	}
	if check.Cause == nil || check.Cause.Kind != "own" || check.Cause.Goal != "b" || register.repo.canonical == before || len(flakes) != 1 || flakes[0].Where != "main" || flakes[0].Load != 3.5 || len(b.runs) != 4 {
		t.Fatalf("own red lost to replay flake: %+v cause=%+v flakes=%+v runs=%v", check, check.Cause, flakes, b.runs)
	}
}

// This certifies the shipped contract and ledger rather than a fixture install.
// The host reporter runs it by name to prove the real named execution plan.
func TestLandingMainRepeatRealContract(t *testing.T) {
	t.Parallel()
	contract, err := testpolicy.Load(filepath.Join("..", "..", "testing.json"))
	helmMust(t, err)
	pages, err := filepath.Glob(filepath.Join("..", "..", "plans", "goals", "*.md"))
	helmMust(t, err)
	parsed := 0
	for _, path := range pages {
		if filepath.Base(path) == "backlog.md" {
			continue
		}
		data, err := os.ReadFile(path)
		helmMust(t, err)
		file, problems := goal.ParseFile(data)
		if len(problems) != 0 || goal.GateTier(file) == 0 {
			t.Fatalf("real ledger %s: %v", path, problems)
		}
		parsed++
	}
	if parsed == 0 || len(contract.Groups) == 0 {
		t.Fatal("real contract or ledger absent")
	}
	for _, name := range []string{"trunk-repeat-red.json", "main-repeat-batch-red.json"} {
		row := mainRepeatRecord(t, name)
		if len(row.Failed) != 1 || row.Failed[0].Unit != "metasystem/cmd/metasystem" || len(row.Failed[0].Tests) == 0 {
			t.Fatalf("real reporter identities: %+v", row.Failed)
		}
	}
	t.Logf("real contract: %d groups; live ledger: %d parsed goal records", len(contract.Groups), parsed)
}
