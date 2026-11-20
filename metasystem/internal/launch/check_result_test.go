package launch

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/processmeasure"
)

func TestCheckResultRequiresCompletePass(t *testing.T) {
	t.Parallel()
	for _, row := range []struct {
		name    string
		version int
		outcome CheckOutcome
		pass    bool
	}{
		{"legacy", 0, "", true}, {"passed", 1, CheckPassed, true},
		{"failed", 1, CheckFailed, false}, {"budget", 1, CheckBudgetExhausted, false},
		{"environment", 1, CheckEnvironmentFailure, false}, {"missing outcome", 1, "", false},
		{"unknown outcome", 1, "interrupted", false}, {"unknown version", 2, CheckPassed, false},
		{"legacy with budget outcome", 0, CheckBudgetExhausted, false},
	} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			result := CheckResult{Version: row.version, Outcome: row.outcome, ExecutionID: "check-one", Exits: []CheckExit{{Name: "cheap"}, {Name: "audits"}}}
			if (result.RequirePassed() == nil) != row.pass {
				t.Fatalf("outcome %q: acceptance=%v, want %v", row.outcome, result.RequirePassed() == nil, row.pass)
			}
			if row.pass {
				for _, damage := range []string{"missing", "red", "error", "order", "identity", "cut"} {
					broken := result
					broken.Exits = append([]CheckExit(nil), result.Exits...)
					switch damage {
					case "missing":
						broken.Exits = broken.Exits[:1]
					case "red":
						broken.Exits[1].Exit = 1
					case "error":
						broken.Exits[0].Error = "interrupted"
					case "order":
						broken.Exits[0], broken.Exits[1] = broken.Exits[1], broken.Exits[0]
					case "cut":
						broken.Exits[0].Cut = true
					case "identity":
						broken.ExecutionID = ""
					}
					if broken.RequirePassed() == nil {
						t.Fatalf("accepted %s result", damage)
					}
				}
			}
		})
	}
}

func TestCheckResultReadBindsExecution(t *testing.T) {
	t.Parallel()
	dir := filepath.Join(t.TempDir(), "check-one")
	if err := os.Mkdir(dir, 0700); err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"check-one", "check-other"} {
		data, err := json.Marshal(CheckResult{ExecutionID: id})
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "result.json"), data, 0600); err != nil {
			t.Fatal(err)
		}
		_, err = ReadCheckResult(dir)
		if (err == nil) != (id == "check-one") {
			t.Fatalf("execution %s: %v", id, err)
		}
	}
}

func TestCheckOutcomePreservesFailureBeforeInterruption(t *testing.T) {
	t.Parallel()
	for _, row := range []struct {
		first, second string
		want          CheckOutcome
	}{
		{"passed", "passed", CheckPassed},
		{"deadline", "passed", CheckEnvironmentFailure},
		{"passed", "cancelled", CheckFailed},
		{"failed", "deadline", CheckFailed},
		{"deadline", "failed", CheckFailed},
		{"cancelled", "deadline", CheckFailed},
		{"deadline", "cancelled", CheckFailed},
		{"deadline", "deadline", CheckEnvironmentFailure},
		{"passed", "unknown", CheckFailed},
		{"passed", "", CheckFailed},
	} {
		execution := CheckExecution{Steps: []processmeasure.Step{{Outcome: row.first, Terminal: true}, {Outcome: row.second, Terminal: row.second != ""}}}
		round := t.TempDir()
		writeCheckObservation(t, round, "check-one", "launch-one", "2026-11-19T20:00:00Z", row.first, row.second)
		step := UnitStep{LaunchID: "launch-one", Cause: "unclassified"}
		if environment := checkCause(&step, round); environment != (row.want == CheckEnvironmentFailure) {
			t.Fatalf("%s/%s: round environment=%v disagrees with outcome %s", row.first, row.second, environment, row.want)
		}
		if got := execution.outcome(); got != row.want {
			t.Fatalf("%s/%s: %s, want %s", row.first, row.second, got, row.want)
		}
	}
}

func TestApplicableCheckRejectsIncompleteOutcomes(t *testing.T) {
	t.Parallel()
	for _, outcome := range []CheckOutcome{CheckBudgetExhausted, CheckFailed, CheckEnvironmentFailure, "unknown"} {
		for _, strength := range []string{"cheap", "audited"} {
			evidence, need := applicableCheckFixture(t)
			evidence.Outcome, need.Strength = outcome, strength
			decision := ApplicableCheck(evidence, "evidence.json", nil, need)
			if decision.Reuse != nil || decision.Field != "outcome" {
				t.Fatalf("%s/%s accepted as complete: %+v", outcome, strength, decision)
			}
		}
	}
}

func TestPublicationRerunsIncompleteCheck(t *testing.T) {
	t.Parallel()
	f, run, evidence, need, messages := publicationFixture(t, "match")
	record, err := f.runner.Status(run)
	if err != nil {
		t.Fatal(err)
	}
	evidence.Outcome = CheckBudgetExhausted
	path := filepath.Join(record.Rounds[0].Directory, "check-source", "evidence.json")
	if err := writeUnitJSON(path, evidence, f.runner.Root); err != nil {
		t.Fatal(err)
	}
	before := len(f.starter.ids)
	subject := provePublication(t, f.runner, run, f.worktree, need.Base)
	if subject.GateReuse != nil || len(f.starter.ids) != before+1 || len(*messages) != 1 {
		t.Fatal("publication reused incomplete coverage instead of running its check")
	}
}

func TestCheckEvidenceUsesExecutionOutcome(t *testing.T) {
	t.Parallel()
	for _, outcome := range []CheckOutcome{CheckPassed, CheckFailed, CheckBudgetExhausted, CheckEnvironmentFailure} {
		directory := t.TempDir()
		evidence := (UnitCheck{}).evidence(directory, directory, directory, "tree", CheckContext{}, outcome)
		if evidence.Outcome != outcome {
			t.Fatalf("evidence outcome=%s, want %s", evidence.Outcome, outcome)
		}
	}
}

func TestCheckResultLatestExecution(t *testing.T) {
	t.Parallel()
	round := t.TempDir()
	for _, row := range []struct {
		id, start string
		outcome   CheckOutcome
	}{
		{"check-z", "2026-11-20T00:00:00Z", CheckBudgetExhausted},
		{"check-a", "2026-11-20T00:01:00Z", CheckPassed},
	} {
		writeCheckObservation(t, round, row.id, "launch-one", row.start, "passed", "passed")
		result := CheckResult{ExecutionID: row.id, Outcome: row.outcome}
		if err := writeUnitJSON(filepath.Join(round, row.id, "result.json"), result, round); err != nil {
			t.Fatal(err)
		}
		got, err := LatestCheckResult(round)
		if err != nil || got.ExecutionID != row.id {
			t.Fatalf("latest execution=%s error=%v", got.ExecutionID, err)
		}
	}
	writeCheckObservation(t, round, "check-retry", "launch-one", "2026-11-20T00:02:00Z", "passed", "")
	if _, err := LatestCheckResult(round); err == nil {
		t.Fatal("unfinished retry inherited a previous result")
	}
}
