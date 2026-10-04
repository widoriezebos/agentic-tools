package main

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/lane"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/plain"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testpolicy"
)

func TestLandingFlakeJudgeFromMain(t *testing.T) {
	t.Parallel()
	for _, name := range []string{"unaffected", "consumer", "provider", "fallback", "no files", "closed", "missing test", "wrong class", "wrong identity", "no contract", "config error", "contract error", "decode error", "register error", "register decode error", "diff error", "files error", "uncertainty"} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			b := newResolveVerbFixture(t)
			contract := testpolicy.Contract{SchemaVersion: 1,
				ProjectRisk: testpolicy.ProjectRisk{Severity: 1, Exposure: 1, Reversibility: "revert", Detection: "immediate", Recovery: "bounded"},
				Fallback:    "residual", Surfaces: []testpolicy.Surface{
					{ID: "provider", Paths: []string{"src/provider/**"}, Standard: []string{"check"}},
					{ID: "consumer", Paths: []string{"src/unit/one.go", "src/consumer/**"}, DependsOn: []string{"provider"}},
					{ID: "shared", Paths: []string{"src/unit/two.go"}}, {ID: "residual"}, {ID: "records", Paths: []string{"metasystem/testing.json", "metasystem/plans/goals/**"}}},
				Groups: []testpolicy.Group{{ID: "check", Kind: "unit", Adapter: "go", CWD: ".", Inputs: []string{"src/**"}, Platforms: []string{"any"}, TargetMS: 1, Packages: []string{"src"}, Tests: json.RawMessage(`"all"`)}},
				Always: testpolicy.Always{Canary: []string{"check"}}, Unknown: []string{"check"}, Cadence: []string{"check"}}
			if name == "uncertainty" {
				contract.Fallback = ""
				contract.Surfaces = contract.Surfaces[:3]
			}
			if err := contract.Validate(); err != nil {
				t.Fatal(err)
			}
			data, err := json.Marshal(contract)
			if err != nil {
				t.Fatal(err)
			}
			entry := goal.TrunkRedEntry{ID: "flaky:src/unit", Identity: "flaky:src/unit", Group: "src/unit", Status: "failed", Class: goal.TrunkRedClassPendingFlake,
				Failures: []goal.TrunkRedFailure{{Report: "red.log", Classname: "src/unit", Name: "TestOne"}, {Report: "red.log", Classname: "src/unit", Name: "TestTwo"}}, Holds: []string{}, Opened: "2026-10-04T10:00:00Z",
				Sightings: []goal.TrunkRedSighting{{Attempt: "red", BaseCommit: "old", SeenAt: "2026-10-04T10:00:00Z", Opid: "01J5X0000000000000000000F1-lane-12345678"}}}
			if name == "closed" {
				entry.Closed = &goal.TrunkRedClosure{At: entry.Opened, How: "hand", By: "Wido", Why: "fixed", Opid: entry.Sightings[0].Opid}
			}
			if name == "missing test" {
				entry.Failures = entry.Failures[:1]
			}
			if name == "wrong class" {
				entry.Class = goal.TrunkRedClassTrunkRed
			}
			if name == "wrong identity" {
				entry.ID, entry.Identity = "other", "other"
			}
			if name == "no contract" || name == "config error" {
				conf := "testing.contract=\n"
				if name == "config error" {
					conf += "testing.contract=duplicate\n"
				}
				if err := os.WriteFile(filepath.Join(b.install, "metasystem.conf"), []byte(conf), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			b.owners.landing.plainProve.Git = func(dir string, args ...string) (string, error) {
				if dir != b.root {
					t.Fatalf("judge read outside lane: %s", dir)
				}
				switch strings.Join(args, " ") {
				case "show origin/main:metasystem/testing.json":
					if name == "contract error" {
						return "", errors.New("cannot read")
					}
					if name == "decode error" {
						return "{", nil
					}
					return string(data), nil
				case "show origin/main:metasystem/plans/goals/trunk-red.json":
					if name == "register error" {
						return "", errors.New("cannot read")
					}
					if name == "register decode error" {
						return "{", nil
					}
					return string(goal.RenderTrunkRed([]goal.TrunkRedEntry{entry})), nil
				case "diff --name-only origin/main...batch":
					if name == "diff error" {
						return "", errors.New("cannot compare")
					}
					if name == "consumer" {
						return "src/consumer/change.go", nil
					}
					if name == "provider" {
						return "src/provider/change.go", nil
					}
					if name == "fallback" || name == "uncertainty" {
						return "unowned/file", nil
					}
					return "metasystem/testing.json\nmetasystem/plans/goals/trunk-red.json", nil
				case "ls-tree -z origin/main:src/unit":
					if name == "files error" {
						return "", errors.New("cannot list")
					}
					if name == "no files" {
						return "", nil
					}
					if name == "fallback" {
						return "100644 blob abc\tthree.go", nil
					}
					return "100644 blob abc\tone.go\x00100644 blob def\ttwo.go\x00040000 tree ghi\tnested\x00", nil
				case "ls-tree -z origin/main:src/other":
					return "100644 blob abc\tother.go", nil
				default:
					t.Fatalf("unexpected git read %v", args)
					return "", nil
				}
			}
			judge := b.owners.landing.proveSeams(b.install).Judge
			if judge == nil {
				t.Fatal("the lane has no judge")
			}
			if _, problems := goal.ParseTrunkRed(goal.RenderTrunkRed([]goal.TrunkRedEntry{entry})); len(problems) != 0 {
				t.Fatal(problems)
			}
			got, judgeErr := judge(b.root, "batch", []plain.FailedUnit{{Unit: "src/unit", Tests: []string{"TestOne", "TestTwo"}}, {Unit: "src/other", Tests: []string{"TestOther"}}})
			uncertain := strings.Contains(name, "error") || name == "no contract" || name == "uncertainty"
			wantAffected := uncertain || name == "consumer" || name == "provider" || name == "fallback" || name == "no files"
			if got["src/unit"].Affected != wantAffected || got["src/other"].Affected != (uncertain || name == "fallback") {
				t.Fatalf("affected = %+v; error %v", got, judgeErr)
			}
			if !uncertain {
				if judgeErr != nil {
					t.Fatal(judgeErr)
				}
				wantKnown := name != "closed" && name != "missing test" && name != "wrong class" && name != "wrong identity"
				if got["src/unit"].Known != wantKnown || got["src/other"].Known {
					t.Fatalf("known = %+v", got)
				}
				want := []string{"consumer", "shared"}
				if name == "fallback" {
					want = []string{"residual"}
				}
				if name == "no files" {
					want = []string{}
				}
				if !reflect.DeepEqual(got["src/unit"].Surfaces, want) {
					t.Fatalf("holding surfaces = %v, want %v", got["src/unit"].Surfaces, want)
				}
			}
		})
	}
}

func TestLandingFlakeRecordFieldsAndPublication(t *testing.T) {
	t.Parallel()
	for _, name := range []string{"confirmed", "rejected", "expired", "record error", "endpoint error", "machine error"} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			endpoint := goal.Endpoint{Root: "main", Remote: "origin", Branch: "refs/heads/main"}
			record := plain.FlakeRecord{FailedUnit: plain.FailedUnit{Unit: "src/unit", Tests: []string{"TestOne", "TestTwo"}, Surfaces: []string{"consumer"}}, Commit: "batch", Tree: "tree", Attempt: "red", Log: "red.log", Load: 2.5, Repeat: "alone", RepeatAttempt: "green", RepeatLog: "green.log"}
			called := false
			owners := laneVerbOwners{now: func() time.Time { return laneTestNow }, machine: func(root string) (string, error) {
				if root != "install" {
					t.Fatal(root)
				}
				if name == "machine error" {
					return "", errors.New("no machine")
				}
				return "lane", nil
			},
				mainEndpoint: func(root string) (goal.Endpoint, error) {
					if root != "install" {
						t.Fatal(root)
					}
					if name == "endpoint error" {
						return goal.Endpoint{}, errors.New("no endpoint")
					}
					return endpoint, nil
				},
				recordFlake: func(r goal.VerbRequest, args goal.FlakeRecordArgs) (goal.FlakeRecordResult, error) {
					called = true
					want := goal.FlakeRecordArgs{Unit: record.Unit, Tests: record.Tests, Surfaces: record.Surfaces, Commit: record.Commit, Tree: record.Tree, Attempt: record.Attempt, LogPath: record.Log, Load: record.Load, Repeat: record.Repeat, Rerun: goal.TrunkRedRerun{Attempt: record.RepeatAttempt, LogPath: record.RepeatLog}}
					if !reflect.DeepEqual(args, want) || !reflect.DeepEqual(r.Endpoint, endpoint) || r.Actor != (goal.Actor{Machine: "lane", Lineage: lane.AgentLineage}) || r.Now != laneTestNow || r.Ulid == "" {
						t.Fatalf("record request = %+v %+v", r, args)
					}
					if name == "record error" {
						return goal.FlakeRecordResult{}, errors.New("not recorded")
					}
					outcome := goal.OutcomeConfirmed
					if name == "rejected" {
						outcome = goal.OutcomeRejected
					}
					if name == "expired" {
						outcome = goal.OutcomeExpired
					}
					return goal.FlakeRecordResult{FixGoal: "fix-flaky-src-unit", Sightings: 3, Publish: goal.PublishResult{Outcome: outcome}}, nil
				}}
			seam := owners.proveSeams("install").RecordFlake
			if seam == nil {
				t.Fatal("the lane cannot record a flake")
			}
			got, err := seam(record)
			if (err == nil) != (name == "confirmed") || name == "confirmed" && got != (plain.FlakeRecorded{Goal: "fix-flaky-src-unit", Seen: 3}) || called != (name != "endpoint error" && name != "machine error") {
				t.Fatalf("record = %+v %v, called %v", got, err, called)
			}
		})
	}
}

func TestLandingFlakeRefusalAndInheritedReason(t *testing.T) {
	t.Parallel()
	for _, action := range []string{"refusal", "prove", "prove wait", "status", "status json"} {
		t.Run(action, func(t *testing.T) {
			t.Parallel()
			b := newResolveVerbFixture(t)
			if err := os.WriteFile(filepath.Join(b.install, "metasystem.conf"), []byte("landing.prove.command=unused\n"), 0o644); err != nil {
				t.Fatal(err)
			}
			reason := "src/unit failed once and passed when run again alone; seen 3 times; goal fix-flaky-src-unit fixes it"
			result := plain.Result{Commit: "old", Tree: "old-tree", Result: plain.Green, Reason: reason, At: laneTestNow.Format(time.RFC3339)}
			if action == "refusal" || action == "status json" {
				result.Commit, result.Tree, result.Result = "head", "tree", plain.Red
				result.Failed = []plain.FailedUnit{{Unit: "src/unit", Tests: []string{"TestOne"}, Surfaces: []string{"consumer"}}}
				result.Load, result.Repeat = 2.5, "started"
			}
			if err := os.MkdirAll(plain.Dir(b.install), 0o755); err != nil {
				t.Fatal(err)
			}
			data, _ := json.Marshal(result)
			if err := os.WriteFile(filepath.Join(plain.Dir(b.install), "results.jsonl"), append(data, '\n'), 0o644); err != nil {
				t.Fatal(err)
			}
			b.owners.landing.plainProve = plain.ProveSeams{Now: func() time.Time { return laneTestNow }, NewID: func() string { return "inherited" }, Git: func(_ string, args ...string) (string, error) {
				switch strings.Join(args, " ") {
				case "rev-parse --verify HEAD^{commit}":
					return "head", nil
				case "rev-parse --verify HEAD^{tree}":
					return "tree", nil
				case "diff --name-only --no-renames old-tree tree":
					return "metasystem/plans/goals/trunk-red.json", nil
				}
				t.Fatalf("unexpected git %v", args)
				return "", nil
			}}
			words := []string{"prove"}
			if action == "prove wait" {
				words = append(words, "--wait")
			}
			if strings.HasPrefix(action, "status") {
				if action == "status" {
					if _, ok, err := plain.Settled(b.install, b.root, b.owners.landing.plainProve); err != nil || !ok {
						t.Fatalf("inherit = %v %v", ok, err)
					}
				}
				words = []string{"status"}
				if action == "status json" {
					words = append(words, "--json")
				}
			}
			code, output := b.run(t, b.root, words...)
			if action == "refusal" {
				if code != 1 || !strings.Contains(output, "landing return GOAL --reason TEXT") || strings.Contains(output, "could not run") || !strings.Contains(output, "gets no other") {
					t.Fatalf("refusal = %d %s", code, output)
				}
				return
			}
			if code != 0 {
				t.Fatalf("command = %d %s", code, output)
			}
			if action == "status json" {
				var out struct {
					Data struct {
						Last plain.Result `json:"last_proof"`
					}
				}
				if err := json.Unmarshal([]byte(output), &out); err != nil || !reflect.DeepEqual(out.Data.Last, result) {
					t.Fatalf("last check = %+v %v", out, err)
				}
				return
			}
			if !strings.Contains(strings.Join(strings.Fields(output), " "), reason) || !strings.Contains(output, "green") {
				t.Fatalf("inherited green = %s", output)
			}
		})
	}
}

func TestLandingFlakeSkillAndProtocol(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		path    string
		phrases []string
	}{
		{"skills/landing-agent/SKILL.md", []string{"last_proof.repeat", "allowed", "last_proof.failed", "recorded flake", "no other check of that tree"}},
		{"docs/flake-registry.md", []string{"one repeat per tree", "fix-flaky-UNIT", "first sighting", "plans/goals/trunk-red.json", "old hand register"}},
	} {
		t.Run(tc.path, func(t *testing.T) {
			t.Parallel()
			data, err := os.ReadFile(filepath.Join("../..", tc.path))
			if err != nil {
				t.Fatal(err)
			}
			for _, phrase := range tc.phrases {
				if !strings.Contains(string(data), phrase) {
					t.Errorf("%s lacks %q", tc.path, phrase)
				}
			}
		})
	}
}
