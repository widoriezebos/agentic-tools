package dispatch

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
)

func TestReservedUnknownReadAdmissionPreservesAuthorityLimits(t *testing.T) {
	t.Parallel()
	for _, variation := range []string{"full-work-box", "active-ceiling", "elapsed", "unreserved", "wrong-revision"} {
		t.Run(variation, func(t *testing.T) {
			t.Parallel()
			bed := newGoalAdmissionBed(t, 2)
			path := filepath.Join(bed.root, "plans/goals/bounded.md")
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			file, problems := goal.ParseFile(data)
			if len(problems) != 0 {
				t.Fatal(problems)
			}
			file.Budget.AttemptLimit = 1
			if err := os.WriteFile(path, goal.RenderFile(file), 0o644); err != nil {
				t.Fatal(err)
			}
			bed.accept(t)
			writeBudgetJob(t, bed.root, "original", "original-op", 2, 30, "completed", budgetJobLife{})
			recordPath := filepath.Join(bed.root, "artifacts/agents/jobs/original.json")
			record := readJSONFile(t, recordPath)
			record["role"], record["round"] = "code-critic", 1
			if err := writeRecord(recordPath, record); err != nil {
				t.Fatal(err)
			}
			if variation != "unreserved" {
				if err := ReserveUnknownExaminationRetry(bed.root, "original"); err != nil {
					t.Fatal(err)
				}
			}
			if variation == "active-ceiling" {
				writeBudgetJob(t, bed.root, "live", "live-op", 2, 30, "running", budgetJobLife{})
			}
			now := time.Date(2026, 8, 28, 10, 0, 0, 0, time.UTC)
			if variation == "elapsed" {
				now = now.Add(48 * time.Hour)
			}
			revision := uint64(2)
			if variation == "wrong-revision" {
				revision = 3
			}
			reads := ProofAdmissionReads{ResolveEndpoint: bed.reads.ResolveEndpoint, ResolveMachine: bed.reads.ResolveMachine,
				Receipt: ReceiptAdmissionSource{AcceptedLedgerTip: func(string) (string, bool, error) { return admissionFixtureTip, true, nil },
					TopLevel: func(string) (string, error) { return bed.root, nil },
					FileAt:   func(string, string, string) ([]byte, bool, error) { return nil, false, nil }}}
			verdict, err := EvaluateUnknownExaminationRetryAdmission(bed.root, "original", "bounded", revision, 120, now, reads)
			if variation == "full-work-box" {
				if err != nil || verdict.Refused() {
					t.Fatalf("reserved read consumed another work allowance: %+v %v", verdict, err)
				}
			} else if err == nil && !verdict.Refused() {
				t.Fatalf("%s bypassed the examination's authority limits", variation)
			}
			global, globalErr := evaluateGoalAdmissionForUnknownExaminationRetryWithReads(bed.root, "coordinator", "original", now, bed.reads)
			if variation == "full-work-box" && (globalErr != nil || global.Refused()) {
				t.Fatalf("global goal admission charged the reserved read: %+v %v", global, globalErr)
			}
			if (variation == "active-ceiling" || variation == "elapsed" || variation == "unreserved") && globalErr == nil && !global.Refused() {
				t.Fatalf("global %s admission bypassed authority limits", variation)
			}
		})
	}
}
