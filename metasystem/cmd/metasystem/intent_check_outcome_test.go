package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/launch"
)

func TestIntentUnitCheckRecordsTerminalOutcome(t *testing.T) {
	t.Parallel()
	for _, row := range []struct {
		name, cheap, audits string
		outcome             launch.CheckOutcome
	}{
		{"passed", "printf cheap", "printf audits", launch.CheckPassed},
		{"cheap red", "printf cheap; exit 7", "printf audits", launch.CheckFailed},
		{"audit red", "printf cheap", "printf audits; exit 9", launch.CheckFailed},
		{"signal killed", "kill -TERM $$", "printf audits", launch.CheckFailed},
		{"red before interruption", "exit 7", "kill -TERM $$", launch.CheckFailed},
	} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			b := declaredCheckBed(t, "proof.cheap="+row.cheap+"\nproof.audits="+row.audits+"\nproof.deadline=15\n")
			code, built, _ := b.work("work", "build", b.id, "outcome", "--brief", b.brief("outcome.md", "Build the fixture.\n"), "--lines", "100")
			if code != 0 {
				t.Fatalf("fixture admission failed with code %d", code)
			}
			run := resultData(t, built)["run"].(string)
			code, result := declaredCheckAt(t, b, b.worktree, "proof", "test", "run", "--unit-run", run)
			data := resultData(t, result)
			if (code == 0) != (row.outcome == launch.CheckPassed) || data["checkOutcome"] != string(row.outcome) {
				t.Fatalf("public outcome=%v code=%d, want %s", data["checkOutcome"], code, row.outcome)
			}
			execution := data["execution"].(string)
			retained, err := launch.ReadCheckResult(execution)
			if err != nil || retained.Version != 1 || retained.Outcome != row.outcome || len(retained.Exits) != 2 || retained.Exits[0].Name != "cheap" || retained.Exits[1].Name != "audits" {
				t.Fatalf("terminal record lost outcome or command order: %v", err)
			}
			observations, unknown := launch.ReadCheckExecutions(execution)
			if len(unknown) != 0 || len(observations) != 1 || observations[0].Outcome != row.outcome {
				t.Fatal("execution observation lost terminal outcome")
			}
			if row.outcome == launch.CheckPassed {
				data, err := os.ReadFile(filepath.Join(execution, "evidence.json"))
				var evidence launch.CheckEvidence
				if err != nil || json.Unmarshal(data, &evidence) != nil || evidence.Outcome != retained.Outcome {
					t.Fatal("coverage evidence lost the execution outcome")
				}
			}
			if row.outcome != launch.CheckPassed {
				if _, err := os.Stat(filepath.Join(execution, "evidence.json")); !os.IsNotExist(err) {
					t.Fatal("nonpassing execution retained green evidence")
				}
			}
		})
	}
}

func TestCarryRefusesRetainedBudgetOutcome(t *testing.T) {
	t.Parallel()
	b, owners, args, _ := carryRecoveryBed(t)
	if code, _ := b.runJSON(owners, args...); code != 0 {
		t.Fatalf("carry fixture failed: %d", code)
	}
	records := filepath.Join(b.root(), "artifacts", "unit-checks", "carry", args[3])
	paths, err := filepath.Glob(filepath.Join(records, "check-*", "result.json"))
	if err != nil || len(paths) != 1 {
		t.Fatal("missing carry execution")
	}
	retained, err := launch.ReadCheckResult(filepath.Dir(paths[0]))
	if err != nil {
		t.Fatal(err)
	}
	retained.Outcome = launch.CheckBudgetExhausted
	data, err := json.Marshal(retained)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(paths[0], data, 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(records, "gate.json")); err != nil {
		t.Fatal(err)
	}
	code, result := b.runJSON(owners, args...)
	if code == 0 || !strings.Contains(result.Summary, "--rerun") || !strings.Contains(result.Summary, "not a complete pass") {
		t.Fatal("carry accepted budget-exhausted exits as a completed pass")
	}
	if _, err := os.Stat(filepath.Join(records, "gate.json")); !os.IsNotExist(err) {
		t.Fatal("incomplete execution produced a carry gate")
	}
}
