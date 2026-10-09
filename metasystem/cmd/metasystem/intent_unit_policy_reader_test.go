package main

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/launch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/stateroot"
)

func TestIntentUnitUsesItsPolicyReader(t *testing.T) {
	t.Parallel()
	for _, broken := range []bool{false, true} {
		name := "zero corrections"
		if broken {
			name = "unreadable policy"
		}
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			bed := newWorkBed(t)
			owners := bed.workOwners()
			units := owners.work.units
			reads := 0
			owners.work.units = func(layout stateroot.Layout) *launch.UnitRunner {
				runner := units(layout)
				runner.ReviewPolicy = func() (string, error) {
					reads++
					if broken {
						return "", errors.New("the unit policy evidence cannot be read")
					}
					return "0", nil
				}
				return runner
			}
			brief := bed.brief("policy.md", "Build the unit.\n")
			code, result := bed.runJSON(owners, append([]string{"work", "build", bed.id, "policy", "--brief", brief, "--lines", "5", "--json"}, workCheck...)...)
			if reads == 0 {
				t.Fatal("unit admission bypassed its policy reader")
			}
			if broken {
				if code != 1 || !strings.Contains(result.Summary, "unit policy evidence cannot be read") || len(bed.starter.launched()) != 0 {
					t.Fatalf("unreadable policy admitted a build: code=%d %+v", code, result)
				}
				return
			}
			if code != 0 {
				t.Fatalf("unit policy did not admit the build: code=%d %+v", code, result)
			}
			record, err := (&launch.UnitRunner{Root: bed.unitRoot}).Status(resultData(t, result)["run"].(string))
			if err != nil || record.CorrectionBudget == nil || *record.CorrectionBudget != 0 {
				t.Fatalf("the unit did not freeze its zero correction allowance: %+v %v", record, err)
			}
		})
	}
}

func TestIntentUnitOnlyDecidesAfterARead(t *testing.T) {
	t.Parallel()
	for _, red := range []bool{false, true} {
		name := "checks pass"
		if red {
			name = "failed checks can be corrected"
		}
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			bed := newWorkBed(t)
			bed.starter.fail["proof"] = red
			brief := bed.brief("pending.md", "Build the unit without a preliminary read.\n")
			code, built, _ := bed.work(append([]string{"work", "build", bed.id, "pending", "--brief", brief, "--lines", "5"}, workCheck...)...)
			if code != map[bool]int{false: 0, true: 1}[red] || red && !strings.Contains(built.Summary, "stopped unclassified") {
				t.Fatalf("build: code=%d %+v", code, built)
			}
			run := resultData(t, built)["run"].(string)
			if red {
				delete(bed.starter.fail, "proof")
				code, corrected, _ := bed.work("work", "revise", bed.id, "--work", "pending", "--after", "1", "--brief", bed.brief("corrected.md", "Correct the failed checks.\n"), "--reason", "Repair the retained failed check", "--by", "Wido")
				if code != 0 || resultData(t, corrected)["round"] != float64(2) || resultData(t, corrected)["outcome"] != "green" {
					t.Fatalf("failed checks could not be corrected: code=%d %+v", code, corrected)
				}
			}
			record, err := (&launch.UnitRunner{Root: bed.unitRoot}).Status(run)
			if err != nil {
				t.Fatal(err)
			}
			for _, round := range record.Rounds {
				if round.Stop != nil && (round.Stop.Loop != "unit-build" || round.Cause != "unclassified") || len(round.Reads) != 0 || round.Material != -1 {
					t.Fatalf("an unexamined unit acquired a read decision: %+v", round)
				}
				if round.Stop != nil {
					var decision struct {
						Stop struct {
							Loop  string `json:"loop"`
							Cause struct {
								Kind string `json:"kind"`
							} `json:"cause"`
						} `json:"stop"`
					}
					data, err := os.ReadFile(filepath.Join(round.Directory, "stop-register.json"))
					if err != nil || json.Unmarshal(data, &decision) != nil || decision.Stop.Loop != "unit-build" || decision.Stop.Cause.Kind != "unclassified" {
						t.Fatalf("unexamined failure lost its build stop: %s %v", data, err)
					}
					continue
				}
				for _, name := range []string{"read-decision.json", "stop-register.json"} {
					if _, err := os.Stat(filepath.Join(round.Directory, name)); !os.IsNotExist(err) {
						t.Fatalf("unexamined unit wrote %s: %v", name, err)
					}
				}
			}
		})
	}
}
